package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/hsqbyte/qbot/src/core/bot"
	model "github.com/hsqbyte/qbot/src/models"
	"github.com/hsqbyte/qbot/src/registry"
	"github.com/hsqbyte/qbot/src/services"
)

// skillDisplayConfig 技能显示配置：友好名称 + 预估耗时
var skillDisplayConfig = map[string]struct {
	DisplayName      string
	EstimatedSeconds int
}{
	"claude-code":      {"🧠 Claude Code 智能助手", 120},
	"restart_bot":      {"🔄 Bot 重启", 10},
	"weather":          {"🌤️ 天气查询", 3},
	"get_current_time": {"🕐 时间查询", 1},
	"get_weather":      {"🌤️ 天气查询", 3},
}

// progressBarFrames 进度条动画帧（5 格方块动画）
var progressBarFrames = []string{
	"▰▱▱▱▱",
	"▰▰▱▱▱",
	"▰▰▰▱▱",
	"▰▰▰▰▱",
	"▰▰▰▰▰",
	"▰▰▰▰▱",
	"▰▰▰▱▱",
	"▰▰▱▱▱",
	"▰▱▱▱▱",
}

// buildSkillProgressBar 根据已用时间和预估时间生成进度条
func buildSkillProgressBar(elapsed, estimated int) string {
	if estimated <= 0 {
		// 无预估时间时，使用动画帧
		return progressBarFrames[elapsed%len(progressBarFrames)]
	}
	// 根据时间比例计算进度，最多到 90%（完成时才 100%）
	pct := elapsed * 100 / estimated
	if pct > 90 {
		pct = 90
	}
	if pct < 5 {
		pct = 5
	}
	filled := pct / 10
	bar := strings.Repeat("▰", filled) + strings.Repeat("▱", 10-filled)
	return fmt.Sprintf("%s %d%%", bar, pct)
}

// 触发 AI 的关键词列表（纯文本 @ 的备选匹配）
var aiTriggers = []string{"机器人", "bot", "助手"}

func isAtBot(event *model.Event, msg *model.MessageEvent) bool {
	raw := msg.RawMessage

	// 方式1: CQ码 @ [CQ:at,qq=<self_id>]
	atCQ := fmt.Sprintf("[CQ:at,qq=%d]", event.SelfID)
	if strings.Contains(raw, atCQ) {
		return true
	}

	// 方式2: 纯文本 @昵称
	atNick := "@" + core.Cfg.Bot.Nickname
	if strings.Contains(raw, atNick) {
		return true
	}

	// 方式3: 包含任意 @xxx 且 xxx 是触发关键词
	if strings.Contains(raw, "@") {
		lower := strings.ToLower(raw)
		for _, kw := range aiTriggers {
			if strings.Contains(lower, "@"+kw) {
				return true
			}
		}
	}

	return false
}

func init() {
	registry.RegisterHandler(&AIChatHandler{})
}

// AIChatHandler 监听消息交给 LLM 处理
// 支持: 多模态图片理解、人设切换、引用回复
type AIChatHandler struct{}

var _ bot.Handler = (*AIChatHandler)(nil)

func (h *AIChatHandler) Name() string { return "AIChat" }

func (h *AIChatHandler) Match(event *model.Event, msg *model.MessageEvent) bool {
	if msg == nil {
		return false
	}
	if !core.Cfg.AI.Enable {
		return false
	}

	raw := msg.RawMessage

	// 人设命令始终匹配（/人设 /prompt）
	if strings.HasPrefix(raw, "/人设") || strings.HasPrefix(raw, "/prompt") {
		return true
	}

	// 其他 / 命令交给 command handler
	if strings.HasPrefix(raw, "/") {
		return false
	}

	// 私聊消息全部给 AI
	if msg.MessageType == "private" {
		return true
	}

	// 群消息：根据 group_mode 配置决定
	mode := core.Cfg.AI.GroupMode
	switch mode {
	case "at_only":
		return isAtBot(event, msg)
	default: // "always" 或未配置
		return msg.MessageType == "group"
	}
}

func (h *AIChatHandler) Handle(event *model.Event, msg *model.MessageEvent) []model.Action {
	raw := msg.RawMessage

	// 生成会话 key（群聊按群隔离，私聊按用户隔离）
	key := services.SessionKey(msg.MessageType, msg.UserID, msg.GroupID)

	// === 处理人设命令 ===
	if strings.HasPrefix(raw, "/人设") {
		args := strings.TrimSpace(strings.TrimPrefix(raw, "/人设"))
		reply := services.HandlePersonaCommand(key, args)
		return h.reply(msg, reply)
	}
	if strings.HasPrefix(raw, "/prompt") {
		customPrompt := strings.TrimSpace(strings.TrimPrefix(raw, "/prompt"))
		if customPrompt == "" {
			return h.reply(msg, "❌ 请提供 prompt 内容\n用法：/prompt 你是一只可爱的猫")
		}
		services.SetCustomPersona(key, customPrompt)
		reply := fmt.Sprintf("✅ 已设置自定义 prompt！记忆已清除。\n📝 %s", customPrompt)
		return h.reply(msg, reply)
	}

	// === 清理文本 & 提取图片 ===
	cleanText := raw

	// 去掉 CQ码 @
	atCQ := fmt.Sprintf("[CQ:at,qq=%d]", event.SelfID)
	cleanText = strings.ReplaceAll(cleanText, atCQ, "")

	// 去掉 @昵称、@机器人、@bot 等
	cleanText = strings.ReplaceAll(cleanText, "@"+core.Cfg.Bot.Nickname, "")
	for _, kw := range aiTriggers {
		cleanText = strings.ReplaceAll(cleanText, "@"+kw, "")
		cleanText = strings.ReplaceAll(cleanText, "@"+strings.ToUpper(kw[:1])+kw[1:], "")
	}

	// 提取图片 URL
	images := services.ExtractImages(cleanText)

	// 去掉图片 CQ 码，只保留纯文本
	cleanText = services.StripImageCQ(cleanText)
	cleanText = strings.TrimSpace(cleanText)

	// /clear 清除对话记忆
	if cleanText == "/clear" {
		services.ClearHistory(key)
		core.Log.Infof("[AIChat] 清除会话记忆: %s", key)
		return h.reply(msg, "对话记忆已清除~ 🧹")
	}

	// 如果只有图片没有文字，让 AI 自然地看图聊天
	if cleanText == "" && len(images) > 0 {
		cleanText = "看看这张图"
	} else if cleanText == "" {
		cleanText = "你好"
	}

	core.Log.Infof("[AIChat] 处理 AI 请求: %s (图片: %d 张)", cleanText, len(images))

	// === 下载图片转 base64 ===
	var imageDataURIs []string
	for _, img := range images {
		dataURI, err := services.DownloadImageAsBase64(img.URL)
		if err != nil {
			core.Log.Warnf("[AIChat] 图片下载失败: %v", err)
			continue
		}
		imageDataURIs = append(imageDataURIs, dataURI)
	}

	// === 调用 AI ===
	history := services.GetHistory(key)

	reply := services.ChatWithAI(services.ChatRequest{
		Text:       cleanText,
		ImageURLs:  imageDataURIs,
		History:    history,
		SessionKey: key,
		NotifyToolCall: func(toolName string) (chan<- string, func(error)) {
			if services.GlobalSendAction == nil {
				return nil, func(error) {}
			}

			// 获取技能显示配置
			displayName := toolName
			estimatedSeconds := 0
			if cfg, ok := skillDisplayConfig[toolName]; ok {
				displayName = cfg.DisplayName
				estimatedSeconds = cfg.EstimatedSeconds
			}
			startTime := time.Now()

			sendMsg := func(text string) {
				var action model.Action
				if msg.MessageType == "group" {
					action = model.NewSendGroupMsg(msg.GroupID, fmt.Sprintf("[CQ:reply,id=%d][CQ:at,qq=%d] %s", msg.MessageID, msg.Sender.UserID, text))
				} else {
					action = model.NewSendPrivateMsg(msg.Sender.UserID, text)
				}
				services.GlobalSendAction(action)
			}

			// 发送「开始执行」提示
			startText := fmt.Sprintf("🚀 正在调用 %s", displayName)
			if estimatedSeconds > 5 {
				startText += fmt.Sprintf("\n⏱️ 预计耗时约 %d 秒", estimatedSeconds)
			}
			startText += "\n▱▱▱▱▱ 开始执行..."
			sendMsg(startText)

			// 启动定时器，每 30 秒提醒一次
			progressChan := make(chan string, 100)
			stop := make(chan struct{})

			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				tickCount := 0
				var latestProgress string
				for {
					select {
					case p := <-progressChan:
						latestProgress = p
					case <-ticker.C:
						tickCount++
						elapsed := int(time.Since(startTime).Seconds())
						bar := buildSkillProgressBar(elapsed, estimatedSeconds)

						txt := fmt.Sprintf("⏳ %s 执行中...\n%s 已用时 %ds", displayName, bar, elapsed)
						if latestProgress != "" {
							txt += "\n\n📋 " + latestProgress
						}
						sendMsg(txt)
					case <-stop:
						return
					}
				}
			}()

			return progressChan, func(err error) {
				close(stop)
				elapsed := time.Since(startTime)
				if err != nil {
					sendMsg(fmt.Sprintf("❌ %s 执行失败 (%.1fs)\n💡 建议: 请稍后重试或换个方式描述需求", displayName, elapsed.Seconds()))
				} else {
					sendMsg(fmt.Sprintf("✅ %s 执行完成 (%.1fs)，正在整理回复...", displayName, elapsed.Seconds()))
				}
			}
		},
	})

	// 保存对话记忆（群聊带昵称）
	nickname := ""
	if msg.MessageType == "group" {
		nickname = msg.Sender.Nickname
	}
	// 记忆中记录为纯文本（图片用描述标记）
	historyText := cleanText
	if len(images) > 0 {
		historyText = fmt.Sprintf("%s [附带%d张图片]", cleanText, len(images))
	}
	services.AppendHistory(key, historyText, reply, nickname)

	// === 构建回复（带引用） ===
	return h.replyWithQuote(msg, reply)
}

// reply 构建普通回复（群聊 @发送者，私聊直接回复）
func (h *AIChatHandler) reply(msg *model.MessageEvent, text string) []model.Action {
	if msg.MessageType == "group" {
		finalReply := fmt.Sprintf("[CQ:at,qq=%d] %s", msg.UserID, text)
		return []model.Action{model.NewSendGroupMsg(msg.GroupID, finalReply)}
	}
	return []model.Action{model.NewSendPrivateMsg(msg.UserID, text)}
}

// replyWithQuote 构建引用回复（群消息引用原消息）
func (h *AIChatHandler) replyWithQuote(msg *model.MessageEvent, text string) []model.Action {
	if msg.MessageType == "group" {
		// [CQ:reply,id=xxx] 引用原消息 + [CQ:at,qq=xxx] @发送者
		finalReply := fmt.Sprintf("[CQ:reply,id=%d][CQ:at,qq=%d] %s", msg.MessageID, msg.UserID, text)
		return []model.Action{model.NewSendGroupMsg(msg.GroupID, finalReply)}
	}
	return []model.Action{model.NewSendPrivateMsg(msg.UserID, text)}
}
