package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// Init builds a profile-aware logger.
//   - dev:            human-readable console output, DEBUG level, caller shown
//   - staging / prod: structured JSON to stdout, INFO level, ELK-ready envelope
func Init(serviceName, profile, region string) {
	var core zapcore.Core

	level := resolveLevel(profile)

	switch strings.ToLower(profile) {
	case "dev", "development", "local":
		core = devCore(level)
	default:
		core = jsonCore(level, serviceName, profile, region)
	}

	Log = zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

// devCore writes colorized, human-readable logs to stderr.
func devCore(level zapcore.Level) zapcore.Core {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")

	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.AddSync(os.Stderr),
		level,
	)
}

// jsonCore writes structured JSON to stdout — one line per log entry,
// with static service/profile/region fields baked in at construction.
// This is what Filebeat picks up and ships to Logstash → Elasticsearch.
func jsonCore(level zapcore.Level, service, profile, region string) zapcore.Core {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey    = F.Timestamp
	cfg.LevelKey   = F.Level
	cfg.MessageKey = F.Message
	cfg.CallerKey  = F.Caller
	cfg.EncodeTime  = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Bake service identity into every log line so Kibana can filter
	// by service/profile/region without needing Logstash enrichment.
	return core.With([]zapcore.Field{
		zap.String(F.Service, service),
		zap.String(F.Profile, profile),
		zap.String(F.Region, region),
	})
}

func resolveLevel(profile string) zapcore.Level {
	// Allow explicit override via LOG_LEVEL env var
	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(strings.ToLower(raw))); err == nil {
			return l
		}
	}

	switch strings.ToLower(profile) {
	case "dev", "development", "local":
		return zapcore.DebugLevel
	case "staging":
		return zapcore.DebugLevel // verbose on staging so you can trace issues
	default:
		return zapcore.InfoLevel
	}
}