package config

import (
	"fmt"
	"os"
	"strings"

	"lambda/internal/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// InitProfiles loads environment variables from .env and maps profile-specific variables
func InitProfiles() {
	// 1. Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		logger.Log.Warn("No .env file found, skipping godotenv", zap.Error(err))
	} else {
		logger.Log.Info(".env file loaded successfully")
	}

	// 2. Get the active profile
	profile := os.Getenv("APP_PROFILE")
	if profile == "" {
		profile = "DEV" // Default to DEV
		os.Setenv("APP_PROFILE", profile)
	}
	profile = strings.ToUpper(profile)
	logger.Log.Info("Active Profile", zap.String("profile", profile))

	// 3. Map profile-specific variables to base variables
	// Example: DEV_DATASOURCE_URL -> DB_URL (or DB_HOST, etc.)
	// We'll map the ones provided in the request

	mapVar(profile, "DATASOURCE_URL", "DB_URL")
	mapVar(profile, "DATASOURCE_USERNAME", "DB_USER")
	mapVar(profile, "DATASOURCE_PASSWORD", "DB_PASSWORD")
	mapVar(profile, "NATS_URL", "NATS_URL")
	mapVar(profile, "EUREKA_SERVER_URL", "EUREKA_SERVER_URL")

	// Post-process the DB_URL if it's JDBC-style
	dbURL := os.Getenv("DB_URL")
	if strings.HasPrefix(dbURL, "jdbc:postgresql://") {
		postgresURL := strings.Replace(dbURL, "jdbc:postgresql://", "postgres://", 1)
		os.Setenv("DB_URL", postgresURL)
		logger.Log.Info("Converted JDBC-style URL to PostgreSQL DSN", zap.String("url", postgresURL))
	}
}

func mapVar(profile, suffix, target string) {
	key := fmt.Sprintf("%s_%s", profile, suffix)
	val := os.Getenv(key)
	if val != "" {
		os.Setenv(target, val)
		// Mask sensitive info in logs
		logVal := val
		if strings.Contains(strings.ToLower(target), "password") || strings.Contains(strings.ToLower(target), "secret") {
			logVal = "****"
		}
		logger.Log.Info("Mapped environment variable", zap.String("from", key), zap.String("to", target), zap.String("value", logVal))
	} else {
		logger.Log.Warn("Missing environment variable for profile", zap.String("key", key))
	}
}
