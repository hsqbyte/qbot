package model

import "encoding/json"

// --- OneBot v11 事件模型 ---

// Event 上报事件基础结构
type Event struct {
	Time     int64           `json:"time"`
	SelfID   int64           `json:"self_id"`
	PostType string          `json:"post_type"` // message, notice, request, meta_event
	RawData  json.RawMessage `json:"-"`
}

// MessageEvent 消息事件
type MessageEvent struct {
	Event
	MessageType string          `json:"message_type"` // group, private
	SubType     string          `json:"sub_type"`
	MessageID   int64           `json:"message_id"`
	UserID      int64           `json:"user_id"`
	GroupID     int64           `json:"group_id"`
	RawMessage  string          `json:"raw_message"`
	Message     json.RawMessage `json:"message"` // 可能是 string 或 array
	Font        int             `json:"font"`
	Sender      Sender          `json:"sender"`
}

// Sender 发送者信息
type Sender struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card"`
	Sex      string `json:"sex"`
	Age      int    `json:"age"`
	Role     string `json:"role"` // owner, admin, member
}

// NoticeEvent 通知事件（加群、退群、禁言等）
type NoticeEvent struct {
	Event
	NoticeType string `json:"notice_type"`
	SubType    string `json:"sub_type"`
	GroupID    int64  `json:"group_id"`
	UserID     int64  `json:"user_id"`
	OperatorID int64  `json:"operator_id"`
}

// RequestEvent 请求事件（加好友、加群）
type RequestEvent struct {
	Event
	RequestType string `json:"request_type"`
	SubType     string `json:"sub_type"`
	UserID      int64  `json:"user_id"`
	GroupID     int64  `json:"group_id"`
	Comment     string `json:"comment"`
	Flag        string `json:"flag"`
}

// MetaEvent 元事件（心跳、生命周期）
type MetaEvent struct {
	Event
	MetaEventType string `json:"meta_event_type"`
	SubType       string `json:"sub_type"`
}

// --- 解析方法 ---

func ParseEvent(data []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	e.RawData = data
	return &e, nil
}

func (e *Event) ToMessageEvent() (*MessageEvent, error) {
	var msg MessageEvent
	err := json.Unmarshal(e.RawData, &msg)
	return &msg, err
}

func (e *Event) ToNoticeEvent() (*NoticeEvent, error) {
	var n NoticeEvent
	err := json.Unmarshal(e.RawData, &n)
	return &n, err
}

func (e *Event) ToRequestEvent() (*RequestEvent, error) {
	var r RequestEvent
	err := json.Unmarshal(e.RawData, &r)
	return &r, err
}
