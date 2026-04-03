package src

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/hsqbyte/qbot/src/core/bot"
	_ "github.com/hsqbyte/qbot/src/handler" // init() 自动注册
	"github.com/hsqbyte/qbot/src/registry"
	"github.com/hsqbyte/qbot/src/services"
)

// Run 启动 Bot 服务
func Run() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	// 1. 加载配置
	cfg, err := core.LoadConfig(env)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}
	core.Cfg = cfg

	// 2. 初始化日志
	core.InitLogger(cfg.Log.Level)
	core.Log.Infof("🚀 QQ Bot 启动中... [env=%s]", env)

	// 2.5 初始化 AI
	services.InitAI()
	services.InitPersonas()
	services.InitHistory()
	historyStop := make(chan struct{})
	services.StartHistoryCleaner(historyStop)

	// 3. 创建并启动 Bot
	b := bot.New(cfg)

	// 注册所有 handler（通过 registry 包收集）
	for _, h := range registry.GetHandlers() {
		b.RegisterHandler(h)
	}

	services.GlobalSendAction = b.SendAction
	go b.Run()

	// 4. 启动定时任务
	scheduler := services.NewScheduler(b.SendAction)

	// 注册定时任务（按需为你的群添加）
	// 取配置中的群列表，为每个群注册定时任务
	for _, groupID := range cfg.Bot.Groups {
		scheduler.AddTask(services.MorningGreeting(groupID))
		scheduler.AddTask(services.NightReminder(groupID))
	}

	scheduler.Start()
	core.Log.Info("⏰ 定时任务已启动")

	// 5. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	core.Log.Info("🛑 正在关闭 Bot...")
	close(historyStop) // 停止后台历史清理协程（内部会最后保存一次）
	scheduler.Stop()
	b.Stop()
	core.Log.Info("👋 Bot 已停止")
}
