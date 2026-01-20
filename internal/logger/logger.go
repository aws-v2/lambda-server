package logger

import (
	"context"
	"go.uber.org/zap"
)

var Log *zap.Logger

func Init() {
	var err error
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	Log, err = config.Build()
	if err != nil {
		panic(err)
	}
}

// ForContext returns a logger with fields extracted from the context
func ForContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Log
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return Log.With(zap.String("trace_id", traceID))
	}
	return Log
}
