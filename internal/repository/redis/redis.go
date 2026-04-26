package redisrepo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

// incrementScript sets TTL only on first creation, preventing sliding-window bypass (H-1).
var incrementScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[1]))
end
return c`)

// Increment returns (current count, error). TTL is fixed from the first call — not sliding.
func Increment(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (int64, error) {
	result, err := incrementScript.Run(ctx, rdb, []string{key}, int(ttl.Seconds())).Int64()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// ─── Idempotency / Deduplication ─────────────────────────────────────────────

// SetNX returns true if the key was set (first time), false if already exists.
// Keys set via SetNX always carry a TTL so volatile-lru can evict them if needed,
// and they auto-clean after expiry.
func SetNX(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (bool, error) {
	return rdb.SetNX(ctx, key, 1, ttl).Result()
}

// ─── Distributed Lock (token-guarded) ────────────────────────────────────────

// unlockScript deletes the lock only if the stored value matches the caller's token,
// preventing "lock theft" when the TTL expires while the holder is still working.
var unlockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end`)

// TryLock attempts to acquire a lock with a unique token. Returns (token, true) on success.
// The caller must pass the token to Unlock to safely release only their own lock.
func TryLock(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (string, bool, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", false, err
	}
	token := hex.EncodeToString(b)
	ok, err := rdb.SetNX(ctx, fmt.Sprintf("lock:%s", key), token, ttl).Result()
	return token, ok, err
}

// Unlock releases the lock only if our token matches (prevents releasing another holder's lock).
func Unlock(ctx context.Context, rdb *redis.Client, key, token string) error {
	return unlockScript.Run(ctx, rdb, []string{fmt.Sprintf("lock:%s", key)}, token).Err()
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

// GetFailedLoginCount returns the current brute-force counter without incrementing it.
func GetFailedLoginCount(ctx context.Context, rdb *redis.Client, identifier string) (int64, error) {
	key := fmt.Sprintf("bf:login:%s", identifier)
	val, err := rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func ResetFailedLogin(ctx context.Context, rdb *redis.Client, identifier string) error {
	return rdb.Del(ctx, fmt.Sprintf("bf:login:%s", identifier)).Err()
}

// ─── YAD daily credit cap (atomic, H-2) ──────────────────────────────────────

// yadCapScript atomically checks the daily cap and increments if allowed.
// Returns the new total, or -1 if the cap would be exceeded.
var yadCapScript = redis.NewScript(`
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local cap = tonumber(ARGV[2])
local expire_at = tonumber(ARGV[3])
local current = tonumber(redis.call('GET', key) or 0)
if current + amount > cap then
    return -1
end
local new = redis.call('INCRBY', key, amount)
if new == amount then
    redis.call('EXPIREAT', key, expire_at)
end
return new`)

// CheckAndAddDailyYADCreditAtomic atomically checks the daily cap and adds amount.
// Returns an error if the cap would be exceeded. Uses a fixed TTL until end-of-day.
func CheckAndAddDailyYADCreditAtomic(ctx context.Context, rdb *redis.Client, userID string, amount, cap int64) error {
	key := fmt.Sprintf("yad:daily:%s", userID)
	now := time.Now()
	eod := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	result, err := yadCapScript.Run(ctx, rdb, []string{key}, amount, cap, eod.Unix()).Int64()
	if err != nil {
		return nil // fail open on Redis error
	}
	if result == -1 {
		return errors.New("превышен дневной лимит начисления ЯД")
	}
	return nil
}

// GetDailyYADCredit returns the current daily credit for a user (read-only).
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
	now := time.Now()
	eod := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	pipe.ExpireAt(ctx, key, eod)
	_, err := pipe.Exec(ctx)
	return err
}

// ─── Refresh token allowlist (H-8) ───────────────────────────────────────────

// RegisterRefreshToken stores a refresh token JTI in the allowlist and adds
// it to the per-user refresh-family set. Must be called when issuing a new
// refresh token.
//
// Two keys are written:
//   - rt:<jti>          → userID, TTL = refresh lifetime
//   - rt:user:<userID>  → SET of jti, TTL = refresh lifetime + slack
//
// The set lets us blow the entire family on password change or on a
// reuse-detected event without scanning all rt:* keys.
func RegisterRefreshToken(ctx context.Context, rdb *redis.Client, jti, userID string, ttl time.Duration) error {
	pipe := rdb.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("rt:%s", jti), userID, ttl)
	pipe.SAdd(ctx, fmt.Sprintf("rt:user:%s", userID), jti)
	// Bump the set TTL to the longest remaining child TTL + slack.
	pipe.Expire(ctx, fmt.Sprintf("rt:user:%s", userID), ttl+time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// ValidateAndRevokeRefreshToken atomically validates and consumes a refresh
// JTI. Returns:
//   - (userID, nil)              — token valid; consumed; caller may issue new tokens
//   - ("", ErrRefreshNotFound)   — token absent; either expired naturally OR replayed
//
// On replay, the caller should treat it as a possible theft event and revoke
// the entire refresh-family for that user; this repo exposes the userID via
// the JWT claims, not via the rt: key (it's already gone), so the caller
// must do the parse + RevokeAllRefreshTokens step.
func ValidateAndRevokeRefreshToken(ctx context.Context, rdb *redis.Client, jti string) (string, error) {
	key := fmt.Sprintf("rt:%s", jti)
	val, err := rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrRefreshNotFound
	}
	if err != nil {
		return "", err
	}
	// Best-effort: drop the JTI from the user's set so SCARD stays accurate.
	if val != "" {
		_ = rdb.SRem(ctx, fmt.Sprintf("rt:user:%s", val), jti).Err()
	}
	return val, nil
}

// ErrRefreshNotFound is returned when a refresh JTI isn't in the allowlist —
// either because it expired, was already used (rotation), or because someone
// replayed an already-rotated token (compromise indicator).
var ErrRefreshNotFound = errors.New("refresh token revoked or expired")

// RevokeRefreshToken deletes a refresh token from the allowlist (for logout / ban).
func RevokeRefreshToken(ctx context.Context, rdb *redis.Client, jti string) error {
	return rdb.Del(ctx, fmt.Sprintf("rt:%s", jti)).Err()
}

// RevokeAllUserRefreshTokens drops every refresh JTI in the user's family
// set, plus the set itself. Called on:
//   - password change (security: stolen refresh stops working)
//   - refresh-token reuse detection (theft response)
//   - explicit "log out everywhere" UI action
func RevokeAllUserRefreshTokens(ctx context.Context, rdb *redis.Client, userID string) error {
	setKey := fmt.Sprintf("rt:user:%s", userID)
	jtis, err := rdb.SMembers(ctx, setKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := rdb.Pipeline()
	for _, jti := range jtis {
		pipe.Del(ctx, fmt.Sprintf("rt:%s", jti))
	}
	pipe.Del(ctx, setKey)
	_, err = pipe.Exec(ctx)
	return err
}

// ─── Payment dedup ────────────────────────────────────────────────────────────

func MarkPaymentQueued(ctx context.Context, rdb *redis.Client, transactionID string) (bool, error) {
	return SetNX(ctx, rdb, fmt.Sprintf("pay:queued:%s", transactionID), 24*time.Hour)
}

// ─── Password version (session invalidation on password change) ───────────────

// SetPasswordVersion records the current Unix timestamp under `pw:ver:{userID}`.
// Any access token whose IssuedAt precedes this timestamp is considered stale.
// TTL matches the maximum refresh-token lifetime so the key auto-expires cleanly.
func SetPasswordVersion(ctx context.Context, rdb *redis.Client, userID string, t time.Time) error {
	key := fmt.Sprintf("pw:ver:%s", userID)
	return rdb.Set(ctx, key, t.Unix(), 30*24*time.Hour).Err()
}

// CheckPasswordVersion returns an error when the stored password-change timestamp
// is newer than the JWT's IssuedAt. Returns nil (allow) on Redis errors so a
// transient outage never locks out all users.
func CheckPasswordVersion(ctx context.Context, rdb *redis.Client, userID string, tokenIssuedAt time.Time) error {
	key := fmt.Sprintf("pw:ver:%s", userID)
	changedAt, err := rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return nil // no password change recorded — token is valid
	}
	if err != nil {
		return nil // fail open on Redis error
	}
	if tokenIssuedAt.Unix() < changedAt {
		return errors.New("token issued before last password change")
	}
	return nil
}

// ─── Two-Factor Authentication challenges ─────────────────────────────────────

const (
	TFAPending  = "pending"
	TFAApproved = "approved"
	TFADenied   = "denied"
)

// Create2FAChallenge stores a new 2FA challenge in Redis with a 5-minute TTL.
func Create2FAChallenge(ctx context.Context, rdb *redis.Client, challengeID, userID string) error {
	key := fmt.Sprintf("tfa:%s", challengeID)
	return rdb.Set(ctx, key, userID+":"+TFAPending, 5*time.Minute).Err()
}

// Get2FAChallenge returns (userID, status, error). Returns ("", "", nil) if not found.
func Get2FAChallenge(ctx context.Context, rdb *redis.Client, challengeID string) (string, string, error) {
	key := fmt.Sprintf("tfa:%s", challengeID)
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	// val = "userID:status"
	parts := splitTFAValue(val)
	return parts[0], parts[1], nil
}

// resolveTFAScript atomically reads the current value, preserves the userID,
// and writes the new status — closes the GET/SET race that two near-simultaneous
// /approve and /deny callbacks could otherwise produce.
var resolveTFAScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if not v then return 0 end
local sep = string.find(v, ':[^:]*$')
if not sep then return 0 end
local uid = string.sub(v, 1, sep-1)
redis.call('SET', KEYS[1], uid .. ':' .. ARGV[1], 'EX', tonumber(ARGV[2]))
return 1`)

// Resolve2FAChallenge atomically updates the status to approved/denied.
func Resolve2FAChallenge(ctx context.Context, rdb *redis.Client, challengeID, status string) error {
	key := fmt.Sprintf("tfa:%s", challengeID)
	_, err := resolveTFAScript.Run(ctx, rdb, []string{key}, status, int((2 * time.Minute).Seconds())).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}

func splitTFAValue(val string) [2]string {
	for i := len(val) - 1; i >= 0; i-- {
		if val[i] == ':' {
			return [2]string{val[:i], val[i+1:]}
		}
	}
	return [2]string{val, TFAPending}
}
