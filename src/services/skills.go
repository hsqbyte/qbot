package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hsqbyte/qbot/src/core"
)

// Skill 定义单个功能技能
type Skill struct {
	Name             string
	Description      string
	ParametersSchema string // JSON Schema
	Execute          func(args string, onProgress func(string)) string
}

var skillRegistry = make(map[string]Skill)

// RegisterSkill 注册一个新技能
func RegisterSkill(s Skill) {
	skillRegistry[s.Name] = s
}

// GetRegisteredSkills 获取所有注册的技能
func GetRegisteredSkills() []Skill {
	var list []Skill
	for _, s := range skillRegistry {
		list = append(list, s)
	}
	return list
}

// ExecuteSkill 执行指定名称的技能
func ExecuteSkill(name, args string, onProgress func(string)) string {
	skill, ok := skillRegistry[name]
	if !ok {
		return fmt.Sprintf("Error: Skill '%s' not found", name)
	}
	return skill.Execute(args, onProgress)
}

// --- 预置技能库 ---

func init() {
	// 注册获取当前时间的技能
	RegisterSkill(Skill{
		Name:        "get_current_time",
		Description: "获取当前的日期和时间。如果用户询问当前时间，可以调用此技能。",
		ParametersSchema: `{
			"type": "object",
			"properties": {},
			"required": []
		}`,
		Execute: func(args string, onProgress func(string)) string {
			now := time.Now().Format("2006-01-02 15:04:05")
			return fmt.Sprintf("当前时间是: %s", now)
		},
	})
	
	// 注册一个模拟天气查询的技能
	RegisterSkill(Skill{
		Name:        "get_weather",
		Description: "获取指定城市的天气情况。",
		ParametersSchema: `{
			"type": "object",
			"properties": {
				"city": {
					"type": "string",
					"description": "城市名称，例如: 北京, 上海"
				}
			},
			"required": ["city"]
		}`,
		Execute: func(args string, onProgress func(string)) string {
			var params struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "Error: Invalid parameters"
			}
			
			// 模拟数据。以后可以接真实的第三方API
			return fmt.Sprintf("天气API返回: %s 目前天气晴朗，气温 25°C，适合外出。", params.City)
		},
	})

	// 注册重启 Bot 的技能（独立进程，不会被 Bot 进程死亡影响）
	RegisterSkill(Skill{
		Name:        "restart_bot",
		Description: "重启 QQ Bot 服务。当用户要求重启机器人、重新启动、restart bot 时使用。会自动检测当前运行模式并重启，无需传任何参数。",
		ParametersSchema: `{
			"type": "object",
			"properties": {},
			"required": []
		}`,
		Execute: func() func(args string, onProgress func(string)) string {
			var mu sync.Mutex
			var lastRestart time.Time
			return func(args string, onProgress func(string)) string {
				// 防重复：30 秒内只允许一次重启
				mu.Lock()
				if time.Since(lastRestart) < 30*time.Second {
					mu.Unlock()
					core.Log.Warn("⚠️ 重启冷却中，忽略重复请求")
					return "⚠️ 重启已在进行中，请稍等 30 秒~"
				}
				lastRestart = time.Now()
				mu.Unlock()

				// 强制使用当前运行模式，不让 LLM 瞎选
				mode := os.Getenv("APP_ENV")
				if mode == "" {
					mode = "dev"
				}

			// 获取项目目录（基于当前工作目录）
			projectDir, _ := os.Getwd()
			scriptPath := filepath.Join(projectDir, "skills", "claude-code", "scripts", "restart_bot.py")

			// 确保脚本存在
			if _, err := os.Stat(scriptPath); err != nil {
				return fmt.Sprintf("❌ 重启脚本不存在: %s", scriptPath)
			}

			core.Log.Infof("🔄 触发重启: mode=%s, dir=%s", mode, projectDir)

			if onProgress != nil {
				onProgress(fmt.Sprintf("🔄 正在准备重启 Bot (%s 模式)...", mode))
			}

			// 关键：用 nohup + setsid 完全脱离当前进程树
			// 这样即使当前 Bot 进程被杀，重启脚本也能继续运行
			cmd := exec.Command("nohup", "python3", scriptPath, mode, projectDir)
			cmd.Dir = projectDir
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Stdin = nil
			// 设置新的进程组，完全脱离父进程
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}

			if err := cmd.Start(); err != nil {
				return fmt.Sprintf("❌ 启动重启脚本失败: %v", err)
			}

			// 立即释放，不等待子进程结束
			cmd.Process.Release()

			// 返回成功信息（Bot 会在 2 秒后被杀掉并重启）
			result := fmt.Sprintf("✅ 重启指令已发出！\n"+
				"📋 模式: %s\n"+
				"📂 目录: %s\n"+
				"⏳ Bot 将在 2 秒后重启，届时会短暂离线，请稍等片刻~", mode, projectDir)

			// 将最终信息标记为 [FINAL] 方便识别
			lines := strings.Split(result, "\n")
			for _, l := range lines {
				if onProgress != nil {
					onProgress(l)
				}
			}

				return result
			}
		}(),
	})
}
