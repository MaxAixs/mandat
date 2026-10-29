package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	loginAttemptLimit  = 5
	loginAttemptWindow = 5 * time.Minute
	maxTrackedLoginIPs = 10
)

// LoginRateLimiter applies a bounded in-memory fixed-window limit per client IP.
type LoginRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]loginWindow
	lastCleanup time.Time
	now         func() time.Time
}

type loginWindow struct {
	startedAt time.Time
	count     int
}

// NewLoginRateLimiter creates a limiter for ten attempts per five minutes.
func NewLoginRateLimiter() *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts:    make(map[string]loginWindow),
		lastCleanup: time.Now().UTC(),
		now:         time.Now,
	}
}

// Middleware rejects requests above the login attempt limit.
func (l *LoginRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			const respMsg = "Too many login attempts"
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"status":  http.StatusTooManyRequests,
					"message": respMsg,
				},
			})
			return
		}

		c.Next()
	}
}

func (l *LoginRateLimiter) allow(clientIP string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	if now.Sub(l.lastCleanup) >= loginAttemptWindow {
		for ip, window := range l.attempts {
			if now.Sub(window.startedAt) >= loginAttemptWindow {
				delete(l.attempts, ip)
			}
		}
		l.lastCleanup = now
	}

	window, ok := l.attempts[clientIP]
	if !ok || now.Sub(window.startedAt) >= loginAttemptWindow {
		if !ok && len(l.attempts) >= maxTrackedLoginIPs {
			return false
		}
		l.attempts[clientIP] = loginWindow{startedAt: now, count: 1}
		return true
	}
	if window.count >= loginAttemptLimit {
		return false
	}

	window.count++
	l.attempts[clientIP] = window

	return true
}
