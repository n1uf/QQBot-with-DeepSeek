package deepseek

import (
	"strings"

	"QQBot/internal/common"
)

// ShouldHandleAtMasterChat 判断是否应该处理@主人的情况（仅群聊）
func ShouldHandleAtMasterChat(event common.QQEvent) bool {
	if event.MsgType != "group" || event.GroupID == 0 {
		return false
	}
	return event.AtType == common.AtMaster
}

// ShouldHandleAIChat 判断是否应该触发 AI 对话
func ShouldHandleAIChat(event common.QQEvent) bool {
	if event.MsgType == "private" {
		return true
	}
	// 群聊中：@了机器人 或 包含"小牛"关键词
	if event.AtType == common.AtBot {
		return true
	}
	return strings.Contains(event.Content, "小牛")
}
