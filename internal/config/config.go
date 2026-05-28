package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const EnvConfigPath = "CONTENTFLOW_CONFIG"

type Config struct {
	App           AppConfig           `mapstructure:"app"`
	Server        ServerConfig        `mapstructure:"server"`
	Log           LogConfig           `mapstructure:"log"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Kafka         KafkaConfig         `mapstructure:"kafka"`
	Auth          AuthConfig          `mapstructure:"auth"`
	RateLimit     RateLimitConfig     `mapstructure:"rate_limit"`
	Cache         CacheConfig         `mapstructure:"cache"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	AI            AIConfig            `mapstructure:"ai"`
}

type AIConfig struct {
	Provider       string        `mapstructure:"provider"`
	BaseURL        string        `mapstructure:"base_url"`
	APIKey         string        `mapstructure:"api_key"`
	Model          string        `mapstructure:"model"`
	EmbeddingModel string        `mapstructure:"embedding_model"`
	Timeout        time.Duration `mapstructure:"timeout"`
}

type ObservabilityConfig struct {
	MetricsEnabled bool   `mapstructure:"metrics_enabled"`
	TracingEnabled bool   `mapstructure:"tracing_enabled"`
	ServiceName    string `mapstructure:"service_name"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint"`
}

type CacheConfig struct {
	SourceListTTL  time.Duration `mapstructure:"source_list_ttl"`
	ArticleListTTL time.Duration `mapstructure:"article_list_ttl"`
}

type RateLimitConfig struct {
	LoginLimit    int           `mapstructure:"login_limit"`
	LoginWindow   time.Duration `mapstructure:"login_window"`
	CollectLimit  int           `mapstructure:"collect_limit"`
	CollectWindow time.Duration `mapstructure:"collect_window"`
}

type KafkaConfig struct {
	Enabled                bool          `mapstructure:"enabled"`
	Brokers                []string      `mapstructure:"brokers"`
	GroupID                string        `mapstructure:"group_id"`
	MaxAttempts            int           `mapstructure:"max_attempts"`
	RetryBackoff           time.Duration `mapstructure:"retry_backoff"`
	OutboxBatchSize        int           `mapstructure:"outbox_batch_size"`
	OutboxDispatchInterval time.Duration `mapstructure:"outbox_dispatch_interval"`
}

type AuthConfig struct {
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	JWTSecret       string        `mapstructure:"jwt_secret"`
	JWTIssuer       string        `mapstructure:"jwt_issuer"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
	Mode string `mapstructure:"mode"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

type LogConfig struct {
	Level     string `mapstructure:"level"`
	Format    string `mapstructure:"format"`
	AddSource bool   `mapstructure:"add_source"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

func PathFromEnv(defaultPath string) string {
	path := strings.TrimSpace(os.Getenv(EnvConfigPath))
	if path == "" {
		return defaultPath
	}
	return path
}

func Load(path string) (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}

	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("CONTENTFLOW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)
	applyLegacyAIEnv(v)

	var cfg Config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateSecurity(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "contentflow")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.mode", "all")

	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("server.idle_timeout", "60s")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.add_source", false)

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.username", "contentflow")
	v.SetDefault("database.password", "contentflow")
	v.SetDefault("database.dbname", "contentflow")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "30m")

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	v.SetDefault("kafka.enabled", false)
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.group_id", "contentflow-collection-worker")
	v.SetDefault("kafka.max_attempts", 3)
	v.SetDefault("kafka.retry_backoff", "1m")
	v.SetDefault("kafka.outbox_batch_size", 100)
	v.SetDefault("kafka.outbox_dispatch_interval", "1s")

	v.SetDefault("cache.source_list_ttl", "30s")
	v.SetDefault("cache.article_list_ttl", "30s")

	v.SetDefault("rate_limit.login_limit", 5)
	v.SetDefault("rate_limit.login_window", "1m")
	v.SetDefault("rate_limit.collect_limit", 10)
	v.SetDefault("rate_limit.collect_window", "1m")

	v.SetDefault("observability.metrics_enabled", true)
	v.SetDefault("observability.tracing_enabled", false)
	v.SetDefault("observability.service_name", "contentflow")
	v.SetDefault("observability.otlp_endpoint", "localhost:4317")

	v.SetDefault("auth.access_token_ttl", "15m")
	v.SetDefault("auth.refresh_token_ttl", "168h")
	v.SetDefault("auth.jwt_secret", "default secret")
	v.SetDefault("auth.jwt_issuer", "contentflow")

	v.SetDefault("ai.provider", "local")
	v.SetDefault("ai.base_url", "https://api.openai.com/v1")
	v.SetDefault("ai.api_key", "")
	v.SetDefault("ai.model", "")
	v.SetDefault("ai.embedding_model", "text-embedding-3-small")
	v.SetDefault("ai.timeout", "30s")
}

func applyLegacyAIEnv(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("base_url")); value != "" {
		v.Set("ai.base_url", value)
	}
	if value := strings.TrimSpace(os.Getenv("API_KEY")); value != "" {
		v.Set("ai.api_key", value)
		if strings.TrimSpace(v.GetString("ai.provider")) == "local" {
			v.Set("ai.provider", "openai")
		}
	}
	if value := strings.TrimSpace(os.Getenv("model")); value != "" {
		v.Set("ai.model", value)
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open .env: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set .env key %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .env: %w", err)
	}
	return nil
}

func validateSecurity(cfg Config) error {
	env := strings.ToLower(strings.TrimSpace(cfg.App.Env))
	if env == "" || env == "dev" || env == "development" || env == "test" {
		return nil
	}

	secret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if len(secret) < 32 || isWeakJWTSecret(secret) {
		return fmt.Errorf("weak jwt secret is not allowed outside development")
	}
	return nil
}

func isWeakJWTSecret(secret string) bool {
	switch strings.ToLower(strings.TrimSpace(secret)) {
	case "", "default secret", "replace-me", "change-me", "change-me-before-production", "dev-only-insecure-jwt-secret-change-me":
		return true
	default:
		return false
	}
}
