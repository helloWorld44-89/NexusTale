package config

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

// ValidateProd returns an error if any security-critical defaults are still set
// when running in release mode. Call this from main after loading config.
func (c *Config) ValidateProd() error {
	if c.Server.Mode != "release" {
		return nil
	}
	var errs []string
	if c.Auth.JWTSecret == "change-me-in-production" {
		errs = append(errs, "NEXUSTALE_AUTH_JWTSECRET is still the default — rotate to a ≥32-char random value")
	}
	if c.Encryption.Key == "0000000000000000000000000000000000000000000000000000000000000000" {
		errs = append(errs, "NEXUSTALE_ENCRYPTION_KEY is still the all-zeros default — rotate to a random 64-hex-char value")
	}
	if c.Minio.AccessKey == "minioadmin" || c.Minio.SecretKey == "minioadmin" {
		errs = append(errs, "MinIO credentials are still the default minioadmin — change before going live")
	}
	if c.Server.AllowedOrigin == "*" {
		errs = append(errs, "NEXUSTALE_SERVER_ALLOWEDORIGIN is '*' in release mode — set it to your app domain")
	}
	if len(errs) > 0 {
		return errors.New("insecure defaults detected in release mode:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

type Config struct {
	Server     ServerConfig
	DB         DBConfig
	Auth       AuthConfig
	Encryption EncryptionConfig
	Redis      RedisConfig
	Minio      MinioConfig
	Git        GitConfig
	AI         AIConfig
}

type AIConfig struct {
	OllamaURL     string
	OllamaModel   string
	MaxTokens     int
	BeatMaxTokens int
}

type ServerConfig struct {
	Port             string
	Mode             string // "debug", "release", "test"
	AllowedOrigin    string // CORS allowed origin; defaults to "*" in dev, must be set in prod
	RegistrationOpen bool   // when false, POST /register returns 403 (invite-only alpha)
}

type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MigrationsPath  string
}

type AuthConfig struct {
	JWTSecret          string
	AccessTokenExpiry  string // duration string e.g. "15m"
	RefreshTokenExpiry string // duration string e.g. "7d"
	BcryptCost         int
}

type RedisConfig struct {
	URL string
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type GitConfig struct {
	ReposPath string
}

// EncryptionConfig holds the server-side key used for AES-GCM encryption of
// user-provided API keys. Key must be exactly 32 bytes (64 hex chars).
// NEXUSTALE_ENCRYPTION_KEY env var — change from default in any real deploy.
type EncryptionConfig struct {
	Key string // hex-encoded 32-byte key
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("NEXUSTALE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.allowedorigin", "*")
	v.SetDefault("server.registrationopen", true)
	v.SetDefault("db.url", "postgres://nexustale:nexustale@localhost:5432/nexustale?sslmode=disable")
	v.SetDefault("db.maxconns", 10)
	v.SetDefault("db.minconns", 2)
	v.SetDefault("db.migrationspath", "file://pkg/db/migrations")
	v.SetDefault("auth.jwtsecret", "change-me-in-production")
	v.SetDefault("auth.accesstokenexpiry", "8h")
	v.SetDefault("auth.refreshtokenexpiry", "168h") // 7 days
	v.SetDefault("auth.bcryptcost", 12)
	v.SetDefault("redis.url", "redis://localhost:6379")
	v.SetDefault("minio.endpoint", "localhost:9000")
	v.SetDefault("minio.accesskey", "minioadmin")
	v.SetDefault("minio.secretkey", "minioadmin")
	v.SetDefault("minio.bucket", "nexustale")
	v.SetDefault("minio.usessl", false)
	v.SetDefault("git.repospath", "./data/repos")
	// 32-byte dev default — must be overridden in production
	v.SetDefault("encryption.key", "0000000000000000000000000000000000000000000000000000000000000000")
	v.SetDefault("ai.ollamaurl", "http://localhost:11434")
	v.SetDefault("ai.ollamamodel", "llama3.2")
	v.SetDefault("ai.maxtokens", 2048)
	v.SetDefault("ai.beatmaxtokens", 600)

	// Try config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./backend")
	_ = v.ReadInConfig() // optional

	cfg := &Config{
		Server: ServerConfig{
			Port:             v.GetString("server.port"),
			Mode:             v.GetString("server.mode"),
			AllowedOrigin:    v.GetString("server.allowedorigin"),
			RegistrationOpen: v.GetBool("server.registrationopen"),
		},
		DB: DBConfig{
			URL:            v.GetString("db.url"),
			MaxConns:       v.GetInt32("db.maxconns"),
			MinConns:       v.GetInt32("db.minconns"),
			MigrationsPath: v.GetString("db.migrationspath"),
		},
		Auth: AuthConfig{
			JWTSecret:          v.GetString("auth.jwtsecret"),
			AccessTokenExpiry:  v.GetString("auth.accesstokenexpiry"),
			RefreshTokenExpiry: v.GetString("auth.refreshtokenexpiry"),
			BcryptCost:         v.GetInt("auth.bcryptcost"),
		},
		Redis: RedisConfig{
			URL: v.GetString("redis.url"),
		},
		Minio: MinioConfig{
			Endpoint:  v.GetString("minio.endpoint"),
			AccessKey: v.GetString("minio.accesskey"),
			SecretKey: v.GetString("minio.secretkey"),
			Bucket:    v.GetString("minio.bucket"),
			UseSSL:    v.GetBool("minio.usessl"),
		},
		Encryption: EncryptionConfig{
			Key: v.GetString("encryption.key"),
		},
		Git: GitConfig{
			ReposPath: v.GetString("git.repospath"),
		},
		AI: AIConfig{
			OllamaURL:     v.GetString("ai.ollamaurl"),
			OllamaModel:   v.GetString("ai.ollamamodel"),
			MaxTokens:     v.GetInt("ai.maxtokens"),
			BeatMaxTokens: v.GetInt("ai.beatmaxtokens"),
		},
	}

	return cfg, nil
}
