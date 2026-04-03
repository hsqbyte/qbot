package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/sashabaranov/go-openai"
)

const (
	defaultMaxHistory = 30               // 默认保留最近 30 条消息（15 轮对话）
	historyExpiry     = 30 * time.Minute // 30分钟不活跃自动清理记忆
	historyFile       = "data/chat_history.json"
	saveInterval      = 2 * time.Minute // 每2分钟自动保存一次
)

// persistMessage JSON 持久化用的消息结构（openai.ChatCompletionMessage 不方便直接序列化）
type persistMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// persistSession JSON 持久化用的会话结构
type persistSession struct {
	Messages  []persistMessage `json:"messages"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// chatSession 运行时会话结构
type chatSession struct {
	Messages  []openai.ChatCompletionMessage
	UpdatedAt time.Time
}

// 按会话 key 存储对话历史
// 私聊: key = "private:<userID>"
// 群聊: key = "group:<groupID>"
var (
	chatSessions  = make(map[string]*chatSession)
	chatSessionMu sync.RWMutex
	historyDirty  bool // 标记是否有未保存的变更
)

// SessionKey 生成会话 key
func SessionKey(messageType string, userID, groupID int64) string {
	if messageType == "group" && groupID != 0 {
		return fmt.Sprintf("group:%d", groupID)
	}
	return fmt.Sprintf("private:%d", userID)
}

// InitHistory 启动时从文件加载持久化的对话历史
func InitHistory() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			core.Log.Warnf("读取对话历史文件失败: %v", err)
		}
		return
	}

	var loaded map[string]*persistSession
	if err := json.Unmarshal(data, &loaded); err != nil {
		core.Log.Warnf("解析对话历史文件失败: %v", err)
		return
	}

	chatSessionMu.Lock()
	now := time.Now()
	count := 0
	for key, ps := range loaded {
		// 跳过已过期的会话
		if now.Sub(ps.UpdatedAt) > historyExpiry {
			continue
		}
		session := &chatSession{
			UpdatedAt: ps.UpdatedAt,
		}
		for _, pm := range ps.Messages {
			session.Messages = append(session.Messages, openai.ChatCompletionMessage{
				Role:    pm.Role,
				Content: pm.Content,
			})
		}
		chatSessions[key] = session
		count++
	}
	chatSessionMu.Unlock()

	core.Log.Infof("💬 加载了 %d 个持久化会话记忆", count)
}

// SaveHistory 持久化对话历史到文件
func SaveHistory() {
	chatSessionMu.RLock()
	if !historyDirty {
		chatSessionMu.RUnlock()
		return
	}

	// 在锁内快速拷贝数据，然后释放锁
	toSave := make(map[string]*persistSession, len(chatSessions))
	now := time.Now()
	for key, session := range chatSessions {
		// 不保存过期的会话
		if now.Sub(session.UpdatedAt) > historyExpiry {
			continue
		}
		ps := &persistSession{
			UpdatedAt: session.UpdatedAt,
		}
		for _, msg := range session.Messages {
			ps.Messages = append(ps.Messages, persistMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
		toSave[key] = ps
	}
	chatSessionMu.RUnlock()

	// 在锁外执行耗时的文件 I/O
	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		core.Log.Errorf("序列化对话历史失败: %v", err)
		return
	}

	dir := filepath.Dir(historyFile)
	os.MkdirAll(dir, 0755)

	if err := os.WriteFile(historyFile, data, 0644); err != nil {
		core.Log.Errorf("保存对话历史文件失败: %v", err)
		return
	}

	chatSessionMu.Lock()
	historyDirty = false
	chatSessionMu.Unlock()

	core.Log.Debugf("💾 对话历史已保存 (%d 个会话)", len(toSave))
}

// GetHistory 获取会话的对话历史
func GetHistory(key string) []openai.ChatCompletionMessage {
	chatSessionMu.RLock()
	defer chatSessionMu.RUnlock()

	session, ok := chatSessions[key]
	if !ok {
		return nil
	}

	// 检查过期
	if time.Since(session.UpdatedAt) > historyExpiry {
		return nil
	}

	return session.Messages
}

// AppendHistory 追加一轮对话并自动截断
func AppendHistory(key string, userMsg, assistantMsg, nickname string) {
	chatSessionMu.Lock()
	defer chatSessionMu.Unlock()

	session, ok := chatSessions[key]
	if !ok {
		session = &chatSession{}
		chatSessions[key] = session
	}

	// 如果已过期，先清空再追加
	if time.Since(session.UpdatedAt) > historyExpiry {
		session.Messages = nil
	}

	// 群聊消息带上发言人标记
	content := userMsg
	if nickname != "" {
		content = fmt.Sprintf("[%s]: %s", nickname, userMsg)
	}

	session.Messages = append(session.Messages,
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: content,
		},
		openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: assistantMsg,
		},
	)

	// 超过上限截断
	if len(session.Messages) > defaultMaxHistory {
		session.Messages = session.Messages[len(session.Messages)-defaultMaxHistory:]
	}

	session.UpdatedAt = time.Now()
	historyDirty = true
}

// ClearHistory 清除指定会话的对话历史
func ClearHistory(key string) {
	chatSessionMu.Lock()
	defer chatSessionMu.Unlock()
	delete(chatSessions, key)
	historyDirty = true
}

// CleanExpiredSessions 定期清理过期会话
func CleanExpiredSessions() {
	chatSessionMu.Lock()
	defer chatSessionMu.Unlock()

	now := time.Now()
	for key, session := range chatSessions {
		if now.Sub(session.UpdatedAt) > historyExpiry {
			delete(chatSessions, key)
			historyDirty = true
		}
	}
}

// StartHistoryCleaner 启动后台清理和自动保存协程，传入 stop channel 用于优雅关闭
func StartHistoryCleaner(stop <-chan struct{}) {
	go func() {
		saveTicker := time.NewTicker(saveInterval)
		cleanTicker := time.NewTicker(10 * time.Minute)
		defer saveTicker.Stop()
		defer cleanTicker.Stop()

		for {
			select {
			case <-stop:
				SaveHistory() // 退出前最后保存一次
				return
			case <-saveTicker.C:
				SaveHistory()
			case <-cleanTicker.C:
				CleanExpiredSessions()
				SaveHistory()
			}
		}
	}()
}
