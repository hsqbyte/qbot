package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/hsqbyte/qbot/src/models"
	"github.com/sashabaranov/go-openai"
)

var (
	aiClient *openai.Client
	// GlobalSendAction 暴露给 services 发送工具执行中间提醒
	GlobalSendAction func(action model.Action) error
)

// InitAI 初始化 AI 客户端
func InitAI() {
	if !core.Cfg.AI.Enable {
		return
	}

	config := openai.DefaultConfig(core.Cfg.AI.APIKey)
	if core.Cfg.AI.BaseURL != "" {
		config.BaseURL = core.Cfg.AI.BaseURL
	}
	aiClient = openai.NewClientWithConfig(config)
	core.Log.Info("🧠 AI 大模型客户端已初始化")

	// 加载外部 JSON 动态技能
	LoadExternalSkills()
}

// ChatRequest AI 对话请求参数
type ChatRequest struct {
	Text           string   // 纯文本内容
	ImageURLs      []string // 图片 data URI 列表（base64）
	History        []openai.ChatCompletionMessage
	SessionKey     string // 会话 key（用于获取人设 prompt）
	NotifyToolCall func(toolName string) (chan<- string, func(error))
}

// ChatWithAI 调用 LLM 完成对话，支持多模态（图片）和 Skill 函数调用
func ChatWithAI(req ChatRequest) string {
	if aiClient == nil {
		return "⚠️ AI 未开启或未配置 API_KEY"
	}

	// 获取当前会话的人设 prompt，追加 QQ 格式约束
	prompt := GetPersonaPrompt(req.SessionKey)
	prompt += `

【格式要求】你在QQ群聊中回复消息，请严格遵守：
1. 禁止使用任何Markdown语法（# ** *** ` + "```" + ` - > [] ()链接 等全部禁止）
2. 用纯文本回复，像正常QQ聊天一样自然对话
3. 多用emoji表情让消息更生动 😊🎉👍
4. 回复要简短精炼，像聊天不像写文章，一般3-5句话就够了
5. 如果内容较多，用换行和emoji分隔，不要用序号列表
6. 代码直接贴出来不要用代码块包裹
7. 语气口语化、亲切，不要太正式`

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: prompt,
		},
	}
	messages = append(messages, req.History...)

	// 为了解决大模型经常“嘴上答应不调工具”的顽疾，在最新一条消息后附带强制指令
	enforceToolPrompt := "\n\n(系统指令：如果用户的请求涉及看代码、执行命令、开发或分析任务，请立即触发工具调用，绝对不要回复诸如“我去看看”、“等我查查”、“马上安排”的确认文本！如果不需要工具则正常聊天。)"

	// 构建用户消息（支持多模态）
	if len(req.ImageURLs) > 0 {
		// 多模态消息：文字 + 图片
		parts := BuildMultimodalContent(req.Text+enforceToolPrompt, req.ImageURLs)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: parts,
		})
	} else {
		// 纯文本消息
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: req.Text + enforceToolPrompt,
		})
	}

	// 注入 tools
	var tools []openai.Tool
	for _, skill := range GetRegisteredSkills() {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        skill.Name,
				Description: skill.Description,
				Parameters:  json.RawMessage(skill.ParametersSchema),
			},
		})
	}

	ctx := context.Background()
	apiReq := openai.ChatCompletionRequest{
		Model:    core.Cfg.AI.Model,
		Messages: messages,
		Tools:    tools,
	}

	// 最多两轮循环（防止无限制调用陷入死循环）
	for i := 0; i < 2; i++ {
		resp, err := aiClient.CreateChatCompletion(ctx, apiReq)
		if err != nil {
			core.Log.Errorf("AI 请求失败: %v", err)
			return fmt.Sprintf("⚠️ 思考失败: %v", err)
		}

		if len(resp.Choices) == 0 {
			return "⚠️ 脑袋空空，不知道怎么回答"
		}

		choice := resp.Choices[0]
		
		// 如果不需要函数调用，直接返回内容
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content
		}

		// 有工具调用
		apiReq.Messages = append(apiReq.Messages, choice.Message)

		for _, toolCall := range choice.Message.ToolCalls {
			if toolCall.Type == openai.ToolTypeFunction {
				core.Log.Infof("🛠️ 触发技能: %s (参数: %s)", toolCall.Function.Name, toolCall.Function.Arguments)
				
				// 如果有通知回调，则通知用户并获取关闭信号
				var done func(error)
				var progressChan chan<- string
				if req.NotifyToolCall != nil {
					progressChan, done = req.NotifyToolCall(toolCall.Function.Name)
				}

				// 执行技能并注入流式回调
				result := ExecuteSkill(toolCall.Function.Name, toolCall.Function.Arguments, func(line string) {
					// 过滤无意义日志，并去掉内部标识符
					line = strings.TrimSpace(line)
					line = strings.TrimPrefix(line, "[PROGRESS] ")
					if len(line) > 5 && progressChan != nil {
						// select 防止通道阻塞
						select {
						case progressChan <- line:
						default:
						}
					}
				})
				
				// 停止定时的提醒并通知执行结果
				if done != nil {
					if strings.HasPrefix(result, "Error:") || strings.HasPrefix(result, "获取输出流失败:") || strings.HasPrefix(result, "启动失败:") || strings.HasPrefix(result, "执行失败:") {
						done(fmt.Errorf("%s", result))
					} else {
						done(nil)
					}
				}
				
				// 把结果追加到消息列表
				apiReq.Messages = append(apiReq.Messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}

	// 最终请求一次获取文本
	resp, err := aiClient.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		core.Log.Errorf("AI 函数调用后求值失败: %v", err)
		return "⚠️ 处理技能反馈时出错"
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content
	}
	return "⚠️ 获取最终回答失败"
}

// ChatWithLLM 旧接口兼容（纯文本）
func ChatWithLLM(message string, history []openai.ChatCompletionMessage) string {
	return ChatWithAI(ChatRequest{
		Text:    message,
		History: history,
	})
}
