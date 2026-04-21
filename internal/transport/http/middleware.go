package http

import (
	"fmt"
	"time"

	"lambda/internal/infrastructure/auth"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

// ZapMiddleware logs every request and injects a trace-scoped logger into the
// context so all downstream handlers can call logger.WithContext(ctx, fallback).
func ZapMiddleware(serviceName string) gin.HandlerFunc {
	otelMiddleware := otelgin.Middleware(serviceName)

	return func(c *gin.Context) {
		otelMiddleware(c)
		start := time.Now()

		path  := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Resolve or generate trace ID
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		requestID := uuid.New().String()

		c.Set("trace_id",   traceID)
		c.Set("request_id", requestID)

		// Build the request-scoped logger and inject into context.
		// Every handler downstream gets this logger via logger.WithContext(ctx, fallback).
		ctx, _ := logger.FromRequest(c.Request.Context(), logger.RequestMeta{
			TraceID:   traceID,
			RequestID: requestID,
			Method:    c.Request.Method,
			Path:      path,
		})
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		latency := time.Since(start)
		log     := logger.WithContext(c.Request.Context())

		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				log.Error("request error",
					zap.String(logger.F.Action,    "http.request"),
					zap.String(logger.F.ErrorKind, "gin_error"),
					zap.String("error", e),
				)
			}
			return
		}

		log.Info("request completed",
			zap.String(logger.F.Action,        "http.request"),
			zap.Int(logger.F.HTTPStatus,        c.Writer.Status()),
			zap.String(logger.F.HTTPMethod,     c.Request.Method),
			zap.String(logger.F.HTTPPath,       path),
			zap.String(logger.F.HTTPUserAgent,  c.Request.UserAgent()),
			zap.Int64(logger.F.DurationMS,      latency.Milliseconds()),
			zap.String("query",                 query),
			zap.String("client_ip",             c.ClientIP()),
		)
	}
}

// UserContextMiddleware extracts the user ID from headers and sets it on the
// context-injected logger so all downstream log lines carry user.id automatically.
//
// Priority: X-User-Id (API Gateway) → JWT claim → X-Access-Key-Id resolution.
// Authentication is assumed to be handled upstream by the API Gateway.
func UserContextMiddleware(resolver *auth.ApiKeyResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := ""

		// 1. High priority: user ID forwarded directly by API Gateway
		if xUserID := c.GetHeader("X-User-Id"); xUserID != "" {
			userID = xUserID
		}

		// 2. Medium priority: extract from unverified JWT
		if userID == "" {
			if authHeader := c.GetHeader("Authorization"); authHeader != "" {
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

		// 3. Low priority: resolve from access key ID
		if userID == "" {
			if accessKeyID := c.GetHeader("X-Access-Key-Id"); accessKeyID != "" {
				log := logger.WithContext(c.Request.Context())
				key, err := resolver.Resolve(c.Request.Context(), accessKeyID)
				if err != nil {
					log.Warn("failed to resolve identity from access key",
						zap.String(logger.F.Action,    "auth.resolve"),
						zap.String(logger.F.Domain,    "auth"),
						zap.String(logger.F.ErrorKind, "access_key_resolve_error"),
						zap.String("access_key_id",    accessKeyID),
						zap.Error(err),
					)
				} else if key != nil {
					userID = key.UserID
				}
			}
		}

		c.Set("user_id", userID)

		// Enrich the already-injected logger with user.id so every downstream
		// log line carries it without handlers needing to add it manually.
		existing := logger.WithContext(c.Request.Context())
		enriched := existing.With(zap.String(logger.F.UserID, userID))
		c.Request = c.Request.WithContext(logger.InjectLogger(c.Request.Context(), enriched))

		c.Next()
	}
}