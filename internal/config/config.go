package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App      AppConfig
	DB       DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Platega  PlategalConfig
	Remna    RemnaConfig
	Telegram TelegramConfig
}

type AppConfig struct {
	Port        string
	Env         string
	WebhookPath string // e.g. /webhooks/platega
	// AdminBootstrapToken: when a user registers and supplies a `bootstrap_token`
	// that matches this value AND no admin currently exists in the database,
	// the new user is granted is_admin = true. Set during initial deploy and
	// rotated/cleared after the first admin is created. Empty disables the
	// mechanism entirely (use SQL to promote subsequent admins).
	//
	// This replaces the prior ADMIN_LOGIN auto-promote, which was a bootstrap
	// race vector (any first registration of a publicly-known username became
	// admin).
	AdminBootstrapToken string
	AllowedOrigins      string // comma-separated allowed CORS origins
	// AllowedReturnHosts limits which Host values are accepted from the client
	// in `return_url` / `failed_url` fields when initiating Platega payments.
	// Comma-separated, no scheme. Empty falls back to {DOMAIN, www.DOMAIN}.
	// Prevents an open-redirect / phishing vector via the payment redirect.
	AllowedReturnHosts string
}

type DBConfig struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret         string
	AccessTTLHours int
	RefreshTTLDays int
}

type PlategalConfig struct {
	MerchantID  string
	Secret      string
	// SecretPrev is the previous Platega secret kept valid during a rotation
	// window (24–48 h). Empty disables fallback verification. This makes
	// emergency secret rotation possible without losing in-flight webhooks.
	SecretPrev  string
	BaseURL     string
	CallbackURL string // our public HTTPS webhook
}

type RemnaConfig struct {
	BaseURL   string
	APIKey    string
	SquadUUID string // optional: assign new users to this internal squad UUID
}

type TelegramConfig struct {
	Token       string
	BotUsername string // e.g. "mellowpn_bot" (without @)
	WebAppURL   string // e.g. "https://mellowpn.space"
}

func Load() *Config {
	// L-1: Fail fast on an absent or placeholder JWT secret.
	// The docker-compose already enforces this at container start-up via the
	// :? modifier, but we double-check here so the binary also refuses to boot
	// if run outside Docker without a proper secret.
	jwtSecret := env("JWT_SECRET", "")
	if jwtSecret == "" || jwtSecret == "change-me-in-production" || len(jwtSecret) < 32 {
		panic("JWT_SECRET env var must be set to a strong random secret (minimum 32 characters)")
	}

	appEnv := env("APP_ENV", "production")
	allowedOrigins := env("ALLOWED_ORIGINS", "*")

	// Refuse to start a production server with permissive CORS. Echoing the
	// request Origin and setting Access-Control-Allow-Credentials = true is
	// effectively wildcard-with-credentials, a textbook cross-origin data
	// exfiltration setup. In dev, '*' is fine.
	if appEnv == "production" {
		if strings.TrimSpace(allowedOrigins) == "" || strings.Contains(allowedOrigins, "*") {
			panic("ALLOWED_ORIGINS must list explicit origins in production (no '*')")
		}
	}

	// Clamp access-token TTL to a sane range. The previous default of 24 h
	// kept a stolen access token alive for an entire day; with refresh
	// rotation, 1 h is plenty.
	accessTTLHours := envInt("JWT_ACCESS_TTL_HOURS", 1)
	if accessTTLHours < 1 {
		accessTTLHours = 1
	}
	if accessTTLHours > 24 {
		accessTTLHours = 24
	}

	bootstrapToken := env("ADMIN_BOOTSTRAP_TOKEN", "")
	if bootstrapToken != "" && len(bootstrapToken) < 24 {
		panic("ADMIN_BOOTSTRAP_TOKEN must be at least 24 characters when set")
	}

	return &Config{
		App: AppConfig{
			Port:                env("APP_PORT", "8080"),
			Env:                 appEnv,
			WebhookPath:         env("PLATEGA_WEBHOOK_PATH", "/webhooks/platega"),
			AdminBootstrapToken: bootstrapToken,
			AllowedOrigins:      allowedOrigins,
			AllowedReturnHosts:  env("ALLOWED_RETURN_HOSTS", ""),
		},
		DB: DBConfig{
			DSN:          env("DATABASE_DSN", "postgres://vpn:vpn@postgres:5432/vpnplatform?sslmode=prefer"),
			MaxOpenConns: envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: envInt("DB_MAX_IDLE_CONNS", 10),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "redis:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:         jwtSecret,
			AccessTTLHours: accessTTLHours,
			RefreshTTLDays: envInt("JWT_REFRESH_TTL_DAYS", 30),
		},
		Platega: PlategalConfig{
			MerchantID:  env("PLATEGA_MERCHANT_ID", ""),
			Secret:      env("PLATEGA_SECRET", ""),
			SecretPrev:  env("PLATEGA_SECRET_PREV", ""),
			BaseURL:     env("PLATEGA_BASE_URL", "https://app.platega.io"),
			CallbackURL: env("PLATEGA_CALLBACK_URL", ""),
		},
		Remna: RemnaConfig{
			BaseURL:   env("REMNA_BASE_URL", ""),
			APIKey:    env("REMNA_API_KEY", ""),
			SquadUUID: env("REMNA_SQUAD_UUID", ""),
		},
		Telegram: TelegramConfig{
			Token:       env("TELEGRAM_TOKEN", ""),
			BotUsername: env("TELEGRAM_BOT_USERNAME", ""),
			WebAppURL:   env("WEBAPP_URL", "https://mellowpn.space"),
		},
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
