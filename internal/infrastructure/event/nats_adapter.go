package event

import (
	"context"
	"encoding/json"
	"lambda/internal/domain"
	"log"

	"github.com/nats-io/nats.go"
)

type NatsEventPublisher struct {
	conn *nats.Conn
}

func NewNatsEventPublisher(conn *nats.Conn) *NatsEventPublisher {
	return &NatsEventPublisher{conn: conn}
}

func (n *NatsEventPublisher) PublishInvocationEvent(ctx context.Context, invocation *domain.Invocation) error {
	event := map[string]interface{}{
		"invocationId": invocation.ID,
		"functionId":   invocation.FunctionID,
		"functionName": invocation.FunctionName,
		"status":       invocation.Status,
		"startedAt":    invocation.StartedAt,
		"completedAt":  invocation.CompletedAt,
		"durationMs":   invocation.DurationMs,
		"error":        invocation.Error,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if err := n.conn.Publish("lambda.events.invoked", data); err != nil {
		log.Printf("Failed to publish invocation event: %v", err)
		return err
	}

	log.Printf("Published invocation event for function: %s, status: %s", invocation.FunctionName, invocation.Status)
	return nil
}
