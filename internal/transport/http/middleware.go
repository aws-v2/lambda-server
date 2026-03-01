package http

import (
	"context"
	"fmt"
	"time"

	"lambda/internal/infrastructure/auth"
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

// UserContextMiddleware extracts the user ID from headers.
// It prioritizes X-User-Id (from API Gateway), then unverified JWT,
// and finally resolves it from X-Access-Key-Id if provided.
// Authentication is assumed to be handled upstream by the API Gateway.
func UserContextMiddleware(resolver *auth.ApiKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := ""

		// 1. High priority: User ID passed directly by API Gateway
		if xUserID := c.GetHeader("X-User-Id"); xUserID != "" {
			userID = xUserID
		}

		// 2. Medium priority: Extract from Authorization header (unverified JWT)
		if userID == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				tokenString := authHeader
				if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
					tokenString = authHeader[7:]
				}

				parser := jwt.NewParser()
				token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
				if err == nil {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						if id, exists := claims["userId"]; exists {
							userID = fmt.Sprintf("%v", id)
						} else if sub, exists := claims["sub"]; exists {
							userID = fmt.Sprintf("%v", sub)
						}
					}
				}
			}
		}

		// 3. Low priority: Resolve from Access Key ID
		if userID == "" {
			accessKeyID := c.GetHeader("X-Access-Key-Id")
			if accessKeyID != "" {
				// Resolve the key to find the owner's userID
				// We do NOT perform secret validation here as it's assumed to be done by Gateway
				key, err := resolver.Resolve(c.Request.Context(), accessKeyID)
				if err != nil {
					logger.ForContext(c.Request.Context()).Warn("Failed to resolve identity from Access Key", zap.String("id", accessKeyID), zap.Error(err))
				} else if key != nil {
					userID = key.UserID
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
