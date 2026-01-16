package deepseek

import (
	"log"

	"QQBot/internal/common"
)

// HandleAIChat 处理 AI 对话请求
func HandleAIChat(event common.QQEvent) {
	hint := getUserRoleHint(event.UserID)
	log.Printf("[收到] <- 用户:%d 内容:%s", event.UserID, event.Content)

	var answer string
	var err error

	switch {
	case event.MsgType == "private":
		answer, err = CallDeepSeekWithPrivateHistory(event.UserID, event.Content, hint)
	case event.MsgType == "group" && event.GroupID > 0:
		answer, err = CallDeepSeekWithGroupContext(event.GroupID, event.UserID, event.Content, hint)
	default:
		// 其他消息类型，使用简单调用
		answer, err = CallDeepSeekSimple(event.Content, hint)
	}

	if err != nil {
		handleAIError(event, err)
		return
	}

	common.SendReply(event, answer)
}

// HandleAtMasterChat 处理群聊中@主人的情况
func HandleAtMasterChat(event common.QQEvent) {
	hint := "当前有人在群里@了你的主人（爸爸） niuf ，你需要转告给 niuf ，并总结一下群友@niuf的原因"

	log.Printf("[@主人] <- 群:%d 用户:%d 内容:%s", event.GroupID, event.UserID, event.Content)

	content := event.Content
	if content == "" {
		content = "@了你的主人（爸爸）"
	}

	answer, err := CallDeepSeekWithGroupContext(event.GroupID, event.UserID, content, hint)
	if err != nil {
		handleAIError(event, err)
		return
	}

	// 构造私信事件，发送给主人
	privateEvent := common.QQEvent{
		MsgType: "private",
		UserID:  common.MasterQQNumber,
		GroupID: 0,
		Content: answer,
	}
	common.SendReply(privateEvent, answer)
}

// getUserRoleHint 根据用户ID获取角色提示
func getUserRoleHint(userID int64) string {
	switch userID {
	case common.MasterQQNumber:
		// 对爸爸的特殊态度
		return "现在说话的是你的爸爸 niuf。"
	case common.MasterGirlFriendQQNumber:
		// 对爸爸女朋友的态度
		return "现在说话的是爸爸的女朋友，要有礼貌"
	default:
		// 对普通人的态度
		return "现在说话的是爸爸的朋友。请乖巧懂事，并礼貌地提供帮助。"
	}
}

// handleAIError 统一处理 AI 错误
func handleAIError(event common.QQEvent, err error) {
	log.Printf("[AI] 出错: %v", err)
	common.SendReply(event, errorMessage)
}
