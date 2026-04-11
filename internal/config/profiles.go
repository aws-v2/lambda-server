package config

import (
	"os"
	"strings"

	"lambda/internal/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// InitProfiles loads environment variables from .env and sets standard defaults
func InitProfiles() {
	// 1. Load .env file if it exists
	err := godotenv.Load()
	if err != nil {
		logger.Log.Debug("No .env file found, skipping godotenv")
	} else {
		logger.Log.Info(".env file loaded successfully")
	}

	// 2. Get the active profile
	profile := os.Getenv("APP_PROFILE")
	if profile == "" {
		profile = "dev" // Default to dev
		os.Setenv("APP_PROFILE", profile)
	}
	profile = strings.ToLower(profile)
	logger.Log.Info("Active Profile", zap.String("profile", profile))
}
