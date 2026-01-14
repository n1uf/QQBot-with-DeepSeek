package common

// @类型常量
const (
	AtNone   = iota // 未@任何人
	AtBot           // @了机器人
	AtMaster        // @了主人
	AtOthers        // @了其他人
)

// QQEvent 表示一个QQ消息事件
type QQEvent struct {
	MsgType    string
	UserID     int64
	GroupID    int64
	Content    string // 解析后的消息（@用户名 文本内容）
	RawContent string // 原始 array 的 JSON（用于调试）
	AtType     int    // @类型（AtNone/AtBot/AtMaster/AtOthers）
}
