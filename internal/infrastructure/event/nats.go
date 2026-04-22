package event

import (
	"context"
	"time"

	"lambda/internal/utils/logger"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
)

type NatsClient struct {
	Conn *nats.Conn
	log  *zap.Logger
}

func NewNatsClient(nc *nats.Conn) *NatsClient {
	return &NatsClient{
		Conn: nc,
		log: logger.Log.With(
			zap.String(logger.F.Domain, "nats"),
		),
	}
}

func (n *NatsClient) Request(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error) {
	log := logger.WithContext(ctx)

	log.Debug("sending request",
		zap.String(logger.F.Action, "nats.request"),
		zap.String("subject", subject),
		zap.Duration("timeout", timeout),
	)

	msg := nats.NewMsg(subject)
	msg.Data = data
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))

	resp, err := n.Conn.RequestMsgWithContext(ctx, msg)
	if err != nil {
		log.Error("request failed",
			zap.String(logger.F.Action,    "nats.request"),
			zap.String(logger.F.ErrorKind, "nats_request_error"),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return nil, err
	}

	log.Debug("response received",
		zap.String(logger.F.Action, "nats.request"),
		zap.String("subject", subject),
		zap.Int("response_bytes", len(resp.Data)),
	)

	return resp.Data, nil
}

func (n *NatsClient) Subscribe(subject string, cb nats.MsgHandler) (*nats.Subscription, error) {
	n.log.Info("subscribing",
		zap.String(logger.F.Action, "nats.subscribe"),
		zap.String("subject", subject),
	)

	wrapped := func(m *nats.Msg) {
		if m.Header == nil {
			m.Header = make(nats.Header)
		}

		ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(m.Header))
		log := logger.WithContext(ctx)

		log.Debug("message received",
			zap.String(logger.F.Action, "nats.receive"),
			zap.String("subject", m.Subject),
			zap.Int("payload_bytes", len(m.Data)),
		)

		cb(m)
	}

	sub, err := n.Conn.Subscribe(subject, wrapped)
	if err != nil {
		n.log.Error("subscribe failed",
			zap.String(logger.F.Action,    "nats.subscribe"),
			zap.String(logger.F.ErrorKind, "nats_subscribe_error"),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return nil, err
	}

	return sub, nil
}