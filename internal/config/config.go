package config

import (
	"os"
	"strconv"
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
	Port           string
	Env            string
	WebhookPath    string // e.g. /webhooks/platega
	AdminLogin     string // login (username) that is auto-granted admin on register/login
	AllowedOrigins string // comma-separated allowed CORS origins
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
	AdminID     int64
	BotUsername string // e.g. "mellowpn_bot" (without @)
	WebAppURL   string // e.g. "https://mellowpn.space"
}

func Load() *Config {
	return &Config{
		App: AppConfig{
			Port:           env("APP_PORT", "8080"),
			Env:            env("APP_ENV", "production"),
			WebhookPath:    env("PLATEGA_WEBHOOK_PATH", "/webhooks/platega"),
			AdminLogin:     env("ADMIN_LOGIN", ""),
			AllowedOrigins: env("ALLOWED_ORIGINS", "*"),
		},
		DB: DBConfig{
			DSN:          env("DATABASE_DSN", "postgres://vpn:vpn@postgres:5432/vpnplatform?sslmode=disable"),
			MaxOpenConns: envInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: envInt("DB_MAX_IDLE_CONNS", 10),
		},
		Redis: RedisConfig{
			Addr:     env("REDIS_ADDR", "redis:6379"),
			Password: env("REDIS_PASSWORD", ""),
			DB:       envInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:         env("JWT_SECRET", "change-me-in-production"),
			AccessTTLHours: envInt("JWT_ACCESS_TTL_HOURS", 24),
			RefreshTTLDays: envInt("JWT_REFRESH_TTL_DAYS", 30),
		},
		Platega: PlategalConfig{
			MerchantID:  env("PLATEGA_MERCHANT_ID", ""),
			Secret:      env("PLATEGA_SECRET", ""),
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
			AdminID:     int64(envInt("TELEGRAM_ADMIN_ID", 0)),
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
