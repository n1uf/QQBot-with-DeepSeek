package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// HandleAIChat 处理 AI 对话请求
func HandleAIChat(event QQEvent) {
	var hint string
	switch event.UserID {
	case MasterQQNumber:
		hint = "当前说话的是你的主人 niuf，对他要亲切一点。"
	case MasterGirlFriendQQNumber:
		hint = "当前说话的是你主人 niuf 的女朋友，你的主人很爱她，你要尊重她。"
	default:
		hint = "当前说话的是一位普通好友，保持礼貌即可。"
	}

	log.Printf("[收到] <- 用户:%d 内容:%s", event.UserID, event.Content)

	// 私聊时使用对话历史，群聊时不使用
	var answer string
	var err error
	if event.MsgType == "private" {
		answer, err = callDeepSeekWithHistory(event.UserID, event.Content, hint)
	} else {
		answer, err = callDeepSeek(event.Content, hint)
	}

	if err != nil {
		log.Printf("[AI] 出错: %v", err)
		sendReply(event, "小牛有点累了，稍后再试吧...")
		return
	}

	sendReply(event, answer)
}

// HandleAtMasterChat 处理群聊中@主人的情况
func HandleAtMasterChat(event QQEvent) {
	hint := "当前有人在群里@了你的主人 niuf，请帮他回复一下，语气要友好。"

	log.Printf("[@主人] <- 群:%d 用户:%d 内容:%s", event.GroupID, event.UserID, event.Content)

	// 如果消息为空，给个默认回复
	content := event.Content
	if content == "" {
		content = "有人@了主人"
	}

	answer, err := callDeepSeek(content, hint)
	if err != nil {
		log.Printf("[AI] 出错: %v", err)
		sendReply(event, "小牛有点累了，稍后再试吧...")
		return
	}

	sendReply(event, answer)
}

// ShouldHandleAtMasterChat 判断是否应该处理@主人的情况（仅群聊）
func ShouldHandleAtMasterChat(event QQEvent) bool {
	// 只处理群聊
	if event.MsgType != "group" || event.GroupID == 0 {
		return false
	}

	// 检查是否设置了主人QQ号
	if MasterQQNumber == 0 {
		return false
	}

	// 构造@主人的CQ码
	atMasterCode := fmt.Sprintf("[CQ:at,qq=%d]", MasterQQNumber)

	// 检查是否@了主人
	return strings.Contains(event.RawContent, atMasterCode)
}

// ShouldHandleAIChat 判断是否应该触发 AI 对话
func ShouldHandleAIChat(event QQEvent) bool {
	// 构造精准的艾特标识
	atMeCode := fmt.Sprintf("[CQ:at,qq=%d]", BotQQNumber)

	// 判定触发条件
	isPrivate := event.MsgType == "private"
	isAtMe := strings.Contains(event.RawContent, atMeCode) // 严格匹配艾特标签
	isCalledMe := strings.Contains(event.Content, "小牛")    // 匹配名字

	// 汇总触发状态
	return isPrivate || isAtMe || isCalledMe
}

// callDeepSeekWithHistory 调用 DeepSeek API（带对话历史，用于私聊）
func callDeepSeekWithHistory(userID int64, content string, roleHint string) (string, error) {
	// 获取对话历史
	conv := getOrCreateConversation(userID)

	// 构建消息列表
	systemMessage := fmt.Sprintf("你是一个幽默的助手小牛。你的主人是 niuf。%s", roleHint)
	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
	}

	// 添加历史消息（不包括当前消息）
	messages = append(messages, conv.getMessages()...)

	// 添加当前用户消息
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": content,
	})

	// 调用 API
	answer, err := callDeepSeekAPI(messages)
	if err != nil {
		return "", err
	}

	// 添加用户消息和助手回复到历史
	conv.addUserMessage(content)
	conv.addAssistantMessage(answer)

	return answer, nil
}

// callDeepSeek 调用 DeepSeek API（不带历史，用于群聊）
func callDeepSeek(content string, roleHint string) (string, error) {
	systemMessage := fmt.Sprintf("你是一个幽默的助手小牛。你的主人是 niuf。%s", roleHint)

	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
		{"role": "user", "content": content},
	}

	return callDeepSeekAPI(messages)
}

// callDeepSeekAPI 实际调用 DeepSeek API
func callDeepSeekAPI(messages []map[string]string) (string, error) {
	payload := map[string]interface{}{
		"model":       "deepseek-chat",
		"messages":    messages,
		"temperature": 0.7,
	}

	requestBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", DeepSeekBaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
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
