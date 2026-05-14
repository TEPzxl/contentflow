package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

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
	Enabled     bool     `mapstructure:"enabled"`
	Brokers     []string `mapstructure:"brokers"`
	GroupID     string   `mapstructure:"group_id"`
	MaxAttempts int      `mapstructure:"max_attempts"`
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

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("CONTENTFLOW")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	var cfg Config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "contentflow")
	v.SetDefault("app.env", "dev")

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
	v.SetDefault("database.user", "contentflow")
	v.SetDefault("database.password", "contentflow")
	v.SetDefault("database.dbname", "contentflow")
	v.SetDefault("database.ssl_mode", "disable")
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
}
