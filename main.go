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
	ListenPort     = ":8080"
	BotQQNumber    = 1851469506
	MasterQQNumber = 2318607163
	// TODO 建议通过环境变量获取：os.Getenv("DEEPSEEK_API_KEY")
	DeepSeekAPIKey  = "DEEPSEEK_API_KEY"
	DeepSeekBaseURL = "https://api.deepseek.com/chat/completions"
)

type QQEvent struct {
	MsgType    string
	UserID     int64
	GroupID    int64
	Content    string // 过滤过CQ码消息后的内容
	RawContent string // 原始消息
}

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsConn   *websocket.Conn
	connMu   sync.Mutex
)

// --- 逻辑分发器 ---

func dispatch(event QQEvent) {
	// 1. 本地指令
	if strings.HasPrefix(event.Content, "niuf") {
		handleLocalCommand(event)
		return
	}

	// 2. 走AI
	// 1) 构造精准的艾特标识
	atMeCode := fmt.Sprintf("[CQ:at,qq=%d]", BotQQNumber)

	// 2) 判定触发条件
	isPrivate := event.MsgType == "private"
	isAtMe := strings.Contains(event.RawContent, atMeCode) // 严格匹配艾特标签
	isCalledMe := strings.Contains(event.Content, "小牛")  // 匹配名字

	// 3) 汇总触发状态
	shouldRespond := isPrivate || isAtMe || isCalledMe

	if shouldRespond {
		if event.Content == "" {
			sendReply(event, "干嘛？艾特我又不说话，是不是想我了？")
			return
		}
		go handleAIChat(event)
	}
}

func handleLocalCommand(e QQEvent) {
	log.Printf("[本地] 收到指令: %s", e.Content)
	sendReply(e, "1")
}

func handleAIChat(e QQEvent) {
	var hint string
	if e.UserID == MasterQQNumber {
		hint = "当前说话的是你的主人 niuf，对他要亲切一点。"
	} else {
		hint = "当前说话的是一位普通好友，保持礼貌即可。"
	}

	log.Printf("[收到] <- 用户:%d 内容:%s", e.UserID, e.Content)

	answer, err := callDeepSeek(e.Content, hint)
	if err != nil {
		log.Printf("[AI] 出错: %v", err)
		sendReply(e, "小牛有点累了，稍后再试吧...")
		return
	}

	sendReply(e, answer)
}

// --- DeepSeek API 调用 ---

func callDeepSeek(content string, roleHint string) (string, error) {
	systemMessage := fmt.Sprintf("你是一个幽默的助手小牛。你的主人是 niuf。%s", roleHint)

	payload := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemMessage},
			{"role": "user", "content": content},
		},
		"temperature": 0.7,
	}

	requestBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", DeepSeekBaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: 60 * time.Second} // 增加超时时间
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误: %s", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "我不知道该怎么回答呢。", nil
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
	http.HandleFunc("/ws", wsHandler)
	log.Printf("🤖 小牛系统已就绪，端口%s", ListenPort)
	if err := http.ListenAndServe(ListenPort, nil); err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}
