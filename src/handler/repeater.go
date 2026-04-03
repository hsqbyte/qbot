package handler

import (
	"sync"

	"github.com/hsqbyte/qbot/src/core/bot"
	model "github.com/hsqbyte/qbot/src/models"
)

func init() {
	// RepeaterHandler 已禁用，所有消息统一走 AI 回复
	// registry.RegisterHandler(&RepeaterHandler{})
}

// RepeaterHandler 复读机
// 当群内连续 N 人发送相同消息时，机器人也复读一次
type RepeaterHandler struct {
	mu    sync.Mutex
	cache map[int64]*repeatState // groupID -> state
}

type repeatState struct {
	LastMsg string  // 上一条消息内容
	Count   int     // 连续相同消息次数
	Users   []int64 // 发送过的用户（防止同一人刷）
	Sent    bool    // 本轮是否已复读
}

const repeatThreshold = 3 // 连续3人相同消息触发

var _ bot.Handler = (*RepeaterHandler)(nil)

func (h *RepeaterHandler) Name() string { return "Repeater" }

func (h *RepeaterHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil || msg.MessageType != "group" {
		return false
	}
	// 跳过命令消息
	if len(msg.RawMessage) > 0 && msg.RawMessage[0] == '/' {
		return false
	}
	// 跳过太长的消息
	if len(msg.RawMessage) > 100 {
		return false
	}
	return h.shouldRepeat(msg)
}

func (h *RepeaterHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	return []model.Action{model.NewSendGroupMsg(msg.GroupID, msg.RawMessage)}
}

func (h *RepeaterHandler) shouldRepeat(msg *model.MessageEvent) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cache == nil {
		h.cache = make(map[int64]*repeatState)
	}

	state, ok := h.cache[msg.GroupID]
	if !ok {
		h.cache[msg.GroupID] = &repeatState{
			LastMsg: msg.RawMessage,
			Count:   1,
			Users:   []int64{msg.UserID},
		}
		return false
	}

	// 消息不同，重置
	if state.LastMsg != msg.RawMessage {
		h.cache[msg.GroupID] = &repeatState{
			LastMsg: msg.RawMessage,
			Count:   1,
			Users:   []int64{msg.UserID},
		}
		return false
	}

	// 同一个人连续发不算
	for _, uid := range state.Users {
		if uid == msg.UserID {
			return false
		}
	}

	state.Users = append(state.Users, msg.UserID)
	state.Count++

	// 达到阈值且本轮未复读
	if state.Count >= repeatThreshold && !state.Sent {
		state.Sent = true
		return true
	}

	return false
}
