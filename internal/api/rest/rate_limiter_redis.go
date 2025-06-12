package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

// RedisRateLimiter implements distributed rate limiting using Redis
type RedisRateLimiter struct {
	client       *redis.Client
	config       RateLimitConfig
	localLimiter sync.Map // Local cache for performance
	tracer       trace.Tracer
}

// RateLimitResult contains rate limit check results
type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
	RetryAfter time.Duration
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter
func NewRedisRateLimiter(client *redis.Client, config RateLimitConfig) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		config: config,
		tracer: otel.Tracer("api.rest.ratelimit"),
	}
}

// CheckLimit checks if a request should be allowed
func (r *RedisRateLimiter) CheckLimit(ctx context.Context, key string) (*RateLimitResult, error) {
	ctx, span := r.tracer.Start(ctx, "ratelimit.check",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int("limit", r.config.RequestsPerSecond),
		),
	)
	defer span.End()

	// Try local limiter first for performance
	if r.config.RequestsPerSecond > 100 {
		if limiter, ok := r.localLimiter.Load(key); ok {
			if l := limiter.(*rate.Limiter); l.Allow() {
				return &RateLimitResult{
					Allowed:   true,
					Limit:     r.config.RequestsPerSecond,
					Remaining: int(l.Tokens()),
				}, nil
			}
		}
	}

	// Use Redis for distributed rate limiting
	now := time.Now()
	window := now.Truncate(time.Second).Unix()
	redisKey := fmt.Sprintf("ratelimit:%s:%d", key, window)

	// Increment counter atomically
	count, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		span.RecordError(err)
		// Fall back to local rate limiting on Redis failure
		return r.fallbackToLocal(key)
	}

	// Set expiration on first request in window
	if count == 1 {
		r.client.Expire(ctx, redisKey, 2*time.Second)
	}

	allowed := count <= int64(r.config.RequestsPerSecond)
	remaining := r.config.RequestsPerSecond - int(count)
	if remaining < 0 {
		remaining = 0
	}

	result := &RateLimitResult{
		Allowed:   allowed,
		Limit:     r.config.RequestsPerSecond,
		Remaining: remaining,
		ResetAt:   time.Unix(window+1, 0),
	}

	if !allowed {
		result.RetryAfter = time.Until(result.ResetAt)
	}

	span.SetAttributes(
		attribute.Bool("allowed", allowed),
		attribute.Int("count", int(count)),
		attribute.Int("remaining", remaining),
	)

	return result, nil
}

// CheckLimitWithCost checks rate limit with variable cost
func (r *RedisRateLimiter) CheckLimitWithCost(ctx context.Context, key string, cost int) (*RateLimitResult, error) {
	ctx, span := r.tracer.Start(ctx, "ratelimit.check_with_cost",
		trace.WithAttributes(
			attribute.String("key", key),
			attribute.Int("cost", cost),
		),
	)
	defer span.End()

	// Use sliding window for more accurate rate limiting
	now := time.Now()
	windowStart := now.Add(-time.Second).UnixNano()
	windowEnd := now.UnixNano()

	// Remove old entries
	r.client.ZRemRangeByScore(ctx, "rl:"+key, "0", strconv.FormatInt(windowStart, 10))

	// Count requests in current window
	count, err := r.client.ZCount(ctx, "rl:"+key, strconv.FormatInt(windowStart, 10), strconv.FormatInt(windowEnd, 10)).Result()
	if err != nil {
		return r.fallbackToLocal(key)
	}

	if count+int64(cost) > int64(r.config.RequestsPerSecond) {
		return &RateLimitResult{
			Allowed:    false,
			Limit:      r.config.RequestsPerSecond,
			Remaining:  0,
			ResetAt:    now.Add(time.Second),
			RetryAfter: time.Second,
		}, nil
	}

	// Add current request
	member := &redis.Z{
		Score:  float64(now.UnixNano()),
		Member: uuid.New().String(),
	}
	r.client.ZAdd(ctx, "rl:"+key, member)
	r.client.Expire(ctx, "rl:"+key, 2*time.Second)

	return &RateLimitResult{
		Allowed:   true,
		Limit:     r.config.RequestsPerSecond,
		Remaining: r.config.RequestsPerSecond - int(count) - cost,
		ResetAt:   now.Add(time.Second),
	}, nil
}

// Reset resets the rate limit for a key
func (r *RedisRateLimiter) Reset(ctx context.Context, key string) error {
	pattern := fmt.Sprintf("ratelimit:%s:*", key)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}

	// Also reset sliding window key
	return r.client.Del(ctx, "rl:"+key).Err()
}

// Middleware returns a rate limiting middleware
func (r *RedisRateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := r.getKey(req)
			
			result, err := r.CheckLimit(req.Context(), key)
			if err != nil {
				// Log error but allow request on rate limiter failure
				span := trace.SpanFromContext(req.Context())
				span.RecordError(err)
				next.ServeHTTP(w, req)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
				writeRateLimitExceeded(w)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

// Private methods

func (r *RedisRateLimiter) getKey(req *http.Request) string {
	if r.config.CustomKeyFunc != nil {
		return r.config.CustomKeyFunc(req)
	}

	var parts []string

	if r.config.ByIP {
		parts = append(parts, getClientIP(req))
	}

	if r.config.ByUser {
		if userID, ok := req.Context().Value(contextKeyUserID).(uuid.UUID); ok {
			parts = append(parts, userID.String())
		}
	}

	if r.config.ByEndpoint {
		parts = append(parts, req.Method, req.URL.Path)
	}

	if len(parts) == 0 {
		parts = append(parts, "global")
	}

	return strings.Join(parts, ":")
}

func (r *RedisRateLimiter) fallbackToLocal(key string) (*RateLimitResult, error) {
	// Create or get local limiter
	limiterInterface, _ := r.localLimiter.LoadOrStore(key, rate.NewLimiter(
		rate.Limit(r.config.RequestsPerSecond),
		r.config.Burst,
	))
	limiter := limiterInterface.(*rate.Limiter)

	allowed := limiter.Allow()
	
	return &RateLimitResult{
		Allowed:   allowed,
		Limit:     r.config.RequestsPerSecond,
		Remaining: int(limiter.Tokens()),
		ResetAt:   time.Now().Add(time.Second),
		RetryAfter: func() time.Duration {
			if !allowed {
				return time.Second
			}
			return 0
		}(),
	}, nil
}

// DistributedRateLimiter provides global rate limiting across all instances
type DistributedRateLimiter struct {
	redis  *redis.Client
	config DistributedRateLimitConfig
	tracer trace.Tracer
}

// DistributedRateLimitConfig configures distributed rate limiting
type DistributedRateLimitConfig struct {
	GlobalLimit       int           // Total requests across all instances
	PerInstanceLimit  int           // Limit per instance
	WindowDuration    time.Duration // Time window for rate limiting
	SyncInterval      time.Duration // How often to sync with Redis
}

// NewDistributedRateLimiter creates a distributed rate limiter
func NewDistributedRateLimiter(redis *redis.Client, config DistributedRateLimitConfig) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		redis:  redis,
		config: config,
		tracer: otel.Tracer("api.rest.distributed_ratelimit"),
	}
}

// CheckGlobalLimit checks the global rate limit
func (d *DistributedRateLimiter) CheckGlobalLimit(ctx context.Context) (bool, error) {
	ctx, span := d.tracer.Start(ctx, "distributed_ratelimit.check_global")
	defer span.End()

	now := time.Now()
	window := now.Truncate(d.config.WindowDuration).Unix()
	key := fmt.Sprintf("global_ratelimit:%d", window)

	count, err := d.redis.Incr(ctx, key).Result()
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	if count == 1 {
		d.redis.Expire(ctx, key, d.config.WindowDuration+time.Second)
	}

	allowed := count <= int64(d.config.GlobalLimit)
	
	span.SetAttributes(
		attribute.Bool("allowed", allowed),
		attribute.Int("count", int(count)),
		attribute.Int("limit", d.config.GlobalLimit),
	)

	return allowed, nil
}

// AcquireToken tries to acquire a token from the global pool
func (d *DistributedRateLimiter) AcquireToken(ctx context.Context, tokens int) (bool, error) {
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local tokens = tonumber(ARGV[2])
		local window = tonumber(ARGV[3])
		
		local current = redis.call('GET', key) or 0
		current = tonumber(current)
		
		if current + tokens <= limit then
			redis.call('INCRBY', key, tokens)
			redis.call('EXPIRE', key, window)
			return {1, limit - (current + tokens)}
		else
			return {0, 0}
		end
	`

	now := time.Now()
	window := now.Truncate(d.config.WindowDuration).Unix()
	key := fmt.Sprintf("token_bucket:%d", window)

	result, err := d.redis.Eval(ctx, script, []string{key}, 
		d.config.GlobalLimit, tokens, int(d.config.WindowDuration.Seconds())).Result()
	
	if err != nil {
		return false, err
	}

	values := result.([]interface{})
	allowed := values[0].(int64) == 1
	
	return allowed, nil
}