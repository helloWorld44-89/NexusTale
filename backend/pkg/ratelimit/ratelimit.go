package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/auth"
)

type window struct {
	count  int
	resets time.Time
}

type limiter struct {
	mu      sync.Mutex
	max     int
	period  time.Duration
	windows map[string]*window
}

func newLimiter(max int, period time.Duration) *limiter {
	l := &limiter{
		max:     max,
		period:  period,
		windows: make(map[string]*window),
	}
	go l.sweep()
	return l
}

// sweep removes expired windows every period to prevent unbounded growth.
func (l *limiter) sweep() {
	ticker := time.NewTicker(l.period)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for k, w := range l.windows {
			if now.After(w.resets) {
				delete(l.windows, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *limiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[key]
	if !ok || now.After(w.resets) {
		l.windows[key] = &window{count: 1, resets: now.Add(l.period)}
		return true
	}
	if w.count >= l.max {
		return false
	}
	w.count++
	return true
}

// ByIP limits requests per remote IP address.
func ByIP(max int, period time.Duration) gin.HandlerFunc {
	l := newLimiter(max, period)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !l.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests — please wait before trying again",
			})
			return
		}
		c.Next()
	}
}

// ByUser limits requests per authenticated user ID.
// Falls back to IP limiting when no user is present in context.
func ByUser(max int, period time.Duration) gin.HandlerFunc {
	l := newLimiter(max, period)
	return func(c *gin.Context) {
		userID := auth.GetUserID(c)
		key := userID.String()
		if userID == uuid.Nil {
			key = "ip:" + c.ClientIP()
		}
		if !l.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "AI request limit reached — please wait a moment before generating again",
			})
			return
		}
		c.Next()
	}
}
