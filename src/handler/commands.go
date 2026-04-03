package handler

import (
	"strings"

	"github.com/hsqbyte/qbot/src/core/bot"
	model "github.com/hsqbyte/qbot/src/models"
	"github.com/hsqbyte/qbot/src/registry"
	"github.com/hsqbyte/qbot/src/services"
)

// init() 自动注册
func init() {
	registry.RegisterHandler(&GroupCommandHandler{})
	// GroupKeywordHandler 已禁用，所有消息统一走 AI 回复
	// registry.RegisterHandler(&GroupKeywordHandler{})
	registry.RegisterHandler(&GroupWelcomeHandler{})
}

// ===== GroupCommandHandler: 群命令处理（/开头） =====

type GroupCommandHandler struct{}

var _ bot.Handler = (*GroupCommandHandler)(nil)

func (h *GroupCommandHandler) Name() string { return "GroupCommand" }

func (h *GroupCommandHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil || msg.MessageType != "group" {
		return false
	}
	return strings.HasPrefix(msg.RawMessage, "/")
}

func (h *GroupCommandHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	cmd := strings.TrimPrefix(msg.RawMessage, "/")
	parts := strings.SplitN(cmd, " ", 2)
	command := strings.TrimSpace(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	reply := services.HandleGroupCommand(command, args, msg)
	if reply == "" {
		return nil
	}

	return []model.Action{model.NewSendGroupMsg(msg.GroupID, reply)}
}

// ===== GroupKeywordHandler: 关键词自动回复 =====

type GroupKeywordHandler struct{}

var _ bot.Handler = (*GroupKeywordHandler)(nil)

func (h *GroupKeywordHandler) Name() string { return "GroupKeyword" }

func (h *GroupKeywordHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil || msg.MessageType != "group" {
		return false
	}
	// 不处理命令消息（交给 CommandHandler）
	if strings.HasPrefix(msg.RawMessage, "/") {
		return false
	}
	return services.MatchKeyword(msg.RawMessage)
}

func (h *GroupKeywordHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	reply := services.GetKeywordReply(msg.RawMessage, msg.Sender.Nickname)
	if reply == "" {
		return nil
	}
	return []model.Action{model.NewSendGroupMsg(msg.GroupID, reply)}
}

// ===== GroupWelcomeHandler: 新人入群欢迎 =====

type GroupWelcomeHandler struct{}

var _ bot.Handler = (*GroupWelcomeHandler)(nil)

func (h *GroupWelcomeHandler) Name() string { return "GroupWelcome" }

func (h *GroupWelcomeHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	// 这个处理的是 notice 事件，不是 message 事件
	return event.PostType == "notice"
}

func (h *GroupWelcomeHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	notice, err := event.ToNoticeEvent()
	if err != nil {
		return nil
	}

	// 新人入群
	if notice.NoticeType == "group_increase" {
		reply := services.WelcomeNewMember(notice.GroupID, notice.UserID)
		if reply != "" {
			return []model.Action{model.NewSendGroupMsg(notice.GroupID, reply)}
		}
	}

	return nil
}
