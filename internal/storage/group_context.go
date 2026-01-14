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

var (
	// 群聊上下文（持久化）
	groupContexts sync.Map // map[int64]*GroupContext
)

// AddGroupContextMessage 添加群聊消息到上下文
func AddGroupContextMessage(groupID int64, userID int64, content string) {
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

// GetGroupContextForAI 获取群聊上下文（转换为AI格式）
// 返回除最后一条外的所有消息作为上下文，最后一条作为当前消息
func GetGroupContextForAI(groupID int64) (context string, lastMessage *GroupContextMessage) {
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

	// 构建上下文消息（使用元数据标签格式）
	contextMsg := "群聊消息：\n"
	for _, msg := range contextMessages {
		roleTag := GetRoleTag(msg.UserID)
		nickname := GetNickname(groupID, msg.UserID)
		contextMsg += fmt.Sprintf("【%s】%s 发言说: %s\n", roleTag, nickname, msg.Content)
	}

	return contextMsg, lastMsg
}

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
