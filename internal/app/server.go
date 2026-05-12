package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/cache"
	"github.com/tepzxl/contentflow/internal/config"
	"github.com/tepzxl/contentflow/internal/database"
	contenthttp "github.com/tepzxl/contentflow/internal/http"
	"github.com/tepzxl/contentflow/internal/logger"
	"github.com/tepzxl/contentflow/internal/module/auth"
	"github.com/tepzxl/contentflow/internal/module/user"
)

func Run() error {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(logger.Config{
		Level:     cfg.Log.Level,
		Format:    cfg.Log.Format,
		AddSource: cfg.Log.AddSource,
	})
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	slog.SetDefault(log)

	ctx := context.Background()
	db, err := database.NewPostgres(ctx, database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		DBName:          cfg.Database.DBName,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	defer sqlDB.Close()
	log.Info("postgres connected",
		slog.String("host", cfg.Database.Host),
		slog.Int("port", cfg.Database.Port),
		slog.String("dbname", cfg.Database.DBName),
	)

	redisClient, err := cache.NewRedis(ctx, cache.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer redisClient.Close()

	log.Info("redis connected",
		slog.String("addr", cfg.Redis.Addr),
		slog.Int("db", cfg.Redis.DB),
	)

	userRepo := user.NewGormRepository(db)
	refreshTokenRepo := auth.NewGormRefreshTokenRepository(db)

	tokenManager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
		Secret:          cfg.Auth.JWTSecret,
		Issuer:          cfg.Auth.JWTIssuer,
		AccessTokenTTL:  cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
	})
	if err != nil {
		return fmt.Errorf("init token manager: %w", err)
	}
	authService := auth.NewAuthService(userRepo, refreshTokenRepo, tokenManager)
	authHandler := auth.NewHandler(authService)

	router := contenthttp.NewRouter(log, db, redisClient, func(api *gin.RouterGroup) {
		auth.RegisterRoutes(api, authHandler)
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Println("contentflow server started on :8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1) // 接受退出信号, 容量为1避免阻塞
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-quit:
		fmt.Println("contentflow server stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil { // 优雅退出
		return fmt.Errorf("shutdown server: %w", err)
	}

	return nil
}
