package registry

import "github.com/hsqbyte/qbot/src/core/bot"

// handlers 存储所有注册的处理器
var handlers []bot.Handler

// RegisterHandler 供 handler 包的 init() 调用
func RegisterHandler(h bot.Handler) {
	handlers = append(handlers, h)
}

// GetHandlers 返回所有已注册的处理器
func GetHandlers() []bot.Handler {
	return handlers
}
