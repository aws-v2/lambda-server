package auth_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"

	"lambda/internal/infrastructure/auth"
	"lambda/internal/infrastructure/event"
	"lambda/internal/logger"
)

func init() {
	logger.Init()
}

func TestAuthorize_Allowed(t *testing.T) {
	// Start NATS Server manually or mock it
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skip("NATS Server not running, skipping test")
	}
	defer nc.Close()

	natsClient := &event.NatsClient{Conn: nc}
	authorizer := auth.NewAuthorizer(natsClient, "test")

	// Mock IAM responder
	sub, err := nc.Subscribe("test.iam.v1.authorize", func(msg *nats.Msg) {
		var req auth.AuthorizeRequest
		json.Unmarshal(msg.Data, &req)

		resp := auth.AuthorizeResponse{
			RequestID: req.RequestID,
			Allowed:   true,
			Reason:    nil,
		}
		respData, _ := json.Marshal(resp)
		msg.Respond(respData)
	})
	assert.NoError(t, err)
	defer sub.Unsubscribe()

	allowed, err := authorizer.Authorize(context.Background(), "user-123", "lambda-123", "s3", "bucket/alice-images/*", "PutObject")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestAuthorize_Denied(t *testing.T) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skip("NATS Server not running, skipping test")
	}
	defer nc.Close()

	natsClient := &event.NatsClient{Conn: nc}
	authorizer := auth.NewAuthorizer(natsClient, "test")

	// Mock IAM responder
	reasonStr := "Not authorized"
	sub, err := nc.Subscribe("test.iam.v1.authorize", func(msg *nats.Msg) {
		var req auth.AuthorizeRequest
		json.Unmarshal(msg.Data, &req)

		resp := auth.AuthorizeResponse{
			RequestID: req.RequestID,
			Allowed:   false,
			Reason:    &reasonStr,
		}
		respData, _ := json.Marshal(resp)
		msg.Respond(respData)
	})
	assert.NoError(t, err)
	defer sub.Unsubscribe()

	allowed, err := authorizer.Authorize(context.Background(), "user-123", "lambda-123", "s3", "bucket/alice-images/*", "PutObject")
	assert.Error(t, err)
	assert.False(t, allowed)
	assert.True(t, strings.Contains(err.Error(), "Not authorized"))
}
