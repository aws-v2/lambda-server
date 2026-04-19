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
	"lambda/internal/logger"
	"lambda/internal/telemetry"
	transportHTTP "lambda/internal/transport/http"
	transportNATS "lambda/internal/transport/nats"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	// 0. Initialize Profile-based Configuration
	config.InitProfiles()

	// 0.1 Load Structured Configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Log.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Log.Info("Starting Modular Lambda Service", 
		zap.String("service", cfg.Server.ServiceName),
		zap.String("profile", cfg.Profile),
		zap.String("region", cfg.Server.Region),
	)

	// Step 2: Reachability & Diagnostics Pre-checks
	// 1. NATS Reachability Check
	if err := config.CheckReachability(cfg.NATS.URL, 5, 2*time.Second); err != nil {
		logger.Log.Fatal("NATS is unreachable", zap.Error(err))
	}

	// 2. Database Reachability Check
	dbAddr := fmt.Sprintf("%s:%d", cfg.DB.Host, cfg.DB.Port)
	if err := config.CheckReachability(dbAddr, 5, 2*time.Second); err != nil {
		logger.Log.Fatal("Database is unreachable", zap.Error(err))
	}

	// Eureka Configuration
	eurekaConfig := discovery.GetEurekaConfig()
	eurekaConfig.ServerURL = cfg.Eureka.ServerURL

	if eurekaConfig.ServerURL != "" {
		if err := discovery.RegisterWithEureka(eurekaConfig); err != nil {
			logger.Log.Error("Failed to register with Eureka", zap.Error(err))
		} else {
			go discovery.SendHeartbeat(eurekaConfig)
		}
	}

	// 0.2 Initialize Telemetry (OTel + Prometheus)
	otelCleanup, err := telemetry.InitTelemetry(cfg.Server.ServiceName)
	if err != nil {
		logger.Log.Fatal("Failed to initialize telemetry", zap.Error(err))
	}
	defer otelCleanup()

	// 1. Connect to NATS
	var nc *nats.Conn
	if cfg.NATS.User != "" && cfg.NATS.Password != "" {
		nc, err = nats.Connect(cfg.NATS.URL, nats.UserInfo(cfg.NATS.User, cfg.NATS.Password))
	} else {
		nc, err = nats.Connect(cfg.NATS.URL)
	}

	if err != nil {
		logger.Log.Fatal("Failed to connect to NATS", zap.Error(err), zap.String("url", cfg.NATS.URL))
	}
	defer nc.Close()
	logger.Log.Info("Connected to NATS", zap.String("url", cfg.NATS.URL))

	// 2. Connect to PostgreSQL
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
		logger.Log.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer db.Close()
	logger.Log.Info("Successfully connected to PostgreSQL")

	migrationsPath := getEnv("MIGRATIONS_PATH", "internal/infrastructure/migrations")
	if err := db.RunMigrations(migrationsPath); err != nil {
		logger.Log.Fatal("Failed to run database migrations", zap.Error(err))
	}
	logger.Log.Info("Database migrations completed")

// 3. Initialize Infrastructure
	natsClient := event.NewNatsClient(nc)
	storagePath := getEnv("CODE_STORAGE_PATH", "./storage")
	codeStorage := storage.NewStorage(storagePath)

	// 4. Initialize Handlers and Auth Resolver
	resolver := auth.NewApiKeyResolver(db, natsClient, cfg.Profile, "v1")

	docsBasePath := getEnv("DOCS_PATH", "./docs")
	docsService := application.NewDocsService(docsBasePath)
	docsHandler := transportHTTP.NewDocsHandler(docsService)

	handlers := transportHTTP.NewLambdaHandlers(db, natsClient, codeStorage, resolver, cfg.Server.Region,docsHandler)

	// 5. Setup Gin router
	router := gin.New()
	router.Use(transportHTTP.ZapMiddleware(cfg.Server.ServiceName), gin.Recovery())

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	transportHTTP.SetupRoutes(router, handlers)

	// 5.5 Start NATS listeners
	if err := transportNATS.StartScaleEventServer(natsClient, db); err != nil {
		logger.Log.Error("Failed to start scale event listener", zap.Error(err))
	}

	// 6. Start server
	logger.Log.Info("Lambda Service starting", zap.String("port", cfg.Server.Port))
	if err := router.Run(":" + cfg.Server.Port); err != nil {
		logger.Log.Fatal("Failed to start server", zap.Error(err))
	}

}

// Helpers

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
