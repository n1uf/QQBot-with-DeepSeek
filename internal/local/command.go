package local

import (
	"log"

	"QQBot/internal/common"
)

// HandleLocalCommand 处理本地命令（以 niuf 开头的消息）
func HandleLocalCommand(event common.QQEvent) {
	log.Printf("[本地] 收到指令: %s", event.Content)
	common.SendReply(event, "1")
}

// ShouldHandleLocalCommand 判断是否应该处理本地命令
func ShouldHandleLocalCommand(content string) bool {
	return content == "小牛"
}
