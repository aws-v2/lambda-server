package config

import (
	"os"
	"strings"

	"lambda/internal/utils/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// InitProfiles loads environment variables from .env and sets standard defaults.
// Must be called before logger.Init — uses a temporary bootstrap logger since
// the real logger depends on the profile this function resolves.
func InitProfiles() {
	// Bootstrap logger for this function only — plain stderr, no JSON,
	// because we don't yet know the profile or service name.
	boot, _ := zap.NewDevelopment()
	defer boot.Sync()

	log := boot.With(
		zap.String(logger.F.Domain, "config"),
		zap.String(logger.F.Action, "profile.init"),
	)

	// 1. Load .env if present — silently skip in production containers
	if err := godotenv.Load(); err != nil {
		log.Debug(".env not found, relying on environment")
	} else {
		log.Info(".env loaded")
	}

	// 2. Resolve and normalise APP_PROFILE
	profile := strings.ToLower(os.Getenv("APP_PROFILE"))
	if profile == "" {
		profile = "dev"
		os.Setenv("APP_PROFILE", profile)
		log.Debug("APP_PROFILE not set, defaulting",
			zap.String(logger.F.Env, profile),
		)
	}

	log.Info("profile resolved",
		zap.String(logger.F.Env, profile),
	)
}