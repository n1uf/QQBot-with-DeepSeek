package storage

import (
	"crypto/md5"
	"fmt"

	"QQBot/internal/common"
)

// GetRoleTag 根据 userID 获取角色标签（用于身份识别）
func GetRoleTag(userID int64) string {
	if userID == common.MasterQQNumber && common.MasterQQNumber > 0 {
		return "你的爸爸/主人"
	} else if userID == common.MasterGirlFriendQQNumber && common.MasterGirlFriendQQNumber > 0 {
		return "爸爸的女朋友"
	} else if userID == common.BotQQNumber || userID == 0 {
		return "你"
	}
	return "普通群友"
}

// FormatAtMessage 格式化 @ 消息：@【角色标签】昵称
func FormatAtMessage(groupID int64, userID int64) string {
	roleTag := GetRoleTag(userID)
	nickname := GetNickname(groupID, userID)
	return fmt.Sprintf("@【%s】%s", roleTag, nickname)
}

// FormatGroupMessage 格式化群聊消息：【角色标签】昵称 发言说: 内容
func FormatGroupMessage(groupID int64, userID int64, content string) string {
	roleTag := GetRoleTag(userID)
	nickname := GetNickname(groupID, userID)
	return fmt.Sprintf("【%s】%s 发言说: %s", roleTag, nickname, content)
}

// getUserStableID 获取用户的稳定标识符（基于QQ号的哈希，如"用户AA"）
// 使用MD5哈希确保同一QQ号总是得到相同的标识符
// 使用两个字母（AA-ZZ）可以支持最多676个用户
func getUserStableID(userID int64) string {
	// 将QQ号转换为字符串并计算MD5哈希
	userIDStr := fmt.Sprintf("%d", userID)
	hash := md5.Sum([]byte(userIDStr))

	// 使用哈希的前4个字节计算两个0-25的值（对应A-Z）
	// 第一个字母：使用前2个字节
	hashValue1 := uint32(hash[0])<<8 | uint32(hash[1])
	letterIndex1 := int(hashValue1 % 26)

	// 第二个字母：使用后2个字节
	hashValue2 := uint32(hash[2])<<8 | uint32(hash[3])
	letterIndex2 := int(hashValue2 % 26)

	// 生成标识符（用户AA-ZZ，支持676个用户）
	stableID := fmt.Sprintf("用户%c%c", 'A'+letterIndex1, 'A'+letterIndex2)
	return stableID
}
