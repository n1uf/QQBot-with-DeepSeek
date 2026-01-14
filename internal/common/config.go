package common

import (
	"log"
	"os"
	"strconv"
)

// 配置常量
const (
	ListenPort             = ":8080"
	DeepSeekBaseURL        = "https://api.deepseek.com/chat/completions"
	RepeatMessageQueueSize = 3 // 连续相同消息检测队列大小
)

// 配置变量
var (
	DeepSeekAPIKey           string
	BotQQNumber              int64
	MasterQQNumber           int64
	MasterGirlFriendQQNumber int64
)

func init() {
	DeepSeekAPIKey = os.Getenv("DEEPSEEK_API_KEY")

	// 尝试读取并转换，如果失败则给个提醒
	botQQStr := os.Getenv("BOT_QQ")
	masterQQStr := os.Getenv("MASTER_QQ")
	masterGirlFriendQQStr := os.Getenv("MASTER_GIRL_FRIEND_QQ")

	if botQQStr == "" || masterQQStr == "" || masterGirlFriendQQStr == "" {
		log.Println("⚠️  警告: BOT_QQ 或 MASTER_QQ 或 MASTER_GIRL_FRIEND_QQ 未设置，机器人可能无法识别艾特或主人身份")
	}

	BotQQNumber, _ = strconv.ParseInt(botQQStr, 10, 64)
	MasterQQNumber, _ = strconv.ParseInt(masterQQStr, 10, 64)
	MasterGirlFriendQQNumber, _ = strconv.ParseInt(masterGirlFriendQQStr, 10, 64)
}
