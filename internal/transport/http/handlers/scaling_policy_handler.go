package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"lambda/internal/application"
	"lambda/internal/domain/dto"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type PolicyLambdaHandlers struct {
	PolicyService *application.LambdaScalingPolicyService
}


func NewPolicyLambdaHandlers(policyService *application.LambdaScalingPolicyService) *PolicyLambdaHandlers {
	return &PolicyLambdaHandlers{PolicyService: policyService}
}

func (h *PolicyLambdaHandlers) CreateLambdaScalingPolicy(c *gin.Context) {
	functionID := c.Param("functionId")

	var req dto.LambdaScalingPolicyRequest
	if err := bindPolicyPayload(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FunctionID = functionID
	req.TenantID = extractTenantIDFromJWT(c)

	policy, err := h.PolicyService.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"policy": policy})
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

func (h *PolicyLambdaHandlers) UpdateLambdaScalingPolicy(c *gin.Context) {
	functionID := c.Param("functionId")

	var req dto.LambdaScalingPolicyRequest
	if err := bindPolicyPayload(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FunctionID = functionID
	req.TenantID = extractTenantIDFromJWT(c)

	policy, err := h.PolicyService.Update(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"policy": policy})
}

func (h *PolicyLambdaHandlers) DeleteLambdaScalingPolicy(c *gin.Context) {
	functionID := c.Param("functionId")

	var req dto.LambdaScalingPolicyRequest
	_ = bindPolicyPayload(c, &req)

	req.FunctionID = functionID
	req.TenantID = extractTenantIDFromJWT(c)

	err := h.PolicyService.Delete(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *PolicyLambdaHandlers) ListLambdaScalingPolicies(c *gin.Context) {
	var req dto.LambdaScalingPolicyRequest
	_ = bindPolicyPayload(c, &req)

	req.TenantID = extractTenantIDFromJWT(c)

	policies, err := h.PolicyService.List(c.Request.Context(), req.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"policies": policies})
}