package main

import (
	"encoding/json"
	"log"
	"net/http"
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

	ev.RawContent, _ = raw["raw_message"].(string)

	// 增强清理逻辑：移除所有 [CQ:...] 标签
	ev.Content = ev.RawContent
	for strings.Contains(ev.Content, "[CQ:") {
		start := strings.Index(ev.Content, "[CQ:")
		end := strings.Index(ev.Content[start:], "]")
		if end == -1 {
			break
		}
		ev.Content = ev.Content[:start] + ev.Content[start+end+1:]
	}
	ev.Content = strings.TrimSpace(ev.Content)

	// 提取昵称并更新映射（群聊时）
	if ev.MsgType == "group" && ev.GroupID > 0 && ev.UserID > 0 {
		nickname := extractNickname(raw)
		if nickname != "" {
			storage.UpdateNicknameMap(ev.GroupID, ev.UserID, nickname)
		} else {
			log.Printf("[DEBUG] 未提取到昵称: 群%d 用户%d，将使用稳定标识符", ev.GroupID, ev.UserID)
		}
		// 添加到群聊上下文（所有群聊消息都添加）
		if ev.Content != "" && ev.UserID != common.BotQQNumber {
			storage.AddGroupContextMessage(ev.GroupID, ev.UserID, ev.Content)
		}
	}

	return ev
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
