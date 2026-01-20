package event

import (
	"context"
	"time"

	"lambda/internal/logger"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type NatsClient struct {
	Conn *nats.Conn
}

func NewNatsClient(nc *nats.Conn) *NatsClient {
	return &NatsClient{Conn: nc}
}

func (n *NatsClient) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error) {
	logger.ForContext(ctx).Debug("Sending NATS request", zap.String("subject", subject), zap.Duration("timeout", timeout))
	
	msg := nats.NewMsg(subject)
	msg.Data = data
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	
	// Inject trace context into NATS headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))

	resp, err := n.Conn.RequestMsgWithContext(ctx, msg)
	if err != nil {
		logger.ForContext(ctx).Error("NATS request failed", zap.String("subject", subject), zap.Error(err))
		return nil, err
	}
	return resp.Data, nil
}

func (n *NatsClient) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	// For subscription, we wrap the callback to extract context if present
	wrappedCb := func(m *nats.Msg) {
		if m.Header == nil {
			m.Header = make(nats.Header)
		}
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(m.Header))
		logger.ForContext(ctx).Debug("NATS message received", zap.String("subject", m.Subject))
		m.Header.Set("trace_context_extracted", "true") // Hint for internal use if needed
		
		// Note: The callback signature doesn't take context, so we might need a custom handler type 
		// if we want to pass context down. For now, the handler can extract it from headers or use headers in logs.
		cb(m)
	}
	return n.Conn.Subscribe(subject, wrappedCb)
}
