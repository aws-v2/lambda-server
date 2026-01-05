package main

import (
	"lambda/internal/application"
	"lambda/internal/domain"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/repository"
	"lambda/internal/infrastructure/runtime"
	"lambda/internal/infrastructure/storage"
	transportHTTP "lambda/internal/transport/http"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
)

func main() {
	log.Println("Starting Lambda Service...")

	// 1. Connect to NATS (for IAM auth and events)
	natsURL := getEnv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("Connected to NATS at %s", natsURL)

	// 2. Connect to PostgreSQL
	dbConfig := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "lambda_db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// 3. Initialize infrastructure adapters
	functionRepo := repository.NewPostgresFunctionRepository(db)
	invocationRepo := repository.NewPostgresInvocationRepository(db)
	eventPublisher := event.NewNatsEventPublisher(nc)

	storagePath := getEnv("CODE_STORAGE_PATH", "./lambda-code")
	codeStorage, err := storage.NewLocalCodeStorage(storagePath)
	if err != nil {
		log.Fatalf("Failed to initialize code storage: %v", err)
	}

	// 4. Initialize runtime executors
	executors := map[domain.Runtime]domain.RuntimeExecutor{
		domain.RuntimeJavaScript: runtime.NewJavaScriptExecutor(),
		domain.RuntimeDocker:     runtime.NewDockerExecutor(),
	}

	// 5. Initialize application services
	functionService := application.NewFunctionService(functionRepo, codeStorage)
	invocationService := application.NewInvocationService(functionRepo, invocationRepo, executors, eventPublisher)

	// 6. Initialize HTTP handlers
	handlers := transportHTTP.NewLambdaHandlers(functionService, invocationService)

	// 7. Setup Gin router
	router := gin.Default()
	transportHTTP.SetupRoutes(router, handlers, nc)

	// 8. Start server
	port := getEnv("PORT", "8083")
	log.Printf("Lambda Service starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
