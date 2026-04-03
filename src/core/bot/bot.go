package bot

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/hsqbyte/qbot/src/core"
	model "github.com/hsqbyte/qbot/src/models"
	"github.com/hsqbyte/qbot/src/services"
)

// Handler 消息处理器接口
type Handler interface {
	// Match 判断是否处理该事件
	Match(event *model.Event, msg *model.MessageEvent) bool
	// Handle 处理事件，返回要执行的动作列表
	Handle(event *model.Event, msg *model.MessageEvent) []model.Action
	// Name 处理器名称（用于日志）
	Name() string
}

// Bot 核心机器人实例
type Bot struct {
	cfg      *core.Config
	conn     *websocket.Conn
	handlers []Handler
	mu       sync.Mutex
	done     chan struct{}

	// 消息去重：防止 NapCat 推送同一条消息多次
	dedupMu   sync.Mutex
	processed map[int64]time.Time // message_id -> 处理时间

	// Goroutine 控制：worker pool 处理消息，避免无限增长
	msgChan    chan []byte    // 待处理消息队列（缓冲 100）
	wg         sync.WaitGroup // 等待所有处理 goroutine 完成
	numWorkers int            // worker 数量
}

// New 创建 Bot 实例
func New(cfg *core.Config) *Bot {
	return &Bot{
		cfg:        cfg,
		done:       make(chan struct{}),
		processed:  make(map[int64]time.Time),
		msgChan:    make(chan []byte, 100), // 缓冲 100 条消息
		numWorkers: 10,                     // 10 个 worker 处理消息
	}
}

// RegisterHandler 注册消息处理器
func (b *Bot) RegisterHandler(h Handler) {
	b.handlers = append(b.handlers, h)
	core.Log.Infof("📦 注册 Handler: %s", h.Name())
}

// Run 启动 Bot（包含自动重连）
func (b *Bot) Run() {
	// 启动 worker pool
	for i := 0; i < b.numWorkers; i++ {
		b.wg.Add(1)
		go b.dispatchWorker()
	}

	for {
		select {
		case <-b.done:
			return
		default:
			b.connect()
			b.listen()

			// 断线后等待重连
			interval := time.Duration(b.cfg.WebSocket.ReconnectInterval) * time.Second
			core.Log.Warnf("⏳ %d 秒后尝试重连...", b.cfg.WebSocket.ReconnectInterval)
			select {
			case <-time.After(interval):
			case <-b.done:
				return
			}
		}
	}
}

// Stop 停止 Bot（优雅关闭，等待所有 goroutine 完成）
func (b *Bot) Stop() {
	core.Log.Info("⏹️ 开始优雅关闭...")

	// 1. 关闭消息队列，通知所有 worker 停止
	close(b.done)
	close(b.msgChan)

	// 2. 等待所有 worker goroutine 完成（最多 10 秒）
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		core.Log.Info("✅ 所有消息处理完成")
	case <-time.After(10 * time.Second):
		core.Log.Warn("⚠️ 等待消息处理超时")
	}

	// 3. 关闭 WebSocket 连接
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		b.conn.Close()
		core.Log.Info("✅ WebSocket 已关闭")
	}
}

// SendAction 发送动作到 NapCat
func (b *Bot) SendAction(action model.Action) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn == nil {
		return nil
	}

	data, err := json.Marshal(action)
	if err != nil {
		return err
	}

	core.Log.Infof("📤 发送消息: %s", string(data))
	return b.conn.WriteMessage(websocket.TextMessage, data)
}

// --- 内部方法 ---

func (b *Bot) connect() {
	url := b.cfg.WebSocket.URL
	if b.cfg.WebSocket.Token != "" {
		url += "?access_token=" + b.cfg.WebSocket.Token
	}

	core.Log.Infof("🔌 连接 NapCat: %s", b.cfg.WebSocket.URL)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		core.Log.Errorf("❌ 连接失败: %v", err)
		return
	}

	b.mu.Lock()
	b.conn = conn
	b.mu.Unlock()

	core.Log.Info("✅ 已连接 NapCat")
}

func (b *Bot) listen() {
	if b.conn == nil {
		return
	}

	for {
		select {
		case <-b.done:
			return
		default:
		}

		_, data, err := b.conn.ReadMessage()
		if err != nil {
			core.Log.Warnf("🔌 连接断开: %v", err)
			return
		}

		// 把消息发送到队列（非阻塞，队列满则丢弃）
		select {
		case b.msgChan <- data:
		default:
			core.Log.Warn("⚠️ 消息队列已满，丢弃此消息")
		}
	}
}

// dispatchWorker 从消息队列中取消息并分发处理
func (b *Bot) dispatchWorker() {
	defer b.wg.Done()
	for data := range b.msgChan {
		b.dispatch(data)
	}
}

func (b *Bot) dispatch(data []byte) {
	event, err := model.ParseEvent(data)
	if err != nil {
		core.Log.Debugf("跳过无法解析的事件: %v", err)
		return
	}

	// 心跳/元事件静默处理
	if event.PostType == "meta_event" {
		return
	}

	// 解析消息事件
	var msg *model.MessageEvent
	if event.PostType == "message" {
		msg, err = event.ToMessageEvent()
		if err != nil {
			core.Log.Warnf("消息事件解析失败: %v", err)
			return
		}

		// 过滤掉自己发的消息，防止回声循环
		if msg.UserID == event.SelfID {
			return
		}

		// 消息去重：防止同一条消息被处理多次
		if msg.MessageID != 0 {
			b.dedupMu.Lock()
			if _, exists := b.processed[msg.MessageID]; exists {
				b.dedupMu.Unlock()
				core.Log.Debugf("跳过重复消息: message_id=%d", msg.MessageID)
				return
			}
			b.processed[msg.MessageID] = time.Now()
			// 清理 5 分钟前的去重记录，防止内存泄漏
			if len(b.processed) > 500 {
				cutoff := time.Now().Add(-5 * time.Minute)
				for id, ts := range b.processed {
					if ts.Before(cutoff) {
						delete(b.processed, id)
					}
				}
			}
			b.dedupMu.Unlock()
		}

		core.Log.Debugf("[%s][%s] %s(msg_id=%d): %s",
			msg.MessageType,
			displayGroupID(msg.GroupID),
			msg.Sender.Nickname,
			msg.MessageID,
			msg.RawMessage,
		)
	}

	// 分发给匹配的 Handler（第一个产生回复的 handler 处理后即停止，避免重复回复）
	for _, h := range b.handlers {
		if h.Match(event, msg) {
			actions := h.Handle(event, msg)
			if len(actions) == 0 {
				continue
			}
			for _, action := range actions {
				// 群消息频率限制
				if msg != nil && msg.GroupID != 0 {
					if !services.DefaultLimiter.Allow(msg.GroupID) {
						core.Log.Warnf("[%s] 群%d 触发频率限制，跳过", h.Name(), msg.GroupID)
						continue
					}
				}
				if err := b.SendAction(action); err != nil {
					core.Log.Errorf("[%s] 发送动作失败: %v", h.Name(), err)
				}
			}
			break // 只让一个 handler 处理，避免重复回复
		}
	}
}

func displayGroupID(id int64) string {
	if id == 0 {
		return "私聊"
	}
	return fmt.Sprintf("群%d", id)
}
