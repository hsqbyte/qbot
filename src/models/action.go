package model

// --- OneBot v11 动作模型 ---

// Action 发送给 NapCat 的动作请求
type Action struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
	Echo   string      `json:"echo,omitempty"`
}

// --- 动作参数 ---

type SendGroupMsgParams struct {
	GroupID    int64  `json:"group_id"`
	Message   string `json:"message"`
	AutoEscape bool  `json:"auto_escape"`
}

type SendPrivateMsgParams struct {
	UserID     int64  `json:"user_id"`
	Message    string `json:"message"`
	AutoEscape bool   `json:"auto_escape"`
}

type DeleteMsgParams struct {
	MessageID int64 `json:"message_id"`
}

type SetGroupKickParams struct {
	GroupID          int64 `json:"group_id"`
	UserID           int64 `json:"user_id"`
	RejectAddRequest bool  `json:"reject_add_request"`
}

type SetGroupBanParams struct {
	GroupID  int64 `json:"group_id"`
	UserID  int64 `json:"user_id"`
	Duration int  `json:"duration"` // 秒，0=解除
}

// --- 快捷构造方法 ---

func NewSendGroupMsg(groupID int64, message string) Action {
	return Action{
		Action: "send_group_msg",
		Params: SendGroupMsgParams{GroupID: groupID, Message: message},
	}
}

func NewSendPrivateMsg(userID int64, message string) Action {
	return Action{
		Action: "send_private_msg",
		Params: SendPrivateMsgParams{UserID: userID, Message: message},
	}
}

func NewDeleteMsg(messageID int64) Action {
	return Action{
		Action: "delete_msg",
		Params: DeleteMsgParams{MessageID: messageID},
	}
}

func NewSetGroupKick(groupID, userID int64) Action {
	return Action{
		Action: "set_group_kick",
		Params: SetGroupKickParams{GroupID: groupID, UserID: userID},
	}
}

func NewSetGroupBan(groupID, userID int64, duration int) Action {
	return Action{
		Action: "set_group_ban",
		Params: SetGroupBanParams{GroupID: groupID, UserID: userID, Duration: duration},
	}
}
