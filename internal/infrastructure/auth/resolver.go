package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/logger"

	"go.uber.org/zap"
)

type ApiKeyResolver struct {
	db         *database.DB
	natsClient *event.NatsClient
	env        string
	version    string
}

func NewApiKeyResolver(db *database.DB, natsClient *event.NatsClient, env, version string) *ApiKeyResolver {
	return &ApiKeyResolver{
		db:         db,
		natsClient: natsClient,
		env:        env,
		version:    version,
	}
}

type resolveRequest struct {
	AccessKeyID string `json:"accessKeyId"`
}

type resolveResponse struct {
	UserID        string `json:"userId"`
	SecretKeyHash string `json:"secretKeyHash"`
	Enabled       bool   `json:"enabled"`
}

func (r *ApiKeyResolver) Resolve(ctx context.Context, accessKeyID string) (*database.ApiKey, error) {
	// 1. Try local cache
	key, err := r.db.GetApiKey(accessKeyID)
	if err != nil {
		return nil, err
	}
	if key != nil {
		return key, nil
	}

	// 2. Fallback to NATS request-reply
	logger.ForContext(ctx).Info("API Key not found in cache, resolving via NATS...", zap.String("id", accessKeyID))

	req := resolveRequest{AccessKeyID: accessKeyID}
	data, _ := json.Marshal(req)

	// Subject follows: <env>.<service>.<version>.<domain>.<action_type>
	subject := fmt.Sprintf("%s.lambda.%s.apikey.resolve",
		strings.ToLower(r.env),
		strings.ToLower(r.version))

	respData, err := r.natsClient.Request(ctx, subject, data, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API key via NATS: %w", err)
	}

	var resp resolveResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode auth-server response: %w", err)
	}

	if resp.UserID == "" {
		return nil, fmt.Errorf("auth-server could not resolve API key: %s", accessKeyID)
	}

	// 3. Save to local cache
	newKey := database.ApiKey{
		AccessKeyID:   accessKeyID,
		UserID:        resp.UserID,
		SecretKeyHash: resp.SecretKeyHash,
		Enabled:       resp.Enabled,
		LastSynced:    time.Now(),
	}

	if err := r.db.SaveApiKey(newKey); err != nil {
		logger.ForContext(ctx).Warn("Failed to cache resolved API key", zap.Error(err))
	}

	return &newKey, nil
}
