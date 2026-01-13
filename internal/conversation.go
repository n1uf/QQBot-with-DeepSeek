package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MaxHistoryMessages      = 50     // 最多保留的历史消息数量（私聊和群聊对话历史）
	MaxGroupContextMessages = 50     // 群聊上下文最多保留的消息数量
	MaxMessageLength        = 500    // 单条消息最大字符数，超过此长度的消息不加入上下文
	HistoryDataDir          = "data" // 历史数据存储目录
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

// GroupContextMessage 群聊上下文消息（短期，包含所有用户）
type GroupContextMessage struct {
	UserID  int64  `json:"user_id"` // QQ号
	Content string `json:"content"` // 消息内容
	Time    string `json:"time"`    // 时间戳
}

// GroupContext 群聊上下文（持久化，用于理解当前对话）
type GroupContext struct {
	GroupID  int64                 `json:"group_id"`
	Messages []GroupContextMessage `json:"messages"`
	mu       sync.RWMutex
}

// GroupNicknameMap 群昵称映射（持久化）
type GroupNicknameMap struct {
	GroupID   int64            `json:"group_id"`
	Nicknames map[int64]string `json:"nicknames"` // QQ号 -> 昵称
	mu        sync.RWMutex
}

var (
	// 内存中存储的对话历史（仅私聊）
	privateConversations sync.Map // map[int64]*Conversation

	// 群聊上下文（持久化）
	groupContexts sync.Map // map[int64]*GroupContext

	// 昵称映射（按群分开，持久化）
	groupNicknameMap sync.Map // map[int64]*GroupNicknameMap
)

// getOrCreateConversation 获取或创建用户的对话历史
func getOrCreateConversation(userID int64) *Conversation {
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

// addUserMessage 添加用户消息到历史
func (c *Conversation) addUserMessage(content string) {
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

// addAssistantMessage 添加助手回复到历史
func (c *Conversation) addAssistantMessage(content string) {
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

// getMessages 获取所有消息（用于 API 调用）
func (c *Conversation) getMessages() []map[string]string {
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

// clearConversation 清空用户的对话历史（用于测试或重置）
// 注意：此函数当前未被使用，保留用于调试或管理功能
func clearConversation(userID int64) {
	privateConversations.Delete(userID)
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("user_%d.json", userID))
	os.Remove(filename)
	log.Printf("[对话历史] 已清空用户 %d 的对话历史", userID)
}

// --- 昵称映射管理（持久化）---

// getOrCreateGroupNicknameMap 获取或创建群的昵称映射（从文件加载）
func getOrCreateGroupNicknameMap(groupID int64) *GroupNicknameMap {
	// 先从内存中查找
	if mapInterface, ok := groupNicknameMap.Load(groupID); ok {
		return mapInterface.(*GroupNicknameMap)
	}

	// 尝试从文件加载
	nm := loadGroupNicknameMapFromFile(groupID)
	if nm == nil {
		// 创建新的昵称映射
		nm = &GroupNicknameMap{
			GroupID:   groupID,
			Nicknames: make(map[int64]string),
		}
	}

	// 存储到内存
	groupNicknameMap.Store(groupID, nm)
	return nm
}

// updateNicknameMap 更新昵称映射（按群分开，自动保存）
func updateNicknameMap(groupID int64, userID int64, nickname string) {
	if nickname == "" || groupID == 0 {
		return
	}

	nm := getOrCreateGroupNicknameMap(groupID)
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 检查是否有变化
	if oldNickname, exists := nm.Nicknames[userID]; exists && oldNickname == nickname {
		return // 没有变化，不需要保存
	}

	nm.Nicknames[userID] = nickname

	// 异步保存到文件
	go nm.saveToFile()
}

// getNickname 获取用户昵称，如果不存在则返回稳定标识符
// groupID=0 表示私聊，直接返回稳定标识符（私聊不需要昵称）
func getNickname(groupID int64, userID int64) string {
	var nickname string

	if groupID == 0 {
		// 私聊直接返回稳定标识符
		nickname = getUserStableID(userID)
	} else {
		// 获取该群的昵称映射
		nm := getOrCreateGroupNicknameMap(groupID)
		nm.mu.RLock()
		if n, ok := nm.Nicknames[userID]; ok && n != "" {
			nickname = n
		} else {
			nickname = getUserStableID(userID)
		}
		nm.mu.RUnlock()
	}

	// 特殊处理：如果是主人女朋友，在昵称后加上标识
	if userID == MasterGirlFriendQQNumber && MasterGirlFriendQQNumber > 0 {
		return fmt.Sprintf("%s（主人的女朋友）", nickname)
	} else if userID == MasterQQNumber && MasterQQNumber > 0 {
		return fmt.Sprintf("%s（主人）", nickname)
	}

	return nickname
}

// getUserStableID 获取用户的稳定标识符（基于QQ号的哈希，如"用户AA"）
// 使用MD5哈希确保同一QQ号总是得到相同的标识符
// 使用两个字母（AA-ZZ）可以支持最多676个用户
func getUserStableID(userID int64) string {
	// 将QQ号转换为字符串并计算MD5哈希
	userIDStr := fmt.Sprintf("%d", userID)
	hash := md5.Sum([]byte(userIDStr))

	// 使用哈希的前4个字节计算两个0-25的值（对应A-Z）
	// 第一个字母：使用前2个字节
	hashValue1 := uint32(hash[0])<<8 | uint32(hash[1])
	letterIndex1 := int(hashValue1 % 26)

	// 第二个字母：使用后2个字节
	hashValue2 := uint32(hash[2])<<8 | uint32(hash[3])
	letterIndex2 := int(hashValue2 % 26)

	// 生成标识符（用户AA-ZZ，支持676个用户）
	stableID := fmt.Sprintf("用户%c%c", 'A'+letterIndex1, 'A'+letterIndex2)
	return stableID
}

// --- 群聊上下文管理（持久化）---

// getOrCreateGroupContext 获取或创建群聊上下文（从文件加载）
func getOrCreateGroupContext(groupID int64) *GroupContext {
	// 先从内存中查找
	if ctxInterface, ok := groupContexts.Load(groupID); ok {
		return ctxInterface.(*GroupContext)
	}

	// 尝试从文件加载
	ctx := loadGroupContextFromFile(groupID)
	if ctx == nil {
		// 创建新的上下文
		ctx = &GroupContext{
			GroupID:  groupID,
			Messages: make([]GroupContextMessage, 0, MaxGroupContextMessages),
		}
		log.Printf("[DEBUG] [群聊上下文] 创建新上下文: 群%d", groupID)
	} else {
		log.Printf("[DEBUG] [群聊上下文] 加载上下文: 群%d, 消息数 %d", groupID, len(ctx.Messages))
	}

	// 存储到内存
	groupContexts.Store(groupID, ctx)
	return ctx
}

// addGroupContextMessage 添加群聊消息到上下文
func addGroupContextMessage(groupID int64, userID int64, content string) {
	if groupID == 0 || content == "" {
		return
	}

	// 过滤超长消息（不加入上下文）
	if len([]rune(content)) > MaxMessageLength {
		log.Printf("[DEBUG] [群聊上下文] 群%d: 消息过长（%d字符），已过滤", groupID, len([]rune(content)))
		return
	}

	ctx := getOrCreateGroupContext(groupID)

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// 添加消息
	ctx.Messages = append(ctx.Messages, GroupContextMessage{
		UserID:  userID,
		Content: content,
		Time:    time.Now().Format(time.RFC3339),
	})

	// 限制长度
	if len(ctx.Messages) > MaxGroupContextMessages {
		log.Printf("[DEBUG] [群聊上下文] 群%d: 消息数 %d -> %d (截断)", groupID, len(ctx.Messages), MaxGroupContextMessages)
		ctx.Messages = ctx.Messages[len(ctx.Messages)-MaxGroupContextMessages:]
	}

	// 保存到文件
	go ctx.saveToFile()
	log.Printf("[DEBUG] [群聊上下文] 群%d: 消息数 %d", groupID, len(ctx.Messages))
}

// getGroupContextForAI 获取群聊上下文（转换为AI格式）
// 返回除最后一条外的所有消息作为上下文，最后一条作为当前消息
func getGroupContextForAI(groupID int64) (context string, lastMessage *GroupContextMessage) {
	ctx := getOrCreateGroupContext(groupID)

	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	if len(ctx.Messages) == 0 {
		return "", nil
	}

	// 如果只有一条消息，没有上下文，只有当前消息
	if len(ctx.Messages) == 1 {
		return "", &ctx.Messages[0]
	}

	// 除最后一条外的所有消息作为上下文
	contextMessages := ctx.Messages[:len(ctx.Messages)-1]
	lastMsg := &ctx.Messages[len(ctx.Messages)-1]

	// 构建上下文消息
	contextMsg := "群聊消息：\n"
	for _, msg := range contextMessages {
		// 如果是AI自己的消息，用"你:"标识
		if msg.UserID == BotQQNumber || msg.UserID == 0 {
			contextMsg += fmt.Sprintf("- 你: %s\n", msg.Content)
		} else {
			nickname := getNickname(groupID, msg.UserID)
			contextMsg += fmt.Sprintf("- [%s]: %s\n", nickname, msg.Content)
		}
	}

	return contextMsg, lastMsg
}

// saveToFile 保存群聊上下文到文件
func (c *GroupContext) saveToFile() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 确保目录存在
	if err := os.MkdirAll(HistoryDataDir, 0755); err != nil {
		log.Printf("[群聊上下文] 创建目录失败: %v", err)
		return
	}

	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d.json", c.GroupID))
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Printf("[群聊上下文] 序列化失败: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[群聊上下文] 保存文件失败: %v", err)
	}
	log.Printf("[DEBUG] [群聊上下文] 群%d: 已保存到文件", c.GroupID)
}

// loadGroupContextFromFile 从文件加载群聊上下文
func loadGroupContextFromFile(groupID int64) *GroupContext {
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d.json", groupID))

	data, err := os.ReadFile(filename)
	if err != nil {
		// 文件不存在是正常的
		return nil
	}

	var ctx GroupContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		log.Printf("[群聊上下文] 加载文件失败: %v", err)
		return nil
	}

	return &ctx
}

// saveToFile 保存群昵称映射到文件
func (nm *GroupNicknameMap) saveToFile() {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// 确保目录存在
	if err := os.MkdirAll(HistoryDataDir, 0755); err != nil {
		log.Printf("[昵称映射] 创建目录失败: %v", err)
		return
	}

	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d_nicknames.json", nm.GroupID))
	data, err := json.MarshalIndent(nm, "", "  ")
	if err != nil {
		log.Printf("[昵称映射] 序列化失败: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[昵称映射] 保存文件失败: %v", err)
	}
}

// loadGroupNicknameMapFromFile 从文件加载群昵称映射
func loadGroupNicknameMapFromFile(groupID int64) *GroupNicknameMap {
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d_nicknames.json", groupID))

	data, err := os.ReadFile(filename)
	if err != nil {
		// 文件不存在是正常的
		return nil
	}

	var nm GroupNicknameMap
	if err := json.Unmarshal(data, &nm); err != nil {
		log.Printf("[昵称映射] 加载文件失败: %v", err)
		return nil
	}

	return &nm
}
