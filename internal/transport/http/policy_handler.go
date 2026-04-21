package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"lambda/internal/domain/dto"
	"lambda/internal/utils/logger"

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
	policyID := c.Param("policy_id")
	h.handlePolicyOperation(c, "delete", &dto.PolicyEvent{
		PolicyID: policyID,
	})
}

func (h *LambdaHandlers) GetPolicy(c *gin.Context) {
	principalID := c.Param("principal_id")
	h.handlePolicyOperation(c, "get", &dto.PolicyEvent{
		PrincipalID: principalID,
	})
}

func (h *LambdaHandlers) handlePolicyOperation(c *gin.Context, op string, event *dto.PolicyEvent) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	requestID := uuid.New().String()
	event.RequestID = requestID

	publishSubject  := fmt.Sprintf("%s.lambda.policy.%s", env, op)
	responseSubject := fmt.Sprintf("%s.iam.policy.*", env)

	// Build a request-scoped logger: picks up trace/request IDs from context
	// if the Gin middleware injected them, otherwise falls back to the handler logger.
	log := logger.WithContext(c.Request.Context()).With(
		zap.String(logger.F.Domain,    "policy"),
		zap.String(logger.F.Action,    "policy."+op),
		zap.String(logger.F.RequestID, requestID),
		zap.String("publish_subject",  publishSubject),
	)

	respChan := make(chan *dto.PolicyEvent, 1)

	sub, err := h.Nats.Subscribe(responseSubject, func(m *github_nats.Msg) {
		var resp dto.PolicyEvent
		if err := json.Unmarshal(m.Data, &resp); err != nil {
			log.Warn("failed to unmarshal IAM response",
				zap.String(logger.F.ErrorKind, "iam_response_parse_error"),
				zap.Error(err),
			)
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
		log.Error("failed to subscribe to response subject",
			zap.String(logger.F.ErrorKind, "nats_subscribe_error"),
			zap.String("response_subject", responseSubject),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal communication error"})
		return
	}
	defer sub.Unsubscribe()

	data, _ := json.Marshal(event)
	if err := h.Nats.Conn.Publish(publishSubject, data); err != nil {
		log.Error("failed to publish policy event",
			zap.String(logger.F.ErrorKind, "nats_publish_error"),
			zap.String("publish_subject", publishSubject),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send request to IAM"})
		return
	}

	log.Debug("policy event published, awaiting IAM response")

	select {
	case resp := <-respChan:
		if resp.Error != "" {
			log.Warn("IAM returned error",
				zap.String(logger.F.ErrorKind, "iam_policy_error"),
				zap.String("iam_error", resp.Error),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": resp.Error})
		} else {
			log.Info("policy operation succeeded")
			c.JSON(http.StatusOK, resp)
		}

	case <-time.After(5 * time.Second):
		log.Warn("IAM response timeout",
			zap.String(logger.F.ErrorKind, "iam_timeout"),
			zap.Duration("timeout", 5*time.Second),
		)
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "timeout waiting for IAM response"})

	case <-c.Request.Context().Done():
		log.Warn("client disconnected before IAM response",
			zap.String(logger.F.ErrorKind, "client_disconnected"),
		)
	}
}