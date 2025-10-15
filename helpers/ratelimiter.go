package helpers

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rate rate.Limit
	burst int
	cleanupAfter time.Duration
	clients sync.Map
	once sync.Once
}

type clientInfo struct {
	limiter *rate.Limiter
	lastSeen int64
}

func NewRateLimiter(max int, per time.Duration) *RateLimiter {
	r := rate.Limit(float64(max) / per.Seconds())
	rl := &RateLimiter{
		rate: r,
		burst: max,
		cleanupAfter: 3 * time.Minute,
	}

	rl.once.Do(func() {
		go rl.cleanupLoop()
	})
	return rl
}

func (rl *RateLimiter) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
	key := clientIP(c)
	info := rl.getOrCreate(key)

	atomic.StoreInt64(&info.lastSeen, time.Now().UnixNano())

	remaining := int(info.limiter.Tokens())
		if remaining < 0 {
			remaining = 0
		}

		c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.burst))
		c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))


	if !info.limiter.Allow() {
		c.Response().Header().Set("Retry-After", strconv.Itoa(1))
		return JSONError(c, http.StatusTooManyRequests, "rate limit exceeded")
	}
	return next(c)
	}
}

func (rl *RateLimiter) getOrCreate(key string) *clientInfo {
	now := time.Now().UnixNano()
	if v, ok := rl.clients.Load(key); ok {
		return v.(*clientInfo)
	}

	info := &clientInfo{
		limiter:  rate.NewLimiter(rl.rate, rl.burst),
		lastSeen: now,
	}
	actual, loaded := rl.clients.LoadOrStore(key, info)
	if loaded {
		return actual.(*clientInfo)
	}
	return info
}

func (rl *RateLimiter) cleanupLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-rl.cleanupAfter).UnixNano()
		rl.clients.Range(func(k, v any) bool {
			info := v.(*clientInfo)
			if atomic.LoadInt64(&info.lastSeen) < cutoff {
				rl.clients.Delete(k)
			}
			return true
		})
	}
}

func clientIP(c echo.Context) string {
	if ip := c.RealIP(); ip != "" {
		if host, _, err := net.SplitHostPort(ip); err == nil {
			return host
		}
		return ip
	}
	if host, _, err := net.SplitHostPort(c.Request().RemoteAddr); err == nil {
		return host
	}
	return c.Request().RemoteAddr
}

