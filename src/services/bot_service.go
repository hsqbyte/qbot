package services

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	model "github.com/hsqbyte/qbot/src/models"
)

var startTime = time.Now()

// HandleGroupCommand 处理群内 /命令
func HandleGroupCommand(command, args string, msg *model.MessageEvent) string {
	switch command {
	case "ping":
		return "🏓 pong!"

	case "help":
		return `📖 可用命令:
— 基础 —
/ping     - 健康检查
/help     - 显示帮助
/status   - 运行状态
/echo     - 复读 (例: /echo 你好)
/at       - @发送者并回复

— 管理（需管理员权限）—
/kick @xxx       - 踢出成员
/ban @xxx [秒]   - 禁言 (默认600秒)
/unban @xxx      - 解除禁言
/mute            - 全员禁言
/unmute          - 解除全员禁言`

	case "status":
		return getStatus()

	case "echo":
		if args == "" {
			return "用法: /echo <内容>"
		}
		return args

	case "at":
		// CQ码 @某人
		return fmt.Sprintf("[CQ:at,qq=%d] 你好呀~", msg.UserID)

	default:
		return ""
	}
}

// --- 关键词自动回复 ---

// KeywordRule 关键词规则
type KeywordRule struct {
	Keywords []string           // 触发关键词（包含任一即触发）
	Reply    func(nickname string) string // 回复内容（支持动态）
}

// keywordRules 关键词规则表 - 在此添加你的自定义规则
var keywordRules = []KeywordRule{
	{
		Keywords: []string{"你好", "hello", "hi"},
		Reply:    func(nick string) string { return fmt.Sprintf("你好呀 %s~ 👋", nick) },
	},
	{
		Keywords: []string{"早上好", "早安", "good morning"},
		Reply:    func(_ string) string { return "早上好！今天也要元气满满哦 ☀️" },
	},
	{
		Keywords: []string{"晚安", "good night"},
		Reply:    func(nick string) string { return fmt.Sprintf("晚安 %s，做个好梦 🌙", nick) },
	},
	{
		Keywords: []string{"谢谢", "感谢", "thanks"},
		Reply:    func(_ string) string { return "不客气~ 😊" },
	},
	{
		Keywords: []string{"机器人", "bot"},
		Reply:    func(_ string) string { return "我在呢！有什么需要帮忙的吗？ 🤖" },
	},
}

// MatchKeyword 检查消息是否命中任何关键词
func MatchKeyword(message string) bool {
	msg := strings.ToLower(message)
	for _, rule := range keywordRules {
		for _, kw := range rule.Keywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				return true
			}
		}
	}
	return false
}

// GetKeywordReply 获取关键词回复内容
func GetKeywordReply(message, nickname string) string {
	msg := strings.ToLower(message)
	for _, rule := range keywordRules {
		for _, kw := range rule.Keywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				return rule.Reply(nickname)
			}
		}
	}
	return ""
}

// --- 新人欢迎 ---

// WelcomeNewMember 生成新人欢迎消息
func WelcomeNewMember(groupID, userID int64) string {
	return fmt.Sprintf("[CQ:at,qq=%d] 欢迎加入群聊！🎉\n请先看看群公告了解群规哦~", userID)
}

// --- 内部工具 ---

func getStatus() string {
	uptime := time.Since(startTime).Round(time.Second)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return fmt.Sprintf(`📊 Bot 状态
• 运行时间: %s
• Go 版本: %s
• 协程数: %d
• 内存使用: %.1f MB`,
		uptime,
		runtime.Version(),
		runtime.NumGoroutine(),
		float64(m.Alloc)/1024/1024,
	)
}
