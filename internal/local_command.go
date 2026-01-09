package main

import (
	"log"
	"strings"
)

// HandleLocalCommand 处理本地命令（以 niuf 开头的消息）
func HandleLocalCommand(event QQEvent) {
	log.Printf("[本地] 收到指令: %s", event.Content)
	sendReply(event, "1")
}

// ShouldHandleLocalCommand 判断是否应该处理本地命令
func ShouldHandleLocalCommand(content string) bool {
	return strings.HasPrefix(content, "niuf")
}
