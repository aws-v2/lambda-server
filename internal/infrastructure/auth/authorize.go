package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"lambda/internal/infrastructure/event"
	"lambda/internal/logger"
)

type AuthorizeRequest struct {
	RequestID    string `json:"request_id"`
	AccountID    string `json:"account_id"`
	PrincipalID  string `json:"principal_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	Action       string `json:"action"`
}

type AuthorizeResponse struct {
	RequestID string  `json:"request_id"`
	Allowed   bool    `json:"allowed"`
	Reason    *string `json:"reason"`
}

type Authorizer struct {
	Nats *event.NatsClient
	Env  string
}

func NewAuthorizer(nats *event.NatsClient, env string) *Authorizer {
	return &Authorizer{
		Nats: nats,
		Env:  env,
	}
}

// Authorize sends an authorization request to IAM and returns true if allowed, or an error if denied.
func (a *Authorizer) Authorize(ctx context.Context, accountID, principalID, resourceType, resourceID, action string) (bool, error) {
	reqID := uuid.New().String()
	l := logger.ForContext(ctx).With(
		zap.String("request_id", reqID),
		zap.String("account_id", accountID),
		zap.String("principal_id", principalID),
		zap.String("resource_type", resourceType),
		zap.String("resource_id", resourceID),
		zap.String("action", action),
	)

	req := AuthorizeRequest{
		RequestID:    reqID,
		AccountID:    accountID,
		PrincipalID:  principalID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		l.Error("Failed to marshal authorization request", zap.Error(err))
		return false, fmt.Errorf("authorization failed: %w", err)
	}

	subject := fmt.Sprintf("%s.iam.v1.authorize", a.Env)
	if a.Env == "" {
		subject = "iam.v1.authorize" // Fallback if env not set
	}

	// Send NATS Request with 5 second timeout
	respData, err := a.Nats.Request(ctx, subject, reqData, 5*time.Second)
	if err != nil {
		l.Error("IAM authorization request failed", zap.Error(err))
		return false, fmt.Errorf("IAM authorization request failed: %w", err)
	}

	var resp AuthorizeResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		l.Error("Failed to parse IAM authorization response", zap.Error(err))
		return false, fmt.Errorf("invalid IAM response: %w", err)
	}

	if resp.Allowed {
		l.Info("Authorization granted")
		return true, nil
	}

	reason := "Not authorized"
	if resp.Reason != nil && *resp.Reason != "" {
		reason = *resp.Reason
	}

	l.Warn("Authorization denied", zap.String("reason", reason))
	return false, errors.New(reason)
}
