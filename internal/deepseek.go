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

// ============================================================================
// 常量定义
// ============================================================================

const (
	// API 配置
	deepSeekModel       = "deepseek-chat"
	deepSeekTemperature = 1.5
	apiTimeout          = 60 * time.Second

	// 错误消息
	errorMessage = "小牛有点累了，稍后再试吧..."

	// System prompt 基础部分
	systemPromptBase  = "你是一个幽默的助手小牛。你的主人是 niuf。"
	naturalSpeechHint = "\n\n请像人类一样自然地说话，不要分点论述，不要用列表格式，用自然的对话方式回复。"
	groupChatContext  = "\n\n你正在一个QQ群聊中。群聊中的用户用昵称标识。注意：群聊消息中的\"你\"指的是你自己（小牛）；群聊消息中消息格式为[昵称]: 消息内容；昵称中的括号标识了特殊用户的身份，例如（主人）、（主人的女朋友）。"
)

// ============================================================================
// 内部辅助函数
// ============================================================================

// getUserRoleHint 根据用户ID获取角色提示
func getUserRoleHint(userID int64) string {
	switch userID {
	case MasterQQNumber:
		return "当前说话的是你的主人 niuf，对他要亲切一点。"
	case MasterGirlFriendQQNumber:
		return "当前说话的是你主人 niuf 的女朋友，你的主人很爱她，你要尊重她。"
	default:
		return "当前说话的是一位普通好友，保持礼貌即可。"
	}
}

// handleAIError 统一处理 AI 错误
func handleAIError(event QQEvent, err error) {
	log.Printf("[AI] 出错: %v", err)
	sendReply(event, errorMessage)
}

// HandleAIChat 处理 AI 对话请求
func HandleAIChat(event QQEvent) {
	hint := getUserRoleHint(event.UserID)
	log.Printf("[收到] <- 用户:%d 内容:%s", event.UserID, event.Content)

	var answer string
	var err error

	switch {
	case event.MsgType == "private":
		answer, err = callDeepSeekWithPrivateHistory(event.UserID, event.Content, hint)
	case event.MsgType == "group" && event.GroupID > 0:
		answer, err = callDeepSeekWithGroupContext(event.GroupID, event.UserID, event.Content, hint)
	default:
		// 其他消息类型，使用简单调用
		answer, err = callDeepSeekSimple(event.Content, hint)
	}

	if err != nil {
		handleAIError(event, err)
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

	answer, err := callDeepSeekWithGroupContext(event.GroupID, event.UserID, content, hint)
	if err != nil {
		handleAIError(event, err)
		return
	}

	sendReply(event, answer)
}

// ============================================================================
// 事件处理函数（对外接口）
// ============================================================================

// ShouldHandleAtMasterChat 判断是否应该处理@主人的情况（仅群聊）
func ShouldHandleAtMasterChat(event QQEvent) bool {
	if event.MsgType != "group" || event.GroupID == 0 || MasterQQNumber == 0 {
		return false
	}
	return strings.Contains(event.RawContent, buildAtCode(MasterQQNumber))
}

// ShouldHandleAIChat 判断是否应该触发 AI 对话
func ShouldHandleAIChat(event QQEvent) bool {
	if event.MsgType == "private" {
		return true
	}
	if BotQQNumber > 0 && strings.Contains(event.RawContent, buildAtCode(BotQQNumber)) {
		return true
	}
	return strings.Contains(event.Content, "小牛")
}

// buildAtCode 构建@用户的CQ码
func buildAtCode(qqNumber int64) string {
	return fmt.Sprintf("[CQ:at,qq=%d]", qqNumber)
}

// buildSystemMessage 构建系统提示词
func buildSystemMessage(isGroupChat bool, roleHint string) string {
	message := systemPromptBase
	if isGroupChat {
		message += groupChatContext
	}
	if roleHint != "" {
		message += " " + roleHint
	}
	message += naturalSpeechHint
	return message
}

// debugPrintMessages 打印发送给AI的完整消息（用于调试）
func debugPrintMessages(messages []map[string]string, chatType string) {
	messagesJSON, _ := json.MarshalIndent(messages, "", "  ")
	fmt.Printf("\n========== %s - 发送给AI的完整消息 ==========\n", chatType)
	fmt.Printf("%s\n", messagesJSON)
	fmt.Printf("==========================================\n\n")
}

// ============================================================================
// DeepSeek API 调用函数
// ============================================================================

// callDeepSeekWithPrivateHistory 调用 DeepSeek API（带私聊对话历史）
func callDeepSeekWithPrivateHistory(userID int64, content string, roleHint string) (string, error) {
	conv := getOrCreateConversation(userID)
	systemMessage := buildSystemMessage(false, roleHint)

	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
	}
	messages = append(messages, conv.getMessages()...)
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": content,
	})

	debugPrintMessages(messages, "私聊AI")

	answer, err := callDeepSeekAPI(messages)
	if err != nil {
		return "", err
	}

	conv.addUserMessage(content)
	conv.addAssistantMessage(answer)
	return answer, nil
}

// callDeepSeekWithGroupContext 调用 DeepSeek API（使用群聊上下文，用于群聊）
func callDeepSeekWithGroupContext(groupID int64, userID int64, content string, roleHint string) (string, error) {
	systemMessage := buildSystemMessage(true, roleHint)
	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
	}

	// 先添加当前消息到上下文（这样它就会成为最后一条）
	addGroupContextMessage(groupID, userID, content)

	// 获取群聊上下文（除最后一条外的所有消息）和最后一条消息
	groupContext, lastMsg := getGroupContextForAI(groupID)

	if groupContext != "" {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": groupContext,
		})
	}

	// 添加最后一条消息（当前消息，使用昵称）
	if lastMsg != nil {
		nickname := getNickname(groupID, lastMsg.UserID)
		currentMsg := fmt.Sprintf("[%s]: %s", nickname, lastMsg.Content)
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": currentMsg,
		})
	}

	debugPrintMessages(messages, "群聊AI")

	answer, err := callDeepSeekAPI(messages)
	if err != nil {
		return "", err
	}

	// 将AI回复添加到群聊上下文
	addGroupContextMessage(groupID, BotQQNumber, answer)
	return answer, nil
}

// callDeepSeekSimple 调用 DeepSeek API（简单调用，不带历史）
func callDeepSeekSimple(content string, roleHint string) (string, error) {
	systemMessage := buildSystemMessage(false, roleHint)
	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
		{"role": "user", "content": content},
	}
	return callDeepSeekAPI(messages)
}

// callDeepSeekAPI 实际调用 DeepSeek API
func callDeepSeekAPI(messages []map[string]string) (string, error) {
	payload := map[string]interface{}{
		"model":       deepSeekModel,
		"messages":    messages,
		"temperature": deepSeekTemperature,
	}

	requestBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", DeepSeekBaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+DeepSeekAPIKey)

	client := &http.Client{Timeout: apiTimeout}
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
