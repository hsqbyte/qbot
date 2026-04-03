package services

import (
	"sync"
	"time"
)

// RateLimiter 消息频率限制器
// 防止机器人被利用发送垃圾消息
type RateLimiter struct {
	mu       sync.Mutex
	counters map[int64]*counter // groupID -> counter
	limit    int                // 每个窗口最大消息数
	window   time.Duration      // 时间窗口
}

type counter struct {
	count    int
	resetAt  time.Time
}

// NewRateLimiter 创建限制器
// limit: 时间窗口内最大回复次数, window: 时间窗口
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		counters: make(map[int64]*counter),
		limit:    limit,
		window:   window,
	}
}

// Allow 检查该群是否允许发送消息
func (r *RateLimiter) Allow(groupID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	c, ok := r.counters[groupID]

	if !ok || now.After(c.resetAt) {
		r.counters[groupID] = &counter{
			count:   1,
			resetAt: now.Add(r.window),
		}
		return true
	}

	if c.count >= r.limit {
		return false
	}

	c.count++
	return true
}

// DefaultLimiter 全局默认限制器: 每分钟最多回复 10 条
var DefaultLimiter = NewRateLimiter(10, time.Minute)
