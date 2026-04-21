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
	"lambda/internal/utils/logger"
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

	subject := fmt.Sprintf("%s.iam.authorize", a.Env)
	if a.Env == "" {
		subject = "iam.authorize" // Fallback if env not set
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

// Token generation structs
type instanceTokenRequest struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id"`
}

type instanceTokenResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

// GenerateMetricsToken requests the IAM service for a scoped JWT token for the metrics agent.
func (a *Authorizer) GenerateMetricsToken(ctx context.Context, userID, functionID string) (string, error) {
	reqID := uuid.New().String()
	l := logger.ForContext(ctx).With(
		zap.String("request_id", reqID),
		zap.String("user_id", userID),
		zap.String("function_id", functionID),
	)

	req := instanceTokenRequest{
		InstanceID: functionID,
		UserID:     userID,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		l.Error("Failed to marshal instance token request", zap.Error(err))
		return "", fmt.Errorf("failed to marshal instance token request: %w", err)
	}

	subject := fmt.Sprintf("%s.iam.v1.token.generate", a.Env)
	if a.Env == "" {
		subject = "iam.v1.token.generate"
	}

	l.Debug("Requesting metrics token from IAM", zap.String("subject", subject))

	msg, err := a.Nats.Request(ctx, subject, reqData, 5*time.Second)
	if err != nil {
		l.Error("IAM token request failed", zap.Error(err))
		return "", fmt.Errorf("NATS request failed: %w", err)
	}

	var resp instanceTokenResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		l.Error("Failed to unmarshal instance token response", zap.Error(err))
		return "", fmt.Errorf("failed to unmarshal instance token response: %w", err)
	}

	if resp.Error != "" {
		l.Error("IAM service returned error on token generation", zap.String("error", resp.Error))
		return "", fmt.Errorf("IAM service error: %s", resp.Error)
	}

	if resp.Token == "" {
		l.Error("IAM service returned an empty token")
		return "", fmt.Errorf("IAM service returned an empty token")
	}

	l.Info("Successfully generated metrics token")
	return resp.Token, nil
}
