// Package config loads runtime configuration from config.yml (optional) with environment
// overrides. .env is loaded into the process environment first (12-factor friendly).
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App           App           `yaml:"app"`
	HTTP          HTTP          `yaml:"http"`
	Database      Database      `yaml:"database"`
	Auth          Auth          `yaml:"auth"`
	Telegram      Telegram      `yaml:"telegram"`
	Gemini        Gemini        `yaml:"gemini"`
	Observability Observability `yaml:"observability"`
	Superadmin    Superadmin    `yaml:"superadmin"`
}

type App struct {
	Name    string `yaml:"name" env:"APP_NAME" env-default:"student-success"`
	Env     string `yaml:"environment" env:"APP_ENV" env-default:"development"`
	Version string `yaml:"version" env:"APP_VERSION" env-default:"0.1.0"`
}

type HTTP struct {
	Host        string   `yaml:"host" env:"HTTP_HOST" env-default:"0.0.0.0"`
	Port        string   `yaml:"port" env:"HTTP_PORT" env-default:"8080"`
	CORSOrigins []string `yaml:"cors_origins" env:"CORS_ORIGINS" env-separator:"," env-default:"*"`
}

type Database struct {
	URL string `yaml:"dsn" env:"DATABASE_URL" env-required:"true"`
}

type Auth struct {
	Issuer     string        `yaml:"issuer" env:"JWT_ISSUER" env-default:"student-success"`
	SigningKey string        `yaml:"signing_key" env:"JWT_SECRET" env-required:"true"`
	AccessTTL  time.Duration `yaml:"access_ttl" env:"JWT_ACCESS_TTL" env-default:"15m"`
	RefreshTTL time.Duration `yaml:"refresh_ttl" env:"JWT_REFRESH_TTL" env-default:"720h"`
}

type Telegram struct {
	BotToken      string `yaml:"bot_token" env:"TELEGRAM_BOT_TOKEN"`
	WebhookSecret string `yaml:"webhook_secret" env:"TELEGRAM_WEBHOOK_SECRET"`
}

type Gemini struct {
	APIKey string `yaml:"api_key" env:"GEMINI_API_KEY"`
	// Model is the Gemini model id. Default is a current, fast model that has quota on this
	// project (the free-tier gemini-2.0-flash returned quota=0; 2.5-flash works).
	Model string `yaml:"model" env:"GEMINI_MODEL" env-default:"gemini-2.5-flash"`
}

type Observability struct {
	LogLevel string `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
}

// Superadmin seeds the platform owner account on boot (idempotent). Leave Email/Password blank
// to skip bootstrapping. The super_admin lives in a reserved "platform" org and logs in via
// /admin/login (no center slug).
type Superadmin struct {
	Email    string `yaml:"email" env:"SUPERADMIN_EMAIL"`
	Password string `yaml:"password" env:"SUPERADMIN_PASSWORD"`
	FullName string `yaml:"full_name" env:"SUPERADMIN_FULL_NAME" env-default:"Super Admin"`
}

// IsDev reports whether the app is running in a local/dev environment.
func (a App) IsDev() bool { return a.Env == "development" || a.Env == "local" }

func DefaultConfigPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return "config.yml"
}

func DefaultEnvPath() string {
	if p := os.Getenv("ENV_FILE"); p != "" {
		return p
	}
	return ".env"
}

// Load reads .env into the environment (optional), then the YAML config file if it exists
// (with env overrides), else env-only. So `go run` on the host now picks up .env without
// any Makefile shimming.
func Load(path string) (*Config, error) {
	_ = godotenv.Load(DefaultEnvPath()) // optional; missing file is fine

	var cfg Config
	loaded := false
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := cleanenv.ReadConfig(path, &cfg); err != nil {
				return nil, err
			}
			loaded = true
		}
	}
	if !loaded {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, err
		}
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Auth.SigningKey) < 32 {
		return fmt.Errorf("config: JWT_SECRET must be at least 32 characters (got %d) — generate one with `openssl rand -base64 48`", len(c.Auth.SigningKey))
	}
	return nil
}
