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

func (a *Authorizer) Authorize(ctx context.Context, accountID, principalID, resourceType, resourceID, action string) (bool, error) {
	reqID := uuid.New().String()

	log := logger.WithContext(ctx).With(
		zap.String(logger.F.Action,        "iam.authorize"),
		zap.String(logger.F.Domain,        "auth"),
		zap.String("auth.request_id",     reqID),
		zap.String("account_id",          accountID),
		zap.String("principal_id",        principalID),
		zap.String("resource_type",       resourceType),
		zap.String("resource_id",         resourceID),
		zap.String("requested_action",    action),
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
		log.Error("failed to marshal authorization request",
			zap.String(logger.F.ErrorKind, "marshal_error"),
			zap.Error(err),
		)
		return false, fmt.Errorf("authorization failed: %w", err)
	}

	subject := fmt.Sprintf("%s.iam.authorize", a.Env)
	if a.Env == "" {
		subject = "iam.authorize"
	}

	respData, err := a.Nats.Request(ctx, subject, reqData, 5*time.Second)
	if err != nil {
		log.Error("IAM authorization request failed",
			zap.String(logger.F.ErrorKind, "nats_request_error"),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return false, fmt.Errorf("IAM authorization request failed: %w", err)
	}

	var resp AuthorizeResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		log.Error("failed to parse IAM authorization response",
			zap.String(logger.F.ErrorKind, "decode_error"),
			zap.Error(err),
		)
		return false, fmt.Errorf("invalid IAM response: %w", err)
	}

	if resp.Allowed {
		log.Info("authorization granted")
		return true, nil
	}

	reason := "not authorized"
	if resp.Reason != nil && *resp.Reason != "" {
		reason = *resp.Reason
	}

	log.Warn("authorization denied",
		zap.String(logger.F.ErrorKind, "access_denied"),
		zap.String("reason", reason),
	)

	return false, errors.New(reason)
}

// ---------------- TOKEN ----------------

type instanceTokenRequest struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id"`
}

type instanceTokenResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

func (a *Authorizer) GenerateMetricsToken(ctx context.Context, userID, functionID string) (string, error) {
	reqID := uuid.New().String()

	log := logger.WithContext(ctx).With(
		zap.String(logger.F.Action,     "iam.token.generate"),
		zap.String(logger.F.Domain,     "auth"),
		zap.String("auth.request_id",  reqID),
		zap.String("user_id",          userID),
		zap.String("function_id",      functionID),
	)

	req := instanceTokenRequest{
		InstanceID: functionID,
		UserID:     userID,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		log.Error("failed to marshal instance token request",
			zap.String(logger.F.ErrorKind, "marshal_error"),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to marshal instance token request: %w", err)
	}

	subject := fmt.Sprintf("%s.iam.token.generate", a.Env)
	if a.Env == "" {
		subject = "iam.token.generate"
	}

	log.Debug("requesting metrics token from IAM",
		zap.String("subject", subject),
	)

	msg, err := a.Nats.Request(ctx, subject, reqData, 5*time.Second)
	if err != nil {
		log.Error("IAM token request failed",
			zap.String(logger.F.ErrorKind, "nats_request_error"),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return "", fmt.Errorf("NATS request failed: %w", err)
	}

	var resp instanceTokenResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		log.Error("failed to unmarshal instance token response",
			zap.String(logger.F.ErrorKind, "decode_error"),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to unmarshal instance token response: %w", err)
	}

	if resp.Error != "" {
		log.Error("IAM service returned error on token generation",
			zap.String(logger.F.ErrorKind, "iam_error"),
			zap.String("iam_error", resp.Error),
		)
		return "", fmt.Errorf("IAM service error: %s", resp.Error)
	}

	if resp.Token == "" {
		log.Error("IAM service returned an empty token",
			zap.String(logger.F.ErrorKind, "empty_response"),
		)
		return "", fmt.Errorf("IAM service returned an empty token")
	}

	log.Info("metrics token generated successfully")
	return resp.Token, nil
}