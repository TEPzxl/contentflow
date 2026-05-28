package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/cache"
	"github.com/tepzxl/contentflow/internal/config"
	"github.com/tepzxl/contentflow/internal/database"
	contenthttp "github.com/tepzxl/contentflow/internal/http"
	"github.com/tepzxl/contentflow/internal/http/middleware"
	"github.com/tepzxl/contentflow/internal/logger"
	"github.com/tepzxl/contentflow/internal/module/ai"
	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/module/auth"
	"github.com/tepzxl/contentflow/internal/module/collectionjob"
	"github.com/tepzxl/contentflow/internal/module/collector"
	emailcollector "github.com/tepzxl/contentflow/internal/module/collector/email"
	rsscollector "github.com/tepzxl/contentflow/internal/module/collector/rss"
	"github.com/tepzxl/contentflow/internal/module/scheduler"
	"github.com/tepzxl/contentflow/internal/module/source"
	"github.com/tepzxl/contentflow/internal/module/user"
	"github.com/tepzxl/contentflow/internal/observability"
	"github.com/tepzxl/contentflow/internal/ratelimit"
)

func Run() error {
	cfg, err := config.Load(config.PathFromEnv("configs/config.yaml"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	plan, err := runtimePlanForMode(cfg.App.Mode)
	if err != nil {
		return err
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
	traceShutdown, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:      cfg.Observability.TracingEnabled,
		ServiceName:  cfg.Observability.ServiceName,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
	})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(shutdownCtx); err != nil {
			log.Warn("shutdown tracing failed", slog.String("error", err.Error()))
		}
	}()

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

	var metrics *observability.Metrics
	if cfg.Observability.MetricsEnabled {
		metrics, err = observability.NewDefaultMetrics()
		if err != nil {
			return fmt.Errorf("init metrics: %w", err)
		}
		if err := metrics.RegisterGormCallbacks(db); err != nil {
			return fmt.Errorf("register database metrics: %w", err)
		}
	}

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

	userRepo := user.NewRepository(db)
	refreshTokenRepo := auth.NewRefreshTokenRepository(db)

	tokenManager, err := auth.NewJWTTokenManager(auth.JWTTokenManagerConfig{
		Secret:          cfg.Auth.JWTSecret,
		Issuer:          cfg.Auth.JWTIssuer,
		AccessTokenTTL:  cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
	})
	if err != nil {
		return fmt.Errorf("init token manager: %w", err)
	}
	authService := auth.NewService(userRepo, refreshTokenRepo, tokenManager)
	authHandler := auth.NewHandler(authService)

	authRequired := middleware.AuthRequired(func(token string) (int64, error) {
		claims, err := tokenManager.ParseAccessToken(token)
		if err != nil {
			return 0, err
		}
		return claims.UserID, nil
	})

	redisLimiter := ratelimit.NewRedisLimiter(redisClient)
	loginRateLimit := ratelimit.Middleware(redisLimiter, ratelimit.Config{
		Limit:   cfg.RateLimit.LoginLimit,
		Window:  cfg.RateLimit.LoginWindow,
		KeyFunc: ratelimit.ClientIPKey("ratelimit:login"),
	})
	collectRateLimit := ratelimit.Middleware(redisLimiter, ratelimit.Config{
		Limit:   cfg.RateLimit.CollectLimit,
		Window:  cfg.RateLimit.CollectWindow,
		KeyFunc: ratelimit.UserIDPathKey("ratelimit:collect", "id"),
	})

	sourceRepo := source.NewRepository(db)
	sourceListCache := source.NewRedisListCache(redisClient, "cache:sources")
	sourceService := source.NewService(
		sourceRepo,
		source.WithListCache(sourceListCache, cfg.Cache.SourceListTTL),
	)
	sourceHandler := source.NewHandler(sourceService)

	articleRepo := article.NewRepository(db)
	articleListCache := article.NewRedisListCache(redisClient, "cache:articles")
	articleService := article.NewService(
		articleRepo,
		article.WithListCache(articleListCache, cfg.Cache.ArticleListTTL),
	)
	articleHandler := article.NewHandler(articleService)
	aiRepo := ai.NewRepository(db)
	assistant, err := newConfiguredAssistant(cfg.AI, log)
	if err != nil {
		return err
	}
	aiSecretBox, err := newAISecretBox(cfg.AI.SettingsEncryptionKey)
	if err != nil {
		return err
	}
	aiOptions := []ai.Option{}
	if aiSecretBox != nil {
		aiOptions = append(aiOptions, ai.WithSecretBox(aiSecretBox))
	}
	aiService := ai.NewService(aiRepo, articleRepo, assistant, aiOptions...)
	aiHandler := ai.NewHandler(aiService)
	aiSummaryWorker := ai.NewSummaryWorker(aiService, ai.WithWorkerLogger(log))

	runRepo := collector.NewRunRepository(db)

	collectorRegistry, err := collector.NewRegistry(
		rsscollector.NewCollector(),
		emailcollector.NewCollector(),
	)
	if err != nil {
		return fmt.Errorf("init collector registry: %w", err)
	}

	collectionOptions := []collector.Option{}
	if metrics != nil {
		collectionOptions = append(collectionOptions, collector.WithObserver(metrics))
	}
	collectionOptions = append(collectionOptions, collector.WithLogger(log))
	collectionOptions = append(collectionOptions, collector.WithCollectionLock(collector.NewRedisCollectionLock(redisClient)))
	collectionService := collector.NewService(sourceRepo, runRepo, collectorRegistry, articleService, collectionOptions...)
	collectionHandler := collector.NewHandler(collectionService)

	scheduledCollector := scheduler.CollectionService(collectionService)
	registerCollectionRoutes := func(api *gin.RouterGroup) {
		collector.RegisterRoutes(api, collectionHandler, authRequired, collectRateLimit)
	}

	var kafkaWriter *collectionjob.KafkaWriter
	var kafkaReader *collectionjob.KafkaReader
	var collectionWorker *collectionjob.Worker
	var outboxDispatcher *collectionjob.OutboxDispatcher
	registerCollectionJobRoutes := func(api *gin.RouterGroup) {}

	if cfg.Kafka.Enabled {
		kafkaWriter = collectionjob.NewKafkaWriter(cfg.Kafka.Brokers)
		kafkaReader = collectionjob.NewKafkaReader(cfg.Kafka.Brokers, cfg.Kafka.GroupID, collectionjob.TopicCollectionRequested)

		outboxRepo := collectionjob.NewGormOutboxRepository(db)
		dlqRepo := collectionjob.NewGormDLQRepository(db)
		jobProducer := collectionjob.NewOutboxProducer(outboxRepo)
		outboxOptions := []collectionjob.OutboxDispatcherOption{
			collectionjob.WithOutboxLogger(log),
			collectionjob.WithOutboxBatchSize(cfg.Kafka.OutboxBatchSize),
			collectionjob.WithOutboxBackoff(cfg.Kafka.RetryBackoff),
			collectionjob.WithOutboxInterval(cfg.Kafka.OutboxDispatchInterval),
		}
		if metrics != nil {
			outboxOptions = append(outboxOptions, collectionjob.WithOutboxObserver(metrics))
		}
		outboxDispatcher = collectionjob.NewOutboxDispatcher(outboxRepo, kafkaWriter, outboxOptions...)
		scheduledCollector = jobProducer
		asyncCollectionHandler := collector.NewAsyncHandler(jobProducer)
		dlqHandler := collectionjob.NewDLQHandler(collectionjob.NewDLQService(dlqRepo, kafkaWriter))
		registerCollectionRoutes = func(api *gin.RouterGroup) {
			collector.RegisterAsyncRoutes(api, asyncCollectionHandler, authRequired, collectRateLimit)
			collector.RegisterCollectionRunRoutes(api, collectionHandler, authRequired)
		}
		registerCollectionJobRoutes = func(api *gin.RouterGroup) {
			collectionjob.RegisterDLQRoutes(api, dlqHandler, authRequired)
		}

		workerOptions := []collectionjob.WorkerOption{
			collectionjob.WithMaxAttempts(cfg.Kafka.MaxAttempts),
			collectionjob.WithRetryBackoff(cfg.Kafka.RetryBackoff),
			collectionjob.WithWorkerLogger(log),
			collectionjob.WithDLQRepository(dlqRepo),
		}
		if metrics != nil {
			workerOptions = append(workerOptions, collectionjob.WithJobObserver(metrics))
		}
		collectionWorker = collectionjob.NewWorker(kafkaReader, kafkaWriter, collectionService, workerOptions...)
	}
	if plan.Worker && collectionWorker == nil {
		return fmt.Errorf("app mode worker requires kafka.enabled=true")
	}

	collectionScheduler := scheduler.New(
		sourceRepo,
		scheduledCollector,
		scheduler.WithLogger(log),
	)

	var server *http.Server
	if plan.HTTP {
		routerOptions := []contenthttp.RouterOption{}
		if metrics != nil {
			routerOptions = append(routerOptions, contenthttp.WithMetrics(metrics))
		}
		if cfg.Observability.TracingEnabled {
			routerOptions = append(routerOptions, contenthttp.WithTracing(cfg.Observability.ServiceName))
		}

		router := contenthttp.NewRouter(log, db, redisClient, func(api *gin.RouterGroup) {
			auth.RegisterRoutes(api, authHandler, authRequired, loginRateLimit)
			source.RegisterRoutes(api, sourceHandler, authRequired)
			article.RegisterRoutes(api, articleHandler, authRequired)
			ai.RegisterRoutes(api, aiHandler, authRequired)
			registerCollectionRoutes(api)
			registerCollectionJobRoutes(api)
		}, routerOptions...)

		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		server = &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}
	}

	backgroundCtx, stopBackground := context.WithCancel(context.Background())
	var backgroundWG sync.WaitGroup

	if plan.Scheduler {
		backgroundWG.Add(1)
		go func() {
			defer backgroundWG.Done()
			if err := collectionScheduler.Run(backgroundCtx); err != nil {
				log.Error("collection scheduler stopped with error", slog.String("error", err.Error()))
			}
		}()
	}

	if plan.Scheduler || plan.Worker {
		backgroundWG.Add(1)
		go func() {
			defer backgroundWG.Done()
			if err := aiSummaryWorker.Run(backgroundCtx); err != nil {
				log.Error("ai summary worker stopped with error", slog.String("error", err.Error()))
			}
		}()
	}

	if plan.Worker && collectionWorker != nil {
		backgroundWG.Add(1)
		go func() {
			defer backgroundWG.Done()
			if err := collectionWorker.Run(backgroundCtx); err != nil {
				log.Error("collection worker stopped with error", slog.String("error", err.Error()))
			}
		}()
	}
	if outboxDispatcher != nil {
		backgroundWG.Add(1)
		go func() {
			defer backgroundWG.Done()
			if err := outboxDispatcher.Run(backgroundCtx); err != nil {
				log.Error("outbox dispatcher stopped with error", slog.String("error", err.Error()))
			}
		}()
	}

	errCh := make(chan error, 1)
	if server != nil {
		go func() {
			fmt.Println("contentflow server started on :8080")
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	} else {
		log.Info("contentflow runtime started", slog.String("mode", cfg.App.Mode))
	}

	quit := make(chan os.Signal, 1) // 接受退出信号, 容量为1避免阻塞
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		stopBackground()
		if kafkaReader != nil {
			_ = kafkaReader.Close()
		}
		backgroundWG.Wait()
		if kafkaWriter != nil {
			_ = kafkaWriter.Close()
		}
		return err
	case <-quit:
		fmt.Println("contentflow server stopped")
	}

	stopBackground()
	if kafkaReader != nil {
		_ = kafkaReader.Close()
	}
	backgroundWG.Wait()
	if kafkaWriter != nil {
		_ = kafkaWriter.Close()
	}

	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil { // 优雅退出
			return fmt.Errorf("shutdown server: %w", err)
		}
	}

	return nil
}
