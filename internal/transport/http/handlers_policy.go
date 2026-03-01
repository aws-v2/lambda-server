package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	github_nats "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func (h *LambdaHandlers) CreatePolicy(c *gin.Context) {
	var req dto.PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.handlePolicyOperation(c, "create", &dto.PolicyEvent{
		AccountID:    req.AccountID,
		PrincipalID:  req.PrincipalID,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		Action:       req.Action,
	})
}

func (h *LambdaHandlers) UpdatePolicy(c *gin.Context) {
	policyID := c.Param("policy_id")
	var req dto.PolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.handlePolicyOperation(c, "update", &dto.PolicyEvent{
		PolicyID:     policyID,
		AccountID:    req.AccountID,
		PrincipalID:  req.PrincipalID,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		Action:       req.Action,
	})
}

func (h *LambdaHandlers) DeletePolicy(c *gin.Context) {
	principalID := c.Param("principal_id")
	h.handlePolicyOperation(c, "delete", &dto.PolicyEvent{
		PrincipalID: principalID,
	})
}

func (h *LambdaHandlers) GetPolicy(c *gin.Context) {
	principalID := c.Param("principal_id")
	h.handlePolicyOperation(c, "get", &dto.PolicyEvent{
		PrincipalID: principalID,
	})
}

func (h *LambdaHandlers) handlePolicyOperation(c *gin.Context, action string, event *dto.PolicyEvent) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	requestID := uuid.New().String()
	event.RequestID = requestID

	publishSubject := fmt.Sprintf("%s.lambda.v1.policy.%s", env, action)
	responseSubject := fmt.Sprintf("%s.iam.v1.policy.*", env)

	l := logger.ForContext(c.Request.Context()).With(
		zap.String("action", action),
		zap.String("request_id", requestID),
		zap.String("publish_subject", publishSubject),
	)

	// Channel to receive the response
	respChan := make(chan *dto.PolicyEvent, 1)

	// Subscribe to IAM responses
	sub, err := h.Nats.Subscribe(responseSubject, func(m *github_nats.Msg) {
		var resp dto.PolicyEvent
		if err := json.Unmarshal(m.Data, &resp); err != nil {
			return
		}

		if resp.RequestID == requestID {
			select {
			case respChan <- &resp:
			default:
			}
		}
	})
	if err != nil {
		l.Error("Failed to subscribe to response subject", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal communication error"})
		return
	}
	defer sub.Unsubscribe()

	// Publish the request
	data, _ := json.Marshal(event)
	err = h.Nats.Conn.Publish(publishSubject, data)
	if err != nil {
		l.Error("Failed to publish policy event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send request to IAM"})
		return
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		if resp.Error != "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Error})
		} else {
			c.JSON(http.StatusOK, resp)
		}
	case <-time.After(5 * time.Second):
		l.Warn("IAM response timeout")
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "timeout waiting for IAM response"})
	case <-c.Request.Context().Done():
		l.Warn("Client disconnected")
	}
}
