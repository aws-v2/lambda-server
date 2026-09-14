package main

import (
	"fmt"
	"os"
	"time"

	"lambda/internal/application"
	"lambda/internal/config"
	"lambda/internal/domain/models"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/discovery"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	transportHTTP "lambda/internal/transport/http"
	transportHandlers "lambda/internal/transport/http/handlers"
	transportMiddleware "lambda/internal/transport/http/middleware"
	transportNATS "lambda/internal/transport/nats"
	"lambda/internal/utils/logger"
	"lambda/internal/utils/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	// ── 0. Configuration ──────────────────────────────────────────────────
	config.InitProfiles()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// ── 1. Logger ─────────────────────────────────────────────────────────
	logger.Init(cfg.Server.ServiceName, cfg.Profile, cfg.Server.Region)
	defer logger.Log.Sync() //nolint:errcheck

	logger.Log.Info("service starting",
		zap.String(logger.F.Service, cfg.Server.ServiceName),
		zap.String(logger.F.Profile, cfg.Profile),
		zap.String(logger.F.Region, cfg.Server.Region),
	)

	// ── 2. Reachability checks ────────────────────────────────────────────
	if err := config.CheckReachability(cfg.NATS.URL, 5, 2*time.Second); err != nil {
		logger.Log.Info("NATS unreachable",
			zap.String(logger.F.ErrorKind, "nats_unreachable"),
			zap.Error(err),
		)
	}

	dbAddr := fmt.Sprintf("%s:%d", cfg.DB.Host, cfg.DB.Port)
	if err := config.CheckReachability(dbAddr, 5, 2*time.Second); err != nil {
		logger.Log.Fatal("database unreachable",
			zap.String(logger.F.ErrorKind, "db_unreachable"),
			zap.Error(err),
		)
	}

	// ── 3. Eureka registration ────────────────────────────────────────────
	eurekaConfig := discovery.GetEurekaConfig()
	eurekaConfig.ServerURL = cfg.Eureka.ServerURL

	if eurekaConfig.ServerURL != "" {
		if err := discovery.RegisterWithEureka(eurekaConfig); err != nil {
			logger.Log.Error("eureka registration failed",
				zap.String(logger.F.ErrorKind, "eureka_registration"),
				zap.Error(err),
			)
		} else {
			go discovery.SendHeartbeat(eurekaConfig)
		}
	}

	// ── 4. Telemetry ──────────────────────────────────────────────────────
	otelCleanup, err := telemetry.InitTelemetry(cfg.Server.ServiceName)
	if err != nil {
		logger.Log.Fatal("telemetry init failed",
			zap.String(logger.F.ErrorKind, "otel_init"),
			zap.Error(err),
		)
	}
	defer otelCleanup()

	// ── 5. NATS connection ────────────────────────────────────────────────
	var nc *nats.Conn
	if cfg.NATS.User != "" && cfg.NATS.Password != "" {
		nc, err = nats.Connect(cfg.NATS.URL, nats.UserInfo(cfg.NATS.User, cfg.NATS.Password))
	} else {
		nc, err = nats.Connect(cfg.NATS.URL)
	}
	if err != nil {
		logger.Log.Info("NATS connect failed",
			zap.String(logger.F.ErrorKind, "nats_connect"),
			zap.String("url", cfg.NATS.URL),
			zap.Error(err),
		)
	}
	defer nc.Close()
	logger.Log.Info("NATS connected", zap.String("url", cfg.NATS.URL))

	// ── 6. PostgreSQL connection ──────────────────────────────────────────
	db, err := database.Connect(models.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		SSLMode:         cfg.DB.SSLMode,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Log.Fatal("postgres connect failed",
			zap.String(logger.F.ErrorKind, "db_connect"),
			zap.Error(err),
		)
	}
	defer db.Close()
	logger.Log.Info("postgres connected")

	migrationsPath := getEnv("MIGRATIONS_PATH", "internal/infrastructure/migrations")
	if err := db.RunMigrations(migrationsPath); err != nil {
		logger.Log.Fatal("migrations failed",
			zap.String(logger.F.ErrorKind, "db_migration"),
			zap.Error(err),
		)
	}
	logger.Log.Info("migrations completed")
	// ── 7. Infrastructure ─────────────────────────────────────────────────
	natsClient := event.NewNatsClient(nc, cfg.NatsPrefix)
	codeStorage := storage.NewStorage(getEnv("CODE_STORAGE_PATH", "./storage"), cfg.NATS.Prefix, natsClient)
	resolver := auth.NewApiKeyResolver(db, natsClient, cfg.Profile, "v1")

	// repos
	// service
	docsService := application.NewDocsService(getEnv("DOCS_PATH", "./docs"))
	funcService := application.NewLambdaService(db, natsClient, codeStorage, cfg.NatsPrefix, cfg.InvokeConfig.SystemUserId)
	configService := application.NewConfigService(db, codeStorage, resolver, cfg.Server.Region, natsClient)
	invokeService := application.NewInvokeService(db, natsClient, cfg.NatsPrefix,cfg.InvokeConfig, )

	// handers

	// ── 8. Handlers ───────────────────────────────────────────────────────
	docsHandler := transportHandlers.NewDocsHandler(docsService)
	invokeHandler := transportHandlers.NewInvokeHandler(invokeService)
	configHandler := transportHandlers.NewConfigHandler(configService)

	handlers := transportHandlers.NewLambdaHandlers(*funcService)


	// ── 9. Router ─────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(
		transportMiddleware.ZapMiddleware(cfg.Server.ServiceName),
		gin.Recovery(),
	)


	transportHTTP.SetupRoutes(router, handlers, invokeHandler, configHandler, docsHandler)

	// ── 10. NATS listeners ────────────────────────────────────────────────
	if err := transportNATS.StartScaleEventServer(natsClient, db); err != nil {
		logger.Log.Error("scale event listener failed to start",
			zap.String(logger.F.ErrorKind, "nats_listener"),
			zap.Error(err),
		)
	}

	if err := transportNATS.StartInvokeEventServer(natsClient, invokeService); err != nil {
		logger.Log.Error("scale event listener failed to start",
			zap.String(logger.F.ErrorKind, "nats_listener"),
			zap.Error(err),
		)
	}

	// ── 11. Start HTTP server ─────────────────────────────────────────────
	logger.Log.Info("HTTP server listening", zap.String("port", cfg.Server.Port))
	if err := router.Run(":" + cfg.Server.Port); err != nil {
		logger.Log.Fatal("HTTP server crashed",
			zap.String(logger.F.ErrorKind, "http_server"),
			zap.Error(err),
		)
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
