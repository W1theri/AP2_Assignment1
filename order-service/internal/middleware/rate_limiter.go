package middleware

import (
	"context"
	"fmt"
	"net"
	"time"

	"order-service/internal/cache"

	"github.com/gin-gonic/gin"
)

// RateLimiter middleware limits requests per IP using Redis INCR
type RateLimiter struct {
	cache          cache.Cache
	maxRequests    int
	windowDuration time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(c cache.Cache, maxRequests int, windowSeconds int) *RateLimiter {
	return &RateLimiter{
		cache:          c,
		maxRequests:    maxRequests,
		windowDuration: time.Duration(windowSeconds) * time.Second,
	}
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.maxRequests <= 0 {
			c.Next()
			return
		}

		clientIP := getClientIP(c)
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		count, err := rl.cache.Incr(ctx, key, rl.windowDuration)
		if err != nil {
			c.Next()
			return
		}

		if count > int64(rl.maxRequests) {
			c.JSON(429, gin.H{
				"error":       "Too many requests",
				"retry_after": int(rl.windowDuration.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getClientIP extracts the client IP from request
func getClientIP(c *gin.Context) string {
	if forwardedFor := c.GetHeader("X-Forwarded-For"); forwardedFor != "" {
		return forwardedFor
	}
	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return realIP
	}
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil {
		return ip
	}
	return c.Request.RemoteAddr
}
