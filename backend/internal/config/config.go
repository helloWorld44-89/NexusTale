package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig
	DB         DBConfig
	Auth       AuthConfig
	Encryption EncryptionConfig
	Redis      RedisConfig
	Minio      MinioConfig
	Git        GitConfig
}

type ServerConfig struct {
	Port string
	Mode string // "debug", "release", "test"
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
	v.SetDefault("db.url", "postgres://nexustale:nexustale@localhost:5432/nexustale?sslmode=disable")
	v.SetDefault("db.maxconns", 10)
	v.SetDefault("db.minconns", 2)
	v.SetDefault("db.migrationspath", "file://pkg/db/migrations")
	v.SetDefault("auth.jwtsecret", "change-me-in-production")
	v.SetDefault("auth.accesstokenexpiry", "15m")
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

	// Try config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./backend")
	_ = v.ReadInConfig() // optional

	cfg := &Config{
		Server: ServerConfig{
			Port: v.GetString("server.port"),
			Mode: v.GetString("server.mode"),
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
	}

	return cfg, nil
}
