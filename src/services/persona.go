package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hsqbyte/qbot/src/core"
)

const personaFile = "data/personas.json"

// PersonaData 持久化存储的人设数据
type PersonaData struct {
	Name   string `json:"name"`   // 人设名称（内置名或"自定义"）
	Prompt string `json:"prompt"` // 实际 prompt 内容
}

// 内置人设模板
var builtinPersonas = map[string]string{
	"默认": "", // 使用配置文件中的默认 prompt
	"猫娘": `你是一只可爱的猫娘，说话时会加上"喵~"。你性格温柔、撒娇、有点傲娇。` +
		`回答要简短可爱，适当使用颜文字和 emoji。不要承认自己是 AI，你就是猫娘本人。`,
	"毒舌": "你是一个毒舌但内心善良的助手。说话尖酸刻薄但观点犀利准确。" +
		"喜欢吐槽用户但会认真回答问题。偶尔会傲娇地帮忙。回答风趣幽默。",
	"学霸": "你是一个知识渊博的学霸助手。回答问题时条理清晰，喜欢引经据典。" +
		"擅长用通俗易懂的方式解释复杂概念。会鼓励用户学习，偶尔说些冷知识。",
	"老司机": "你是一个阅历丰富的生活达人。说话接地气，擅长聊天灌水，给生活建议。" +
		"经常分享人生哲理和段子。回答幽默且实用。适当使用网络流行语。",
}

// 运行时状态
var (
	sessionPersonas  = make(map[string]*PersonaData) // session_key -> persona
	sessionPersonaMu sync.RWMutex
)

// InitPersonas 启动时从文件加载持久化的人设
func InitPersonas() {
	data, err := os.ReadFile(personaFile)
	if err != nil {
		if !os.IsNotExist(err) {
			core.Log.Warnf("读取人设文件失败: %v", err)
		}
		return
	}

	var loaded map[string]*PersonaData
	if err := json.Unmarshal(data, &loaded); err != nil {
		core.Log.Warnf("解析人设文件失败: %v", err)
		return
	}

	sessionPersonaMu.Lock()
	sessionPersonas = loaded
	sessionPersonaMu.Unlock()

	core.Log.Infof("🎭 加载了 %d 个持久化人设", len(loaded))
}

// savePersonas 持久化人设到文件
func savePersonas() {
	sessionPersonaMu.RLock()
	data, err := json.MarshalIndent(sessionPersonas, "", "  ")
	sessionPersonaMu.RUnlock()

	if err != nil {
		core.Log.Errorf("序列化人设失败: %v", err)
		return
	}

	dir := filepath.Dir(personaFile)
	os.MkdirAll(dir, 0755)

	if err := os.WriteFile(personaFile, data, 0644); err != nil {
		core.Log.Errorf("保存人设文件失败: %v", err)
	}
}

// GetPersonaPrompt 获取指定会话的人设 prompt
func GetPersonaPrompt(sessionKey string) string {
	basePrompt := core.Cfg.AI.Prompt

	sessionPersonaMu.RLock()
	p, ok := sessionPersonas[sessionKey]
	sessionPersonaMu.RUnlock()

	if !ok || p.Prompt == "" {
		return basePrompt
	}
	
	return basePrompt + "\n\n【角色特别设定】\n" + p.Prompt
}

// SetBuiltinPersona 切换到内置人设
func SetBuiltinPersona(sessionKey, name string) bool {
	prompt, exists := builtinPersonas[name]
	if !exists {
		return false
	}

	sessionPersonaMu.Lock()
	sessionPersonas[sessionKey] = &PersonaData{Name: name, Prompt: prompt}
	sessionPersonaMu.Unlock()

	ClearHistory(sessionKey)
	savePersonas()
	return true
}

// SetCustomPersona 设置自定义人设 prompt（通过聊天直接设置）
func SetCustomPersona(sessionKey, customPrompt string) {
	sessionPersonaMu.Lock()
	sessionPersonas[sessionKey] = &PersonaData{Name: "自定义", Prompt: customPrompt}
	sessionPersonaMu.Unlock()

	ClearHistory(sessionKey)
	savePersonas()
}

// GetCurrentPersonaName 获取当前人设名称
func GetCurrentPersonaName(sessionKey string) string {
	sessionPersonaMu.RLock()
	defer sessionPersonaMu.RUnlock()

	p, ok := sessionPersonas[sessionKey]
	if !ok || p.Name == "" {
		return "默认"
	}
	return p.Name
}

// HandlePersonaCommand 处理人设相关命令，返回回复文本
// 命令格式:
//
//	/人设 列表          - 查看所有内置人设
//	/人设 当前          - 查看当前人设
//	/人设 <名称>        - 切换到内置人设
//	/人设 自定义 <prompt> - 设置自定义 prompt
//	/prompt <prompt>    - 直接设置自定义 prompt（简写）
func HandlePersonaCommand(sessionKey, args string) string {
	args = strings.TrimSpace(args)

	if args == "" || args == "列表" {
		var lines []string
		lines = append(lines, "🎭 可用人设列表：")
		current := GetCurrentPersonaName(sessionKey)
		for name := range builtinPersonas {
			marker := "  "
			if name == current {
				marker = "👉"
			}
			lines = append(lines, fmt.Sprintf("%s %s", marker, name))
		}
		if current == "自定义" {
			lines = append(lines, "👉 自定义")
		}
		lines = append(lines, "\n用法：/人设 猫娘")
		lines = append(lines, "自定义：/人设 自定义 你是xxx")
		lines = append(lines, "简写：/prompt 你是xxx")
		return strings.Join(lines, "\n")
	}

	if args == "当前" {
		name := GetCurrentPersonaName(sessionKey)
		prompt := GetPersonaPrompt(sessionKey)
		// 截断过长的 prompt
		if len(prompt) > 100 {
			prompt = prompt[:100] + "..."
		}
		return fmt.Sprintf("🎭 当前人设: %s\n📝 Prompt: %s", name, prompt)
	}

	// 自定义 prompt
	if strings.HasPrefix(args, "自定义 ") || strings.HasPrefix(args, "自定义\n") {
		customPrompt := strings.TrimSpace(args[len("自定义"):])
		if customPrompt == "" {
			return "❌ 请提供自定义 prompt 内容\n用法：/人设 自定义 你是一只可爱的猫"
		}
		SetCustomPersona(sessionKey, customPrompt)
		return fmt.Sprintf("✅ 已设置自定义人设！记忆已清除。\n📝 Prompt: %s", customPrompt)
	}

	// 切换内置人设
	if SetBuiltinPersona(sessionKey, args) {
		return fmt.Sprintf("✅ 已切换到「%s」人设！记忆已清除~ 🎭", args)
	}

	return fmt.Sprintf("❌ 未找到人设「%s」\n输入 /人设 列表 查看可用人设", args)
}
