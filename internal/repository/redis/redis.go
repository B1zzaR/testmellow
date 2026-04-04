package redisrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vpnplatform/internal/config"
)

func New(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// ─── Rate Limiting ────────────────────────────────────────────────────────────

// Increment returns (current count, error). Key expires after ttl from first call.
func Increment(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (int64, error) {
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// ─── Idempotency / Deduplication ─────────────────────────────────────────────

// SetNX returns true if the key was set (first time), false if already exists.
func SetNX(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (bool, error) {
	return rdb.SetNX(ctx, key, 1, ttl).Result()
}

// ─── Distributed Lock ─────────────────────────────────────────────────────────

const lockVal = "1"

// TryLock attempts to acquire a lock. Returns true on success.
func TryLock(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (bool, error) {
	return rdb.SetNX(ctx, fmt.Sprintf("lock:%s", key), lockVal, ttl).Result()
}

// Unlock releases the lock if it belongs to caller.
func Unlock(ctx context.Context, rdb *redis.Client, key string) error {
	return rdb.Del(ctx, fmt.Sprintf("lock:%s", key)).Err()
}

// ─── Session / Token cache ────────────────────────────────────────────────────

func SetToken(ctx context.Context, rdb *redis.Client, userID, token string, ttl time.Duration) error {
	return rdb.Set(ctx, fmt.Sprintf("token:%s", userID), token, ttl).Err()
}

func InvalidateToken(ctx context.Context, rdb *redis.Client, userID string) error {
	return rdb.Del(ctx, fmt.Sprintf("token:%s", userID)).Err()
}

// ─── Brute-force protection ───────────────────────────────────────────────────

func RecordFailedLogin(ctx context.Context, rdb *redis.Client, identifier string) (int64, error) {
	key := fmt.Sprintf("bf:login:%s", identifier)
	return Increment(ctx, rdb, key, 15*time.Minute)
}

func ResetFailedLogin(ctx context.Context, rdb *redis.Client, identifier string) error {
	return rdb.Del(ctx, fmt.Sprintf("bf:login:%s", identifier)).Err()
}

// ─── YAD daily credit cap ─────────────────────────────────────────────────────

func GetDailyYADCredit(ctx context.Context, rdb *redis.Client, userID string) (int64, error) {
	key := fmt.Sprintf("yad:daily:%s", userID)
	val, err := rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func AddDailyYADCredit(ctx context.Context, rdb *redis.Client, userID string, amount int64) error {
	key := fmt.Sprintf("yad:daily:%s", userID)
	pipe := rdb.Pipeline()
	pipe.IncrBy(ctx, key, amount)
	// Expires at end of current day
	now := time.Now()
	eod := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	pipe.ExpireAt(ctx, key, eod)
	_, err := pipe.Exec(ctx)
	return err
}

// ─── Payment dedup ────────────────────────────────────────────────────────────

func MarkPaymentQueued(ctx context.Context, rdb *redis.Client, transactionID string) (bool, error) {
	return SetNX(ctx, rdb, fmt.Sprintf("pay:queued:%s", transactionID), 24*time.Hour)
}
