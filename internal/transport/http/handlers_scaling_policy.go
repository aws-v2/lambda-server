package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/auth"
	"lambda/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func extractTenantIDFromJWT(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	tokenString := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	}
	parser := jwt.NewParser()
	if token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{}); err == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if id, exists := claims["userId"]; exists {
				return fmt.Sprintf("%v", id)
			}
		}
	}
	return ""
}

func bindPolicyPayload(c *gin.Context, req *dto.LambdaScalingPolicyRequest) error {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return err
	}
	if policyRaw, ok := raw["policy"].(map[string]interface{}); ok {
		b, _ := json.Marshal(policyRaw)
		json.Unmarshal(b, req)
	} else {
		b, _ := json.Marshal(raw)
		json.Unmarshal(b, req)
	}
	return nil
}

func (h *LambdaHandlers) CreateLambdaScalingPolicy(c *gin.Context) {


	fmt.Println("CreateLambdaScalingPolicy----------w----------")
	functionID := c.Param("functionId")
	var req dto.LambdaScalingPolicyRequest
	if err := bindPolicyPayload(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.FunctionID = functionID

	if req.TenantID == "" {
		req.TenantID = extractTenantIDFromJWT(c)
	}

	h.handleLambdaScalingPolicyOperation(c, "create", &dto.LambdaScalingPolicyEvent{
		Policy: req,
	})

}

func (h *LambdaHandlers) UpdateLambdaScalingPolicy(c *gin.Context) {
	functionID := c.Param("functionId")
	var req dto.LambdaScalingPolicyRequest
	if err := bindPolicyPayload(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.FunctionID = functionID

	if req.TenantID == "" {
		req.TenantID = extractTenantIDFromJWT(c)
	}

	h.handleLambdaScalingPolicyOperation(c, "update", &dto.LambdaScalingPolicyEvent{
		Policy: req,
	})
}

func (h *LambdaHandlers) DeleteLambdaScalingPolicy(c *gin.Context) {
	functionID := c.Param("functionId")
	var req dto.LambdaScalingPolicyRequest
	// For DELETE, body might be empty
	if err := bindPolicyPayload(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.FunctionID = functionID

	if req.TenantID == "" {
		req.TenantID = c.Query("tenant_id")
		if req.TenantID == "" {
			req.TenantID = extractTenantIDFromJWT(c)
		}
	}

	h.handleLambdaScalingPolicyOperation(c, "delete", &dto.LambdaScalingPolicyEvent{
		Policy: req,
	})
}

func (h *LambdaHandlers) ListLambdaScalingPolicies(c *gin.Context) {
	var req dto.LambdaScalingPolicyRequest
	// For GET, read from query or body
	if err := bindPolicyPayload(c, &req); err != nil {
		// Ignore EOF since GET usually doesn't have a body
	}
	if req.TenantID == "" {
		req.TenantID = c.Query("tenant_id")
		if req.TenantID == "" {
			req.TenantID = extractTenantIDFromJWT(c)
		}
	}

	h.handleLambdaScalingPolicyOperation(c, "list", &dto.LambdaScalingPolicyEvent{
		Policy: req,
	})
}

func (h *LambdaHandlers) handleLambdaScalingPolicyOperation(c *gin.Context, action string, event *dto.LambdaScalingPolicyEvent) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	requestID := uuid.New().String()
	event.RequestID = requestID
	event.Action = action

	publishSubject := fmt.Sprintf("%s.lambda.v1.scaling_policy.%s", env, action)


	fmt.Println("CreateLambdaScalingPolicy---------d-------%s----", publishSubject)


	l := logger.ForContext(c.Request.Context()).With(
		zap.String("action", action),
		zap.String("request_id", requestID),
		zap.String("publish_subject", publishSubject),
		zap.String("tenant_id", event.Policy.TenantID),
		zap.String("function_id", event.Policy.FunctionID),
	)

	authorizer := auth.NewAuthorizer(h.Nats, env)
	token, err := authorizer.GenerateMetricsToken(c.Request.Context(), event.Policy.TenantID, event.Policy.FunctionID)
	if err != nil {
		l.Error("Failed to generate metrics token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to authenticate with metrics service"})
		return
	}
	event.Token = token

	var payload map[string]interface{}
	switch action {
	case "create":
		payload = map[string]interface{}{
			"correlation_id": requestID,
			"tenant_id":      event.Policy.TenantID,
			"policy":         event.Policy,
		}
	case "update":
		payload = map[string]interface{}{
			"correlation_id": requestID,
			"tenant_id":      event.Policy.TenantID,
			"function_id":    event.Policy.FunctionID,
			"metric_name":    event.Policy.MetricName,
			"update":         event.Policy,
		}
	case "delete":
		payload = map[string]interface{}{
			"correlation_id": requestID,
			"tenant_id":      event.Policy.TenantID,
			"function_id":    event.Policy.FunctionID,
			"metric_name":    event.Policy.MetricName,
		}
	case "list":
		payload = map[string]interface{}{
			"correlation_id": requestID,
			"tenant_id":      event.Policy.TenantID,
		}
	default:
		payload = map[string]interface{}{
			"correlation_id": requestID,
			"tenant_id":      event.Policy.TenantID,
			"policy":         event.Policy,
		}
	}
	payload["token"] = token

	data, err := json.Marshal(payload)
	if err != nil {
		l.Error("Failed to marshal event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize request"})
		return
	}

	// Use robust 5s timeout; wait for NATS response
	respData, err := h.Nats.Request(c.Request.Context(), publishSubject, data, 5*time.Second)
	if err != nil {
		l.Error("Failed to perform scaling policy operation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send request to metrics server"})
		return
	}

	var rawResp map[string]interface{}
	if err := json.Unmarshal(respData, &rawResp); err != nil {
		l.Warn("Failed to parse metrics server response", zap.Error(err))
		if action == "list" {
			c.JSON(http.StatusOK, gin.H{"policies": []interface{}{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "success", "policy": event.Policy})
		return
	}

	if errMsg, ok := rawResp["error"].(string); ok && errMsg != "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	if action == "list" {
		c.JSON(http.StatusOK, rawResp)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"policy":  event.Policy,
	})
}
