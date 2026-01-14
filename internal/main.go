package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"QQBot/internal/common"
	"QQBot/internal/deepseek"
	"QQBot/internal/local"
	"QQBot/internal/storage"
)

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

// --- 逻辑分发器 ---

func dispatch(event common.QQEvent) {
	// 0. 检查连续相同消息（仅群聊）
	if local.ShouldHandleRepeatMessage(event) {
		if local.HandleRepeatMessage(event) {
			return // 如果触发了重复消息回复，不再处理其他逻辑
		}
	}

	// 1. 本地指令
	if local.ShouldHandleLocalCommand(event.Content) {
		local.HandleLocalCommand(event)
		return
	}

	// 2. 群聊中@主人（优先级高于普通AI对话）
	if deepseek.ShouldHandleAtMasterChat(event) {
		go deepseek.HandleAtMasterChat(event)
		return
	}

	// 3. AI 对话
	if deepseek.ShouldHandleAIChat(event) {
		go deepseek.HandleAIChat(event)
	}
}

// --- 通信处理 ---

func parseEvent(raw map[string]interface{}) common.QQEvent {
	ev := common.QQEvent{}
	ev.MsgType, _ = raw["message_type"].(string)

	// 处理 JSON 中的数字类型
	if uid, ok := raw["user_id"].(float64); ok {
		ev.UserID = int64(uid)
	}
	if gid, ok := raw["group_id"].(float64); ok {
		ev.GroupID = int64(gid)
	}

	// 先更新发送者的昵称映射（群聊时），这样如果消息中 @ 的是发送者自己，就能用最新昵称
	if ev.MsgType == "group" && ev.GroupID > 0 && ev.UserID > 0 {
		nickname := extractNickname(raw)
		if nickname != "" {
			storage.UpdateNicknameMap(ev.GroupID, ev.UserID, nickname)
		} else {
			log.Printf("[DEBUG] 未提取到昵称: 群%d 用户%d，将使用稳定标识符", ev.GroupID, ev.UserID)
		}
	}

	// 解析消息内容（array 格式）
	ev.RawContent, ev.Content, ev.AtType = parseMessageArray(raw, ev.GroupID)

	// 添加到群聊上下文（所有群聊消息都添加）
	if ev.MsgType == "group" && ev.GroupID > 0 && ev.Content != "" && ev.UserID != common.BotQQNumber {
		storage.AddGroupContextMessage(ev.GroupID, ev.UserID, ev.Content)
	}

	return ev
}

// parseMessageArray 解析消息数组（array 格式）
// 返回：原始 JSON、解析后的内容、@类型
func parseMessageArray(raw map[string]interface{}, groupID int64) (rawJSON string, content string, atType int) {
	msgArray, ok := raw["message"].([]interface{})
	if !ok {
		// 保存原始 JSON 用于调试
		if jsonBytes, err := json.Marshal(raw["message"]); err == nil {
			rawJSON = string(jsonBytes)
		}
		return rawJSON, "", common.AtNone
	}

	// 保存原始 JSON 用于调试
	if jsonBytes, err := json.Marshal(msgArray); err == nil {
		rawJSON = string(jsonBytes)
	}

	var contentParts []string
	atType = common.AtNone

	// 遍历消息数组，按顺序处理
	for _, item := range msgArray {
		if msgObj, ok := item.(map[string]interface{}); ok {
			msgType, _ := msgObj["type"].(string)
			switch msgType {
			case "at":
				// 处理 @ 消息
				if data, ok := msgObj["data"].(map[string]interface{}); ok {
					var atQQ int64
					if qqStr, ok := data["qq"].(string); ok {
						// 尝试解析字符串格式的 QQ 号
						if qq, err := strconv.ParseInt(qqStr, 10, 64); err == nil {
							atQQ = qq
						}
					} else if qq, ok := data["qq"].(float64); ok {
						atQQ = int64(qq)
					}

					if atQQ > 0 {
						// 判断 @ 的类型（优先级：主人 > 机器人 > 其他人）
						if common.MasterQQNumber > 0 && atQQ == common.MasterQQNumber {
							if atType == common.AtNone || atType == common.AtOthers {
								atType = common.AtMaster
							}
						} else if common.BotQQNumber > 0 && atQQ == common.BotQQNumber {
							if atType == common.AtNone || atType == common.AtOthers {
								atType = common.AtBot
							}
						} else {
							if atType == common.AtNone {
								atType = common.AtOthers
							}
						}

						// 格式化 @ 消息：@【角色标签】昵称
						contentParts = append(contentParts, storage.FormatAtMessage(groupID, atQQ))
					}
				}
			case "text":
				// 处理文本消息
				if data, ok := msgObj["data"].(map[string]interface{}); ok {
					if text, ok := data["text"].(string); ok {
						contentParts = append(contentParts, text)
					}
				}
				// 其他类型（face、image 等）跳过
			}
		}
	}

	content = strings.TrimSpace(strings.Join(contentParts, ""))
	return rawJSON, content, atType
}

// extractNickname 从消息中提取昵称
func extractNickname(raw map[string]interface{}) string {
	// 尝试从 sender 中获取
	if sender, ok := raw["sender"].(map[string]interface{}); ok {
		// 优先使用群名片（card）
		if card, ok := sender["card"].(string); ok && card != "" {
			return card
		}
		// 其次使用昵称（nickname）
		if nickname, ok := sender["nickname"].(string); ok && nickname != "" {
			return nickname
		}
	}
	return ""
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("升级 WebSocket 失败: %v", err)
		return
	}

	common.SetWebSocketConn(conn)

	log.Println("✨ NapCat 成功连接")

	defer func() {
		common.ClearWebSocketConn()
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("连接中断: %v", err)
			break
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(msg, &raw); err == nil {
			if pt, _ := raw["post_type"].(string); pt == "message" {
				// 打印原始消息用于调试
				//rawJSON, _ := json.MarshalIndent(raw, "", "  ")
				//log.Printf("[DEBUG] 收到原始消息:\n%s\n", rawJSON)
				dispatch(parseEvent(raw))
			}
		}
	}
}

func main() {
	if common.DeepSeekAPIKey == "" {
		log.Fatal("错误：未找到环境变量 DEEPSEEK_API_KEY，请先设置！")
	}
	http.HandleFunc("/ws", wsHandler)
	log.Printf("🤖 小牛系统已就绪，端口%s", common.ListenPort)
	if err := http.ListenAndServe(common.ListenPort, nil); err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}
