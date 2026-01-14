package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Message 表示一条对话消息
type Message struct {
	Role    string `json:"role"`    // "user" 或 "assistant"
	Content string `json:"content"` // 消息内容
	Time    string `json:"time"`    // 时间戳
	UserID  int64  `json:"user_id"` // 发送者的QQ号（user消息时使用，assistant时为0）
}

// Conversation 表示一个用户的对话历史
type Conversation struct {
	UserID   int64     `json:"user_id"`
	Messages []Message `json:"messages"`
	mu       sync.RWMutex
}

var (
	// 内存中存储的对话历史（仅私聊）
	privateConversations sync.Map // map[int64]*Conversation
)

// GetOrCreateConversation 获取或创建用户的对话历史
func GetOrCreateConversation(userID int64) *Conversation {
	// 先从内存中查找
	if convInterface, ok := privateConversations.Load(userID); ok {
		return convInterface.(*Conversation)
	}

	// 尝试从文件加载
	conv := loadConversationFromFile(userID)
	if conv == nil {
		// 创建新的对话历史
		conv = &Conversation{
			UserID:   userID,
			Messages: make([]Message, 0),
		}
	}

	// 存储到内存
	privateConversations.Store(userID, conv)
	return conv
}

// AddUserMessage 添加用户消息到历史
func (c *Conversation) AddUserMessage(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Messages = append(c.Messages, Message{
		Role:    "user",
		Content: content,
		Time:    time.Now().Format(time.RFC3339),
		UserID:  c.UserID, // 保存QQ号
	})

	// 限制历史长度
	c.limitHistory()
}

// AddAssistantMessage 添加助手回复到历史
func (c *Conversation) AddAssistantMessage(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Messages = append(c.Messages, Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now().Format(time.RFC3339),
		UserID:  0, // assistant消息UserID为0
	})

	// 限制历史长度
	c.limitHistory()

	// 保存到文件
	go c.saveToFile()
}

// GetMessages 获取所有消息（用于 API 调用）
func (c *Conversation) GetMessages() []map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	messages := make([]map[string]string, 0, len(c.Messages))
	for _, msg := range c.Messages {
		messages = append(messages, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return messages
}

// limitHistory 限制历史消息数量
func (c *Conversation) limitHistory() {
	if len(c.Messages) > MaxHistoryMessages {
		// 保留最近的 MaxHistoryMessages 条消息
		c.Messages = c.Messages[len(c.Messages)-MaxHistoryMessages:]
	}
}

// saveToFile 保存对话历史到文件
func (c *Conversation) saveToFile() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 确保目录存在
	if err := os.MkdirAll(HistoryDataDir, 0755); err != nil {
		log.Printf("[对话历史] 创建目录失败: %v", err)
		return
	}

	// 保存到文件
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("user_%d.json", c.UserID))
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("[对话历史] 序列化失败: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[对话历史] 保存文件失败: %v", err)
	}
}

// loadConversationFromFile 从文件加载对话历史
func loadConversationFromFile(userID int64) *Conversation {
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("user_%d.json", userID))

	data, err := os.ReadFile(filename)
	if err != nil {
		// 文件不存在是正常的，返回 nil
		return nil
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		log.Printf("[对话历史] 加载文件失败: %v", err)
		return nil
	}

	return &conv
}
