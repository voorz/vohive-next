package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/voorz/vohive/internal/config"
)

const (
	defaultHourlyLimit = 3
	defaultDailyLimit  = 10
)

type smsRateLimitResult struct {
	Allowed    bool
	Code       string
	Message    string
	RetryAfter time.Duration
}

type smsRateLimitDay struct {
	Year  int
	Month time.Month
	Day   int
}

type smsRateLimiter struct {
	mu             sync.Mutex
	startedAt      time.Time
	now            func() time.Time
	day            smsRateLimitDay
	firstHourCount int
	dailyCount     int
	hourlyLimit    int
	dailyLimit     int
}

func newSMSRateLimiter(startedAt time.Time, now func() time.Time) *smsRateLimiter {
	if now == nil {
		now = time.Now
	}
	if startedAt.IsZero() {
		startedAt = now()
	}
	l := &smsRateLimiter{
		startedAt:   startedAt,
		now:         now,
		day:         smsRateLimitDayOf(startedAt),
		hourlyLimit: defaultHourlyLimit,
		dailyLimit:  defaultDailyLimit,
	}
	l.reloadLimits()
	return l
}

// reloadLimits 从全局配置读取限制参数，留空回退默认值
func (l *smsRateLimiter) reloadLimits() {
	if cfg := config.GetConfig(); cfg != nil {
		if cfg.SMSRateLimit.HourlyLimit > 0 {
			l.hourlyLimit = cfg.SMSRateLimit.HourlyLimit
		}
		if cfg.SMSRateLimit.DailyLimit > 0 {
			l.dailyLimit = cfg.SMSRateLimit.DailyLimit
		}
	}
}

// UpdateLimits 更新限制参数（线程安全）
func (l *smsRateLimiter) UpdateLimits(hourly, daily int) {
	if hourly <= 0 || daily <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hourlyLimit = hourly
	l.dailyLimit = daily
}

// Limits 返回当前限制参数
func (l *smsRateLimiter) Limits() (hourly, daily int) {
	if l == nil {
		return defaultHourlyLimit, defaultDailyLimit
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.hourlyLimit, l.dailyLimit
}

func smsRateLimitDayOf(t time.Time) smsRateLimitDay {
	y, m, d := t.Date()
	return smsRateLimitDay{Year: y, Month: m, Day: d}
}

func (l *smsRateLimiter) Allow() smsRateLimitResult {
	if l == nil {
		return smsRateLimitResult{Allowed: true}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	day := smsRateLimitDayOf(now)
	if day != l.day {
		l.day = day
		l.dailyCount = 0
	}

	if now.Before(l.startedAt.Add(time.Hour)) && l.firstHourCount >= l.hourlyLimit {
		return smsRateLimitResult{
			Code:       "first_hour_limit",
			Message:    fmt.Sprintf("首次运行 1 小时内最多只能发送 %d 条短信，请稍后再试", l.hourlyLimit),
			RetryAfter: l.startedAt.Add(time.Hour).Sub(now),
		}
	}
	if l.dailyCount >= l.dailyLimit {
		return smsRateLimitResult{
			Code:       "daily_limit",
			Message:    fmt.Sprintf("每日最多只能发送 %d 条短信，请明天再试", l.dailyLimit),
			RetryAfter: nextLocalMidnight(now).Sub(now),
		}
	}

	if now.Before(l.startedAt.Add(time.Hour)) {
		l.firstHourCount++
	}
	l.dailyCount++
	return smsRateLimitResult{Allowed: true}
}

func nextLocalMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}
