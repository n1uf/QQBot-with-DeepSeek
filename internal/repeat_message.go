package main

import (
	"log"
	"sync"
)

// 消息队列结构，用于检测连续相同消息
type messageQueue struct {
	messages []string
	mu       sync.RWMutex
}

var (
	groupQueues sync.Map // map[int64]*messageQueue，存储每个群的消息队列
)

// HandleRepeatMessage 处理连续相同消息检测
// 返回 true 表示已处理（发送了重复消息），false 表示未触发
func HandleRepeatMessage(event QQEvent) bool {
	// 1. 过滤条件：跳过机器人自己的消息、空消息
	if event.UserID == BotQQNumber || event.Content == "" {
		return false
	}

	// 2. 获取或创建该群的消息队列
	queueInterface, _ := groupQueues.LoadOrStore(event.GroupID, &messageQueue{
		messages: make([]string, 0, RepeatMessageQueueSize),
	})
	queue := queueInterface.(*messageQueue)

	// 3. 更新队列并检查
	queue.mu.Lock()
	defer queue.mu.Unlock()

	// 添加新消息到队列
	queue.messages = append(queue.messages, event.Content)

	// 保持队列大小不超过 RepeatMessageQueueSize
	if len(queue.messages) > RepeatMessageQueueSize {
		queue.messages = queue.messages[len(queue.messages)-RepeatMessageQueueSize:]
	}

	// 4. 检查是否达到触发条件（队列中所有消息都相同）
	if len(queue.messages) == RepeatMessageQueueSize {
		allSame := true
		firstMsg := queue.messages[0]
		for i := 1; i < len(queue.messages); i++ {
			if queue.messages[i] != firstMsg {
				allSame = false
				break
			}
		}

		if allSame {
			// 清空队列，避免重复触发
			queue.messages = queue.messages[:0]
			log.Printf("[重复消息] 群 %d 检测到连续 %d 条相同消息: %s", event.GroupID, RepeatMessageQueueSize, firstMsg)
			// 发送相同消息
			sendReply(event, firstMsg)
			return true
		}
	}

	return false
}

// ShouldHandleRepeatMessage 判断是否应该处理重复消息检测（仅群聊）
func ShouldHandleRepeatMessage(event QQEvent) bool {
	return event.MsgType == "group" && event.GroupID > 0
}
