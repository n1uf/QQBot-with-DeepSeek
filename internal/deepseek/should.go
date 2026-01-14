package deepseek

import (
	"fmt"
	"strings"

	"QQBot/internal/common"
)

// ShouldHandleAtMasterChat 判断是否应该处理@主人的情况（仅群聊）
func ShouldHandleAtMasterChat(event common.QQEvent) bool {
	if event.MsgType != "group" || event.GroupID == 0 || common.MasterQQNumber == 0 {
		return false
	}
	return strings.Contains(event.RawContent, buildAtCode(common.MasterQQNumber))
}

// ShouldHandleAIChat 判断是否应该触发 AI 对话
func ShouldHandleAIChat(event common.QQEvent) bool {
	if event.MsgType == "private" {
		return true
	}
	if common.BotQQNumber > 0 && strings.Contains(event.RawContent, buildAtCode(common.BotQQNumber)) {
		return true
	}
	return strings.Contains(event.Content, "小牛")
}

// buildAtCode 构建@用户的CQ码
func buildAtCode(qqNumber int64) string {
	return fmt.Sprintf("[CQ:at,qq=%d]", qqNumber)
}
