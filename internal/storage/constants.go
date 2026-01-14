package storage

// 共享常量
const (
	MaxHistoryMessages      = 50     // 最多保留的历史消息数量（私聊和群聊对话历史）
	MaxGroupContextMessages = 50     // 群聊上下文最多保留的消息数量
	MaxMessageLength        = 500    // 单条消息最大字符数，超过此长度的消息不加入上下文
	HistoryDataDir          = "data" // 历史数据存储目录
)
