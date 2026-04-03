package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Cfg 全局配置实例
var Cfg *Config

// Config 应用配置
type Config struct {
	WebSocket WebSocketConfig `yaml:"websocket"`
	Bot       BotConfig       `yaml:"bot"`
	Log       LogConfig       `yaml:"log"`
	AI        AIConfig        `yaml:"ai"`
}

// WebSocketConfig NapCat 连接配置
type WebSocketConfig struct {
	URL               string `yaml:"url"`
	Token             string `yaml:"token"`
	ReconnectInterval int    `yaml:"reconnect_interval"`
}

// BotConfig 机器人行为配置
type BotConfig struct {
	Nickname      string  `yaml:"nickname"`
	CommandPrefix string  `yaml:"command_prefix"`
	Admins        []int64 `yaml:"admins"`
	Groups        []int64 `yaml:"groups"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `yaml:"level"`
}

// AIConfig LLM 大模型配置
type AIConfig struct {
	Enable    bool   `yaml:"enable"`
	BaseURL   string `yaml:"base_url"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	Prompt    string `yaml:"prompt"`
	GroupMode string `yaml:"group_mode"` // always: 回复所有群消息, at_only: 仅@bot时回复
}

// LoadConfig 根据环境加载对应的配置文件
func LoadConfig(env string) (*Config, error) {
	filename := fmt.Sprintf("config/%s.yaml", env)

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("获取配置文件绝对路径失败: %w", err)
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件 %s: %w", absPath, err)
	}

	cfg := &Config{
		WebSocket: WebSocketConfig{
			URL:               "ws://127.0.0.1:3001",
			ReconnectInterval: 5,
		},
		Bot: BotConfig{
			Nickname:      "小助手",
			CommandPrefix: "/",
		},
		Log: LogConfig{
			Level: "info",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 环境变量覆盖敏感配置（优先级高于 YAML）
	cfg.overrideFromEnv()

	// 校验必填项
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}

	return cfg, nil
}

// overrideFromEnv 用环境变量覆盖敏感配置项
func (c *Config) overrideFromEnv() {
	if v := os.Getenv("WS_TOKEN"); v != "" {
		c.WebSocket.Token = v
	}
	if v := os.Getenv("AI_API_KEY"); v != "" {
		c.AI.APIKey = v
	}
	if v := os.Getenv("AI_BASE_URL"); v != "" {
		c.AI.BaseURL = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		c.AI.Model = v
	}
}

// Validate 校验配置的必填项
func (c *Config) Validate() error {
	if c.WebSocket.URL == "" {
		return errors.New("websocket.url 不能为空")
	}
	if c.AI.Enable {
		if c.AI.APIKey == "" {
			return errors.New("AI 已启用但 api_key 未配置（可通过环境变量 AI_API_KEY 设置）")
		}
		if c.AI.Model == "" {
			return errors.New("AI 已启用但 model 未配置")
		}
	}
	return nil
}
