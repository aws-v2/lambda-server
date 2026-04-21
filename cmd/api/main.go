package main

import (
	"fmt"
	"os"
	"time"

	"lambda/internal/application"
	"lambda/internal/config"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/discovery"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	transportHTTP "lambda/internal/transport/http"
	transportNATS "lambda/internal/transport/nats"
	"lambda/internal/utils/logger"
	"lambda/internal/utils/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// ── 0. Configuration ──────────────────────────────────────────────────
	config.InitProfiles()

	cfg, err := config.Load()
	if err != nil {
		// Logger isn't ready yet — use stderr directly
		fmt.Fprintf(os.Stderr, "FATAL: failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// ── 1. Logger — must come right after config ──────────────────────────
	// Profile drives format (dev=console, staging/prod=JSON) and level.
	// Service name and region are baked into every JSON line for Kibana.
	logger.Init(cfg.Server.ServiceName, cfg.Profile, cfg.Server.Region)
	defer logger.Log.Sync() //nolint:errcheck

	logger.Log.Info("service starting",
		zap.String(logger.F.Service, cfg.Server.ServiceName),
		zap.String(logger.F.Profile, cfg.Profile),
		zap.String(logger.F.Region,  cfg.Server.Region),
	)

	// ── 2. Reachability checks ────────────────────────────────────────────
	if err := config.CheckReachability(cfg.NATS.URL, 5, 2*time.Second); err != nil {
		logger.Log.Fatal("NATS unreachable",
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
		logger.Log.Fatal("NATS connect failed",
			zap.String(logger.F.ErrorKind, "nats_connect"),
			zap.String("url", cfg.NATS.URL),
			zap.Error(err),
		)
	}
	defer nc.Close()
	logger.Log.Info("NATS connected", zap.String("url", cfg.NATS.URL))

	// ── 6. PostgreSQL connection ──────────────────────────────────────────
	dbConfig := database.Config{
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
	}

	db, err := database.Connect(dbConfig)
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

	// ── 7. Infrastructure & handlers ─────────────────────────────────────
	natsClient  := event.NewNatsClient(nc)
	storagePath := getEnv("CODE_STORAGE_PATH", "./storage")
	codeStorage := storage.NewStorage(storagePath)
	resolver    := auth.NewApiKeyResolver(db, natsClient, cfg.Profile, "v1")

	docsBasePath := getEnv("DOCS_PATH", "./docs")
	docsService  := application.NewDocsService(docsBasePath)
	docsHandler  := transportHTTP.NewDocsHandler(docsService)

	handlers := transportHTTP.NewLambdaHandlers(db, natsClient, codeStorage, resolver, cfg.Server.Region, docsHandler)

	// ── 8. Router ─────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(
		transportHTTP.ZapMiddleware(cfg.Server.ServiceName), // attaches trace/request IDs to ctx
		gin.Recovery(),
	)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	transportHTTP.SetupRoutes(router, handlers)

	// ── 9. NATS listeners ─────────────────────────────────────────────────
	if err := transportNATS.StartScaleEventServer(natsClient, db); err != nil {
		logger.Log.Error("scale event listener failed to start",
			zap.String(logger.F.ErrorKind, "nats_listener"),
			zap.Error(err),
		)
	}

	// ── 10. Start HTTP server ─────────────────────────────────────────────
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