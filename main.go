package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	transportHTTP "lambda/internal/transport/http"

	"go.uber.org/zap"

	"lambda/internal/config"
	"lambda/internal/logger"
	"lambda/internal/telemetry"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// EurekaConfig holds Eureka registration configuration
type EurekaConfig struct {
	ServerURL         string
	AppName           string
	HostName          string
	IPAddr            string
	Port              int
	VipAddress        string
	InstanceID        string
	HeartbeatInterval time.Duration
}

// getEurekaConfig reads Eureka configuration from environment variables
func getEurekaConfig() *EurekaConfig {
	return &EurekaConfig{
		ServerURL:         getEnv("EUREKA_SERVER_URL", "http://localhost:8761/eureka"),
		AppName:           getEnv("EUREKA_APP_NAME", "LAMBDA-SERVICE"),
		HostName:          getEnv("EUREKA_HOSTNAME", "localhost"),
		IPAddr:            getEnv("EUREKA_IP_ADDR", "127.0.0.1"),
		Port:              getEnvInt("SERVER_PORT", 8053),
		VipAddress:        getEnv("EUREKA_VIP_ADDRESS", "lambda-service"),
		InstanceID:        getEnv("EUREKA_INSTANCE_ID", "lambda-service:8053"),
		HeartbeatInterval: getEnvDuration("EUREKA_HEARTBEAT_INTERVAL", 30*time.Second),
	}
}

// registerWithEureka registers the service instance with Eureka server
func registerWithEureka(config *EurekaConfig) error {
	instance := map[string]interface{}{
		"instance": map[string]interface{}{
			"instanceId": config.InstanceID,
			"hostName":   config.HostName,
			"app":        config.AppName,
			"ipAddr":     config.IPAddr,
			"vipAddress": config.VipAddress,
			"status":     "UP",
			"port": map[string]interface{}{
				"$":        config.Port,
				"@enabled": "true",
			},
			"dataCenterInfo": map[string]interface{}{
				"@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
				"name":   "MyOwn",
			},
			"healthCheckUrl": fmt.Sprintf("http://%s:%d/health", config.HostName, config.Port),
			"statusPageUrl":  fmt.Sprintf("http://%s:%d/health", config.HostName, config.Port),
			"homePageUrl":    fmt.Sprintf("http://%s:%d/", config.HostName, config.Port),
		},
	}

	jsonData, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal Eureka registration data: %w", err)
	}

	url := fmt.Sprintf("%s/apps/%s", config.ServerURL, config.AppName)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register with Eureka: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("eureka registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Log.Info("✅ Successfully registered with Eureka server at %s")
	return nil
}

// sendHeartbeat sends periodic heartbeats to Eureka server
func sendHeartbeat(config *EurekaConfig) {
	ticker := time.NewTicker(config.HeartbeatInterval)
	defer ticker.Stop()

	url := fmt.Sprintf("%s/apps/%s/%s", config.ServerURL, config.AppName, config.InstanceID)
	client := &http.Client{Timeout: 5 * time.Second}

	for range ticker.C {
		req, _ := http.NewRequest("PUT", url, nil)

		resp, _ := client.Do(req)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			logger.Log.Info("⚠️  Heartbeat failed")
		} else {
			logger.Log.Info("💓 Heartbeat sent successfully to Eureka")
		}

		resp.Body.Close()
	}
}

// deregisterFromEureka removes the service instance from Eureka
func deregisterFromEureka(config *EurekaConfig) error {
	url := fmt.Sprintf("%s/apps/%s/%s", config.ServerURL, config.AppName, config.InstanceID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create deregistration request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deregister from Eureka: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregistration failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Log.Info("✅ Successfully deregistered from Eureka server")
	return nil
}

func main() {
	logger.Init()
	defer logger.Log.Sync()
	logger.Log.Info("Starting Modular Lambda Service...")

	// 0. Initialize Profile-based Configuration
	config.InitProfiles()

	// 0.1 Initialize Configuration from External Server
	configURL := os.Getenv("CONFIG_SERVER_URL")
	if configURL != "" {
		if err := config.LoadConfig(configURL); err != nil {
			logger.Log.Warn("Failed to load external configuration from server, continuing with local env", zap.String("url", configURL), zap.Error(err))
		}
	} else {
		logger.Log.Info("No CONFIG_SERVER_URL provided, skipping external configuration")
	}

	// Eureka Configuration
	eurekaConfig := getEurekaConfig()
	// Use resolved EUREKA_SERVER_URL if set
	if resolvedURL := os.Getenv("EUREKA_SERVER_URL"); resolvedURL != "" {
		eurekaConfig.ServerURL = resolvedURL
	}

	if err := registerWithEureka(eurekaConfig); err != nil {
		logger.Log.Error("Failed to register with Eureka", zap.Error(err))
	} else {
		go sendHeartbeat(eurekaConfig)
	}

	// 0.1 Initialize Telemetry (OTel + Prometheus)
	serviceName := getEnv("SERVICE_NAME", "lambda-service")
	otelCleanup, err := telemetry.InitTelemetry(serviceName)
	if err != nil {
		logger.Log.Fatal("Failed to initialize telemetry", zap.Error(err))
	}
	defer otelCleanup()

	// 1. Connect to NATS
	natsURL := getEnv("NATS_URL", nats.DefaultURL)
	nc, err := nats.Connect(natsURL)
	if err != nil {
		logger.Log.Fatal("Failed to connect to NATS", zap.Error(err))
	}
	defer nc.Close()
	logger.Log.Info("Connected to NATS", zap.String("url", natsURL))

	// 2. Connect to PostgreSQL
	dbConfig := database.Config{
		Host:            getEnv("DB_HOST", ""),
		Port:            getEnv("DB_PORT", ""),
		User:            getEnv("DB_USER", ""),
		Password:        getEnv("DB_PASSWORD", ""),
		DBName:          getEnv("DB_NAME", ""),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getEnv("DB_CONN_MAX_LIFETIME", "5m"),
	}

	var db *database.DB
	maxRetries := 4

	for i := range maxRetries {
		logger.Log.Info("Attempting to connect to PostgreSQL...", zap.Int("attempt", i+1))
		db, err = database.Connect(dbConfig)
		if err == nil {
			break
		}
		logger.Log.Warn("Failed to connect to PostgreSQL", zap.Int("attempt", i+1), zap.Error(err))
		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		logger.Log.Warn("Could not connect to PostgreSQL after retries, falling back to SQLite")
		sqlitePath := getEnv("SQLITE_PATH", "lambda.db")
		db, err = database.ConnectSQLite(sqlitePath)
		if err != nil {
			logger.Log.Fatal("Failed to connect to SQLite fallback", zap.Error(err))
		}
		logger.Log.Info("Connected to SQLite fallback", zap.String("path", sqlitePath))
	} else {
		logger.Log.Info("Successfully connected to PostgreSQL")
	}
	defer db.Close()

	migrationsPath := getEnv("MIGRATIONS_PATH", "internal/infrastructure/migrations")
	if err := db.RunMigrations(migrationsPath); err != nil {
		logger.Log.Fatal("Failed to run database migrations", zap.Error(err))
	}
	logger.Log.Info("Database migrations completed")

	// 3. Initialize Infrastructure
	natsClient := event.NewNatsClient(nc)
	storagePath := getEnv("CODE_STORAGE_PATH", "./storage")
	codeStorage := storage.NewStorage(storagePath)

	// 4. Initialize HTTP Handlers
	handlers := transportHTTP.NewLambdaHandlers(db, natsClient, codeStorage)

	// 5. Setup Gin router
	router := gin.New()
	router.Use(transportHTTP.ZapMiddleware(serviceName), gin.Recovery())

	// Expose metrics for Prometheus
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	transportHTTP.SetupRoutes(router, handlers)

	// 6. Start server
	port := getEnv("PORT", "8053")
	logger.Log.Info("Lambda Service starting", zap.String("port", port))
	if err := router.Run(":" + port); err != nil {
		logger.Log.Fatal("Failed to start server", zap.Error(err))
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}
