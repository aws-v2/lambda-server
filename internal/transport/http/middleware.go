package http

import (
	"context"
	"fmt"
	"time"

	"lambda/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

// AuthMiddleware extracts the user ID from the JWT token without enforcing authentication (Trust Mode)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		userID := ""

		if authHeader != "" {
			// Header format: Bearer <token>
			tokenString := authHeader
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}

			// Parse without verification as requested (authentication is handled elsewhere)
			parser := jwt.NewParser()
			token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
			if err == nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					// Check "userId" or "sub" for the identifier
					if id, exists := claims["userId"]; exists {
						userID = fmt.Sprintf("%v", id)
					} else if sub, exists := claims["sub"]; exists {
						userID = fmt.Sprintf("%v", sub)
					}
				}
			}
		}

		// Always set the user_id, even if empty, to prevent nil pointer issues in handlers
		c.Set("user_id", userID)
		ctx := context.WithValue(c.Request.Context(), "user_id", userID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
