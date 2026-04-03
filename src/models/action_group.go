package model

// SetGroupWholeBanParams 全员禁言
type SetGroupWholeBanParams struct {
	GroupID int64 `json:"group_id"`
	Enable  bool  `json:"enable"`
}

// SetGroupCardParams 设置群名片
type SetGroupCardParams struct {
	GroupID int64  `json:"group_id"`
	UserID  int64  `json:"user_id"`
	Card    string `json:"card"`
}

// GetGroupMemberInfoParams 获取群成员信息
type GetGroupMemberInfoParams struct {
	GroupID int64 `json:"group_id"`
	UserID  int64 `json:"user_id"`
	NoCache bool  `json:"no_cache"`
}

// --- 快捷构造 ---

func NewSetGroupWholeBan(groupID int64, enable bool) Action {
	return Action{
		Action: "set_group_whole_ban",
		Params: SetGroupWholeBanParams{GroupID: groupID, Enable: enable},
	}
}

func NewSetGroupCard(groupID, userID int64, card string) Action {
	return Action{
		Action: "set_group_card",
		Params: SetGroupCardParams{GroupID: groupID, UserID: userID, Card: card},
	}
}
