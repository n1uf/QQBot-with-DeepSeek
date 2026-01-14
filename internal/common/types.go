package common

// QQEvent 表示一个QQ消息事件
type QQEvent struct {
	MsgType    string
	UserID     int64
	GroupID    int64
	Content    string // 过滤过CQ码消息后的内容
	RawContent string // 原始消息
}
