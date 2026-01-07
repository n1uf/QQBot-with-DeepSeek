package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- 配置区域 ---
const (
	ListenPort      = ":8080"
	BotQQNumber     = 1851469506
	MasterQQNumber  = 2318607163
	DeepSeekAPIKey  = "APIKEY" // 填入你的 Key
	DeepSeekBaseURL = "https://api.deepseek.com/chat/completions"
)

// --- 数据结构 ---
type QQEvent struct {
	MsgType    string
	UserID     int64
	GroupID    int64
	Content    string // 清理后的纯文本
	RawContent string // 包含 CQ 码的原始文本
}

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsConn   *websocket.Conn
	connMu   sync.Mutex
)

// --- 逻辑分发器 (Dispatcher) ---

func dispatch(event QQEvent) {
	// 1. 优先判定：本地指令 - 以 niuf 开头
	if strings.HasPrefix(event.Content, "niuf") {
		handleLocalCommand(event)
		return
	}

	// 2. 判定 AI 触发条件
	isPrivate := event.MsgType == "private"                                         // 私聊
	isAtMe := strings.Contains(event.RawContent, fmt.Sprintf("qq=%d", BotQQNumber)) // 被艾特 (直接检查原始报文)
	isCalledMe := strings.Contains(event.Content, "小牛")                             // 被喊名字

	if isPrivate || isAtMe || isCalledMe {
		// 如果是群聊被艾特，且内容为空（只艾特没说话），给个默认回复
		if event.Content == "" && (isAtMe || isCalledMe) {
			sendReply(event, "干嘛？艾特我又不说话，是不是想我了？")
			return
		}

		go handleAIChat(event)
		return
	}

	// 其他消息不做处理，静默丢弃
}

// --- 具体逻辑实现 ---

// 本地指令逻辑
func handleLocalCommand(e QQEvent) {
	log.Printf("[本地] 收到指令: %s", e.Content)
	sendReply(e, "1")
}

// DeepSeek AI 逻辑
func handleAIChat(e QQEvent) {
	// 身份脱敏逻辑
	var hint string
	if e.UserID == MasterQQNumber {
		hint = "当前说话的是你的主人 niuf，对他要亲切一点。"
	} else {
		hint = "当前说话的是一位普通好友，保持礼貌即可。"
	}

	log.Printf("[收到] <- 用户:%d 内容:%s", e.UserID, e.Content)

	// 调用修改后的函数
	answer, err := callDeepSeek(e.Content, hint)
	if err != nil {
		log.Printf("[AI] 出错: %v", err)
		sendReply(e, "小牛有点累了...")
		return
	}

	sendReply(e, answer)
}

// --- 工具函数：调用 DeepSeek API ---

func callDeepSeek(content string, roleHint string) (string, error) {
	// 动态拼接系统提示词，不含任何数字 ID
	systemMessage := fmt.Sprintf("你是一个幽默的助手小牛。你的主人是 niuf。%s", roleHint)

	requestBody, _ := json.Marshal(map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemMessage},
			{"role": "user", "content": content},
		},
		"temperature": 0.7,
	})

	req, _ := http.NewRequest("POST", DeepSeekBaseURL, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "我不知道该怎么回答呢。", nil
}

// --- 底层解析与通信 (保持不变) ---

func parseEvent(raw map[string]interface{}) QQEvent {
	ev := QQEvent{}
	ev.MsgType, _ = raw["message_type"].(string)
	if uid, ok := raw["user_id"].(float64); ok {
		ev.UserID = int64(uid)
	}
	if gid, ok := raw["group_id"].(float64); ok {
		ev.GroupID = int64(gid)
	}

	// 保存原始报文用于判定艾特
	ev.RawContent, _ = raw["raw_message"].(string)

	// 清理后的内容用于 AI 思考
	atBot := fmt.Sprintf("[CQ:at,qq=%d]", BotQQNumber)
	ev.Content = strings.TrimSpace(strings.ReplaceAll(ev.RawContent, atBot, ""))

	return ev
}

func sendReply(e QQEvent, text string) {
	connMu.Lock()
	defer connMu.Unlock()
	if wsConn == nil {
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
	wsConn.WriteJSON(payload)
	log.Printf("[发送] -> 用户:%d 内容:%s", e.UserID, text)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	log.Println("✨ NapCat 成功连接")
	connMu.Lock()
	wsConn = conn
	connMu.Unlock()
	defer func() {
		connMu.Lock()
		wsConn = nil
		connMu.Unlock()
		conn.Close()
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(msg, &raw); err == nil {
			if pt, _ := raw["post_type"].(string); pt == "message" {
				dispatch(parseEvent(raw)) // 进入分发器
			}
		}
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Printf("🤖 小牛系统已就绪，分发器监听中...")
	http.ListenAndServe(ListenPort, nil)
}
