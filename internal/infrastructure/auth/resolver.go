package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"

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
	log := logger.WithContext(ctx)

	// 1. Try local cache
	key, err := r.db.GetApiKey(accessKeyID)
	if err != nil {
		log.Error("failed to get API key from cache",
			zap.String(logger.F.Action,    "apikey.resolve"),
			zap.String(logger.F.Domain,    "auth"),
			zap.String(logger.F.ErrorKind, "cache_read_error"),
			zap.String("access_key_id",    accessKeyID),
			zap.Error(err),
		)
		return nil, err
	}
	if key != nil {
		log.Info("API key resolved from cache",
			zap.String(logger.F.Action, "apikey.resolve"),
			zap.String(logger.F.Domain, "auth"),
			zap.String("access_key_id", accessKeyID),
		)
		return key, nil
	}

	// 2. Fallback to NATS request-reply
	log.Info("API key not found in cache, resolving via NATS",
		zap.String(logger.F.Action, "apikey.resolve"),
		zap.String(logger.F.Domain, "auth"),
		zap.String("access_key_id", accessKeyID),
	)

	req := resolveRequest{AccessKeyID: accessKeyID}
	data, _ := json.Marshal(req)

	subject := fmt.Sprintf("%s.lambda.%s.apikey.resolve",
		strings.ToLower(r.env),
		strings.ToLower(r.version))

	respData, err := r.natsClient.Request(ctx, subject, data, 2*time.Second)
	if err != nil {
		log.Error("failed to resolve API key via NATS",
			zap.String(logger.F.Action,    "apikey.resolve"),
			zap.String(logger.F.Domain,    "auth"),
			zap.String(logger.F.ErrorKind, "nats_request_error"),
			zap.String("subject",          subject),
			zap.String("access_key_id",    accessKeyID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to resolve API key via NATS: %w", err)
	}

	var resp resolveResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		log.Error("failed to decode auth-server response",
			zap.String(logger.F.Action,    "apikey.resolve"),
			zap.String(logger.F.Domain,    "auth"),
			zap.String(logger.F.ErrorKind, "decode_error"),
			zap.String("access_key_id",    accessKeyID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to decode auth-server response: %w", err)
	}

	if resp.UserID == "" {
		log.Warn("auth-server returned empty user ID",
			zap.String(logger.F.Action,    "apikey.resolve"),
			zap.String(logger.F.Domain,    "auth"),
			zap.String(logger.F.ErrorKind, "invalid_response"),
			zap.String("access_key_id",    accessKeyID),
		)
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
		log.Warn("failed to cache resolved API key",
			zap.String(logger.F.Action,    "apikey.resolve"),
			zap.String(logger.F.Domain,    "auth"),
			zap.String(logger.F.ErrorKind, "cache_write_error"),
			zap.String("access_key_id",    accessKeyID),
			zap.Error(err),
		)
	}

	log.Info("API key resolved successfully via NATS",
		zap.String(logger.F.Action, "apikey.resolve"),
		zap.String(logger.F.Domain, "auth"),
		zap.String("access_key_id", accessKeyID),
	)

	return &newKey, nil
}