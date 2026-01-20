package http

import (
	"context"
	"time"

	"lambda/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// ZapMiddleware is a Gin middleware that logs requests using Zap and OpenTelemetry
func ZapMiddleware(serviceName string) gin.HandlerFunc {
	// Wrap otelgin middleware
	otelMiddleware := otelgin.Middleware(serviceName)

	return func(c *gin.Context) {
		// Call otelgin middleware first
		otelMiddleware(c)
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Trace ID handling
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		c.Set("trace_id", traceID)

		// Synchronize with standard library context
		ctx := context.WithValue(c.Request.Context(), "trace_id", traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.ForContext(c.Request.Context()).Error("Gin Error", zap.String("error", e))
			}
		} else {
			logger.ForContext(c.Request.Context()).Info(path,
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.Duration("latency", latency),
			)
		}
	}
}
