package handler

import (
	"fmt"
	"strings"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/hsqbyte/qbot/src/core/bot"
	model "github.com/hsqbyte/qbot/src/models"
	"github.com/hsqbyte/qbot/src/registry"
	"github.com/hsqbyte/qbot/src/services"
)

func init() {
	registry.RegisterHandler(&PrivateMsgHandler{})
}

// PrivateMsgHandler 私聊消息处理器
type PrivateMsgHandler struct{}

var _ bot.Handler = (*PrivateMsgHandler)(nil)

func (h *PrivateMsgHandler) Name() string { return "PrivateMsg" }

func (h *PrivateMsgHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil {
		return false
	}
	// 仅匹配私聊中的 / 命令，非命令消息交给 AI 处理
	return msg.MessageType == "private" && strings.HasPrefix(msg.RawMessage, "/")
}

func (h *PrivateMsgHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	text := strings.TrimSpace(msg.RawMessage)

	core.Log.Infof("[私聊] %s(%d): %s", msg.Sender.Nickname, msg.UserID, text)

	// 命令处理
	if strings.HasPrefix(text, "/") {
		cmd := strings.TrimPrefix(text, "/")
		parts := strings.SplitN(cmd, " ", 2)
		command := parts[0]
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		reply := handlePrivateCommand(command, args, msg)
		if reply != "" {
			return []model.Action{model.NewSendPrivateMsg(msg.UserID, reply)}
		}
	}

	// 关键词回复
	if services.MatchKeyword(text) {
		reply := services.GetKeywordReply(text, msg.Sender.Nickname)
		if reply != "" {
			return []model.Action{model.NewSendPrivateMsg(msg.UserID, reply)}
		}
	}

	return nil
}

func handlePrivateCommand(command, args string, msg *model.MessageEvent) string {
	switch command {
	case "ping":
		return "🏓 pong!"
	case "help":
		return `📖 私聊可用命令:
/ping    - 健康检查
/help    - 显示帮助
/status  - 运行状态
/id      - 查看你的QQ号
/echo    - 复读 (例: /echo 你好)`
	case "status":
		return services.HandleGroupCommand("status", "", msg)
	case "id":
		return fmt.Sprintf("你的 QQ 号: %d", msg.UserID)
	case "echo":
		if args == "" {
			return "用法: /echo <内容>"
		}
		return args
	default:
		return ""
	}
}
