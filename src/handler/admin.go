package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/hsqbyte/qbot/src/core/bot"
	model "github.com/hsqbyte/qbot/src/models"
	"github.com/hsqbyte/qbot/src/registry"
)

func init() {
	registry.RegisterHandler(&GroupAdminHandler{})
}

// GroupAdminHandler 群管理命令处理器
// 命令: /kick @xxx, /ban @xxx 60, /mute, /unmute
type GroupAdminHandler struct{}

var _ bot.Handler = (*GroupAdminHandler)(nil)

func (h *GroupAdminHandler) Name() string { return "GroupAdmin" }

func (h *GroupAdminHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil || msg.MessageType != "group" {
		return false
	}
	cmd := strings.TrimPrefix(msg.RawMessage, "/")
	for _, prefix := range []string{"kick", "ban", "unban", "mute", "unmute"} {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

func (h *GroupAdminHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	// 检查是否为管理员
	if !isAdmin(msg) {
		return []model.Action{
			model.NewSendGroupMsg(msg.GroupID, "⛔ 权限不足，仅管理员可操作"),
		}
	}

	raw := strings.TrimPrefix(msg.RawMessage, "/")
	parts := strings.Fields(raw)
	command := parts[0]

	switch command {
	case "kick":
		return handleKick(msg, parts)
	case "ban":
		return handleBan(msg, parts)
	case "unban":
		return handleUnban(msg, parts)
	case "mute":
		return handleMuteAll(msg, true)
	case "unmute":
		return handleMuteAll(msg, false)
	}

	return nil
}

// handleKick 踢人: /kick @xxx
func handleKick(msg *model.MessageEvent, parts []string) []model.Action {
	targetID := extractCQAt(msg.RawMessage)
	if targetID == 0 {
		return []model.Action{
			model.NewSendGroupMsg(msg.GroupID, "用法: /kick @某人"),
		}
	}

	core.Log.Infof("[群%d] 管理员 %s 踢出 %d", msg.GroupID, msg.Sender.Nickname, targetID)

	return []model.Action{
		model.NewSetGroupKick(msg.GroupID, targetID),
		model.NewSendGroupMsg(msg.GroupID, fmt.Sprintf("✅ 已踢出 [CQ:at,qq=%d]", targetID)),
	}
}

// handleBan 禁言: /ban @xxx 60 (秒，默认600秒)
func handleBan(msg *model.MessageEvent, parts []string) []model.Action {
	targetID := extractCQAt(msg.RawMessage)
	if targetID == 0 {
		return []model.Action{
			model.NewSendGroupMsg(msg.GroupID, "用法: /ban @某人 [秒数]"),
		}
	}

	duration := 600 // 默认10分钟
	// 尝试从参数中提取时长
	for _, p := range parts[1:] {
		if d, err := strconv.Atoi(p); err == nil && d > 0 {
			duration = d
			break
		}
	}

	core.Log.Infof("[群%d] 管理员 %s 禁言 %d %d秒", msg.GroupID, msg.Sender.Nickname, targetID, duration)

	return []model.Action{
		model.NewSetGroupBan(msg.GroupID, targetID, duration),
		model.NewSendGroupMsg(msg.GroupID, fmt.Sprintf("🔇 已禁言 [CQ:at,qq=%d] %d秒", targetID, duration)),
	}
}

// handleUnban 解除禁言: /unban @xxx
func handleUnban(msg *model.MessageEvent, parts []string) []model.Action {
	targetID := extractCQAt(msg.RawMessage)
	if targetID == 0 {
		return []model.Action{
			model.NewSendGroupMsg(msg.GroupID, "用法: /unban @某人"),
		}
	}

	return []model.Action{
		model.NewSetGroupBan(msg.GroupID, targetID, 0), // duration=0 解除禁言
		model.NewSendGroupMsg(msg.GroupID, fmt.Sprintf("🔊 已解除 [CQ:at,qq=%d] 的禁言", targetID)),
	}
}

// handleMuteAll 全员禁言/解除
func handleMuteAll(msg *model.MessageEvent, enable bool) []model.Action {
	action := model.Action{
		Action: "set_group_whole_ban",
		Params: model.SetGroupWholeBanParams{
			GroupID: msg.GroupID,
			Enable:  enable,
		},
	}

	var tip string
	if enable {
		tip = "🔇 已开启全员禁言"
	} else {
		tip = "🔊 已关闭全员禁言"
	}

	return []model.Action{
		action,
		model.NewSendGroupMsg(msg.GroupID, tip),
	}
}

// --- 工具方法 ---

// isAdmin 检查发送者是否有管理权限
// 优先检查配置中的 admins 列表，再检查群角色
func isAdmin(msg *model.MessageEvent) bool {
	// 检查配置中的超级管理员
	if core.Cfg != nil {
		for _, id := range core.Cfg.Bot.Admins {
			if msg.UserID == id {
				return true
			}
		}
	}
	// 检查群角色 (owner / admin)
	return msg.Sender.Role == "owner" || msg.Sender.Role == "admin"
}

// extractCQAt 从 CQ 码消息中提取 @的QQ号
// 例: [CQ:at,qq=123456] -> 123456
func extractCQAt(message string) int64 {
	const prefix = "[CQ:at,qq="
	idx := strings.Index(message, prefix)
	if idx == -1 {
		return 0
	}
	start := idx + len(prefix)
	end := strings.Index(message[start:], "]")
	if end == -1 {
		return 0
	}
	id, err := strconv.ParseInt(message[start:start+end], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
