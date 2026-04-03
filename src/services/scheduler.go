package services

import (
	"sync"
	"time"

	"github.com/hsqbyte/qbot/src/core"
	model "github.com/hsqbyte/qbot/src/models"
)

// Scheduler 定时任务管理器
type Scheduler struct {
	tasks  []*ScheduledTask
	sendFn func(action model.Action) error
	done   chan struct{}
	mu     sync.Mutex
}

// ScheduledTask 定时任务
type ScheduledTask struct {
	Name     string
	Cron     func(now time.Time) bool // 判断是否该执行
	Action   func() []model.Action   // 生成要执行的动作
	LastExec time.Time               // 上次执行时间（防重复）
}

// NewScheduler 创建调度器
func NewScheduler(sendFn func(action model.Action) error) *Scheduler {
	return &Scheduler{
		sendFn: sendFn,
		done:   make(chan struct{}),
	}
}

// AddTask 添加定时任务
func (s *Scheduler) AddTask(task *ScheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, task)
	core.Log.Infof("⏰ 注册定时任务: %s", task.Name)
}

// Start 启动调度循环
func (s *Scheduler) Start() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case now := <-ticker.C:
				s.tick(now)
			}
		}
	}()
}

// Stop 停止调度
func (s *Scheduler) Stop() {
	close(s.done)
}

func (s *Scheduler) tick(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		// 同一分钟内不重复执行
		if now.Sub(task.LastExec) < time.Minute {
			continue
		}

		if task.Cron(now) {
			task.LastExec = now
			actions := task.Action()
			for _, a := range actions {
				if err := s.sendFn(a); err != nil {
					core.Log.Errorf("[定时任务:%s] 发送失败: %v", task.Name, err)
				}
			}
			core.Log.Infof("[定时任务:%s] 已执行", task.Name)
		}
	}
}

// --- 预置定时任务 ---

// MorningGreeting 每日早安（每天 8:00）
func MorningGreeting(groupID int64) *ScheduledTask {
	return &ScheduledTask{
		Name: "早安问候",
		Cron: func(now time.Time) bool {
			return now.Hour() == 8 && now.Minute() == 0
		},
		Action: func() []model.Action {
			return []model.Action{
				model.NewSendGroupMsg(groupID, "☀️ 早上好！新的一天开始了，大家加油！"),
			}
		},
	}
}

// NightReminder 每日晚安提醒（每天 23:00）
func NightReminder(groupID int64) *ScheduledTask {
	return &ScheduledTask{
		Name: "晚安提醒",
		Cron: func(now time.Time) bool {
			return now.Hour() == 23 && now.Minute() == 0
		},
		Action: func() []model.Action {
			return []model.Action{
				model.NewSendGroupMsg(groupID, "🌙 夜深了，该休息啦~晚安！"),
			}
		},
	}
}
