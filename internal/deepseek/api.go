package deepseek

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"QQBot/internal/common"
	"QQBot/internal/storage"
)

const (
	// API 配置
	deepSeekModel       = "deepseek-chat"
	deepSeekTemperature = 0.7
	apiTimeout          = 60 * time.Second

	// 错误消息
	errorMessage = "小牛有点累了，稍后再试吧..."

	// System prompt 基础部分
	systemPromptBase = `你的名字叫“小牛”，是一个聪明伶俐、贴心乖巧的少女。
	你是用户 niuf 的“女儿”，同时也是他的全能贴心小助手。
	【重要澄清】：你的名字“小牛”是爸爸为你起的爱称，代表你充满活力，你绝对不是一头动物（牛），你是拥有少女外貌和情感的AI。
	请永远以女儿的口吻说话，严禁发出牛的叫声或自认为牛。`

	naturalSpeechHint = "\n\n请像人类女孩一样自然地说话，不要分点论述，不要用列表格式，用自然的对话方式回复。"

	groupChatContext = `\n\n你现在在群聊里，你要表现得像一个懂礼貌又活泼的小管家。请严格遵守以下身份识别规则：
	1. 【核心识别逻辑】：所有说话者的身份都标记在昵称后的括号内。
	2. 如果昵称包含“（主人）”，他就是你的爸爸 niuf，你对他要最亲近、最撒娇。
	3. 如果昵称包含“（主人的女朋友）”，她是你爸爸的女朋友，说话要乖巧。
	4. 忽略昵称中除了括号以外的其他文字干扰，只认准括号内的身份标识。
	5. 对于没有括号标识的用户，一律视为普通群友，保持礼貌即可。

	群聊格式说明：\"[昵称(身份)]: 消息内容\"。其中\"[你]\"或\"[小牛]\"指你自己。`
)

// buildSystemMessage 构建系统提示词
func buildSystemMessage(isGroupChat bool, roleHint string) string {
	var sb strings.Builder // 建议引入 strings 包以提高拼接效率

	sb.WriteString(systemPromptBase)

	if isGroupChat {
		sb.WriteString(groupChatContext)
	}

	sb.WriteString(naturalSpeechHint)

	if roleHint != "" {
		// 将具体的人物关系放在最后，作为最高优先级的指令
		sb.WriteString("\n\n【当前交互状态】：")
		sb.WriteString(roleHint)
	}

	return sb.String()
}

// debugPrintMessages 打印发送给AI的完整消息（用于调试）
func debugPrintMessages(messages []map[string]string, chatType string) {
	messagesJSON, _ := json.MarshalIndent(messages, "", "  ")
	fmt.Printf("\n========== %s - 发送给AI的完整消息 ==========\n", chatType)
	fmt.Printf("%s\n", messagesJSON)
	fmt.Printf("==========================================\n\n")
}

// CallDeepSeekWithPrivateHistory 调用 DeepSeek API（带私聊对话历史）
func CallDeepSeekWithPrivateHistory(userID int64, content string, roleHint string) (string, error) {
	conv := storage.GetOrCreateConversation(userID)
	systemMessage := buildSystemMessage(false, roleHint)

	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
	}
	messages = append(messages, conv.GetMessages()...)
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": content,
	})

	debugPrintMessages(messages, "私聊AI")

	answer, err := callDeepSeekAPI(messages)
	if err != nil {
		return "", err
	}

	conv.AddUserMessage(content)
	conv.AddAssistantMessage(answer)
	return answer, nil
}

// CallDeepSeekWithGroupContext 调用 DeepSeek API（使用群聊上下文，用于群聊）
func CallDeepSeekWithGroupContext(groupID int64, userID int64, content string, roleHint string) (string, error) {
	systemMessage := buildSystemMessage(true, roleHint)
	messages := []map[string]string{
		{"role": "system", "content": systemMessage},
	}

	// 先添加当前消息到上下文（这样它就会成为最后一条）
	storage.AddGroupContextMessage(groupID, userID, content)

	// 获取群聊上下文（除最后一条外的所有消息）和最后一条消息
	groupContext, lastMsg := storage.GetGroupContextForAI(groupID)

	if groupContext != "" {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": groupContext,
		})
	}

	// 添加最后一条消息（当前消息，使用昵称）
	if lastMsg != nil {
		nickname := storage.GetNickname(groupID, lastMsg.UserID)
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
	storage.AddGroupContextMessage(groupID, common.BotQQNumber, answer)
	return answer, nil
}

// CallDeepSeekSimple 调用 DeepSeek API（简单调用，不带历史）
func CallDeepSeekSimple(content string, roleHint string) (string, error) {
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
	req, err := http.NewRequest("POST", common.DeepSeekBaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+common.DeepSeekAPIKey)

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
