package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// --- 配置区域 ---
const (
	ListenPort             = ":8080"
	DeepSeekBaseURL        = "https://api.deepseek.com/chat/completions"
	RepeatMessageQueueSize = 3 // 连续相同消息检测队列大小
)

type QQEvent struct {
	MsgType    string
	UserID     int64
	GroupID    int64
	Content    string // 过滤过CQ码消息后的内容
	RawContent string // 原始消息
}

var (
	upgrader       = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsConn         *websocket.Conn
	connMu         sync.Mutex
	DeepSeekAPIKey string
	BotQQNumber    int64
	MasterQQNumber int64
)

func init() {
	DeepSeekAPIKey = os.Getenv("DEEPSEEK_API_KEY")

	// 尝试读取并转换，如果失败则给个提醒
	botQQStr := os.Getenv("BOT_QQ")
	masterQQStr := os.Getenv("MASTER_QQ")

	if botQQStr == "" || masterQQStr == "" {
		log.Println("⚠️  警告: BOT_QQ 或 MASTER_QQ 未设置，机器人可能无法识别艾特或主人身份")
	}

	BotQQNumber, _ = strconv.ParseInt(botQQStr, 10, 64)
	MasterQQNumber, _ = strconv.ParseInt(masterQQStr, 10, 64)
}

// --- 逻辑分发器 ---

func dispatch(event QQEvent) {
	// 0. 检查连续相同消息（仅群聊）
	if ShouldHandleRepeatMessage(event) {
		if HandleRepeatMessage(event) {
			return // 如果触发了重复消息回复，不再处理其他逻辑
		}
	}

	// 1. 本地指令
	if ShouldHandleLocalCommand(event.Content) {
		HandleLocalCommand(event)
		return
	}

	// 2. AI 对话
	if ShouldHandleAIChat(event) {
		if event.Content == "" {
			sendReply(event, "干嘛？艾特我又不说话，是不是想我了？")
			return
		}
		go HandleAIChat(event)
	}
}

// --- 通信处理 ---

func parseEvent(raw map[string]interface{}) QQEvent {
	ev := QQEvent{}
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

	return ev
}

func sendReply(e QQEvent, text string) {
	connMu.Lock()
	defer connMu.Unlock()
	if wsConn == nil {
		log.Println("[警告] 发送失败：WebSocket 连接为空")
		return
	}

	payload := map[string]interface{}{
		"action": "send_msg",
		"params": map[string]interface{}{
			"message_type": e.MsgType,
			"user_id":      e.UserID,
			"group_id":     e.GroupID,
			"message":      text,
		},
	}

	if err := wsConn.WriteJSON(payload); err != nil {
		log.Printf("[发送失败]: %v", err)
	}
	log.Printf("[发送] -> 用户:%d 内容:%s", e.UserID, text)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("升级 WebSocket 失败: %v", err)
		return
	}

	connMu.Lock()
	wsConn = conn
	connMu.Unlock()

	log.Println("✨ NapCat 成功连接")

	defer func() {
		connMu.Lock()
		wsConn = nil
		connMu.Unlock()
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
	if DeepSeekAPIKey == "" {
		log.Fatal("错误：未找到环境变量 DEEPSEEK_API_KEY，请先设置！")
	}
	http.HandleFunc("/ws", wsHandler)
	log.Printf("🤖 小牛系统已就绪，端口%s", ListenPort)
	if err := http.ListenAndServe(ListenPort, nil); err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}
