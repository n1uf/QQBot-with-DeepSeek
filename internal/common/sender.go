package common

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	wsConn *websocket.Conn
	connMu sync.Mutex
)

// SetWebSocketConn 设置WebSocket连接（由main.go调用）
func SetWebSocketConn(conn *websocket.Conn) {
	connMu.Lock()
	defer connMu.Unlock()
	wsConn = conn
}

// GetWebSocketConn 获取WebSocket连接
func GetWebSocketConn() *websocket.Conn {
	connMu.Lock()
	defer connMu.Unlock()
	return wsConn
}

// ClearWebSocketConn 清除WebSocket连接
func ClearWebSocketConn() {
	connMu.Lock()
	defer connMu.Unlock()
	wsConn = nil
}

// SendReply 发送回复消息
func SendReply(e QQEvent, text string) {
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
