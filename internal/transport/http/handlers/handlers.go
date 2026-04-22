package handlers

import (
	"fmt"
	"strings"

	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/infrastructure/storage"
	// "lambda/internal/transport/http/handlers"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type LambdaHandlers struct {
	DB       *database.DB
	Nats     *event.NatsClient
	Storage  *storage.Storage
	Resolver *auth.ApiKeyResolver
	Region   string
	Docs     *DocsHandler
	InvokeHandler *InvokeHandler
	MetricHandler *MetricHandler
	ConfigHandler *ConfigHandler

}

func NewLambdaHandlers(
	db *database.DB,
	nats *event.NatsClient,
	stor *storage.Storage,
	resolver *auth.ApiKeyResolver,
	region string,
	docsHandler *DocsHandler,
	invokeHandler *InvokeHandler,
	metricHandler *MetricHandler,
	configHandler *ConfigHandler,
 


) *LambdaHandlers {
	return &LambdaHandlers{
		DB:       db,
		Nats:     nats,
		Storage:  stor,
		Resolver: resolver,
		Region:   region,
		Docs:     docsHandler,
		InvokeHandler: invokeHandler,
		MetricHandler: metricHandler,
	}
}

// ResolveIdentifier extracts the function identifier (name or ARN) from the Gin context.
// Priority order:
//  1. _resolved_arn – set by ArnRouter after stripping the sub-path suffix
//  2. *arn wildcard param (raw, with leading slash stripped)
//  3. :name regular param (name-based routes)
func (h *LambdaHandlers) ResolveIdentifier(c *gin.Context) string {
	if resolvedArn, exists := c.Get("_resolved_arn"); exists {
		if arn, ok := resolvedArn.(string); ok && arn != "" {
			return arn
		}
	}
	if arn := c.Param("arn"); arn != "" {
		return strings.TrimPrefix(arn, "/")
	}
	return c.Param("name")
}

// GenerateArn creates a Serwin Lambda ARN.
// Format: arn:serwin:lambda:<region>:<userID>:function:<name>
func (h *LambdaHandlers) GenerateArn(userID, name string) string {
	region := h.Region
	if region == "" {
		region = "eu-north-1"
	}
	return fmt.Sprintf("arn:serwin:lambda:%s:%s:function:%s", region, userID, name)
}

// ResolveFunction looks up a function by name or ARN depending on the identifier.
func (h *LambdaHandlers) ResolveFunction(identifier, userID string) (*database.Function, error) {
	if strings.HasPrefix(identifier, "arn:") {
		return h.DB.GetFunctionByARN(identifier, userID)
	}
	return h.DB.GetFunction(identifier, userID)
}

// ArnRouter handles ARN-based routing and delegates to the correct handler.
func (h *LambdaHandlers) ArnRouter(c *gin.Context) {
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Action, "lambda.route"),
		zap.String(logger.F.Domain, "lambda"),
	)

	raw := strings.TrimPrefix(c.Param("arn"), "/")

	suffixes := []string{"/metrics", "/invoke", "/config", "/code", "/test"}
	suffix := ""
	arnStr := raw

	for _, s := range suffixes {
		if strings.HasSuffix(raw, s) {
			arnStr = raw[:len(raw)-len(s)]
			suffix = s
			break
		}
	}

	c.Set("_resolved_arn", arnStr)

	log.Info("routing ARN request",
		zap.String("function_arn", arnStr),
		zap.String("route_suffix", suffix),
	)

	switch suffix {
	case "/invoke", "/test":
		log.Debug("dispatching to invoke handler")
		h.InvokeHandler.Invoke(c)

	case "/metrics":
		log.Debug("dispatching to metrics handler")
		h.MetricHandler.GetMetrics(c)

	case "/config":
		log.Debug("dispatching to config handler")
		h.ConfigHandler.UpdateConfig(c)

	case "/code":
		log.Debug("dispatching to code handler")
		h.ConfigHandler.UpdateCode(c)

	default:
		log.Debug("dispatching to get function handler")
		h.GetFunction(c)
	}
}

func GetMetrics(c *gin.Context) {
	panic("unimplemented")
}