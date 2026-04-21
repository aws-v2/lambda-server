package nats

import (
	"encoding/json"
	"fmt"
	"os"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/database"
	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"

	github_nats "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func StartScaleEventServer(nc *event.NatsClient, db *database.DB) error {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	queueGroup := "lambda-scale-listeners"
	subjects := []string{
		fmt.Sprintf("%s.lambda.v1.scale.out", env),
		fmt.Sprintf("%s.lambda.v1.scale.in", env),
	}

	for _, subject := range subjects {
		// Use QueueSubscribe to ensure only one instance of lambda-server processes each scaling event.
		_, err := nc.Conn.QueueSubscribe(subject, queueGroup, func(m *github_nats.Msg) {
			handleScaleEvent(m, db)
		})
		if err != nil {
			logger.Log.Error("Failed to subscribe to scale event subject", zap.String("subject", subject), zap.Error(err))
			return err
		}
		logger.Log.Info("Subscribed to scale event subject", zap.String("subject", subject), zap.String("queue", queueGroup))
	}

	return nil
}

func handleScaleEvent(m *github_nats.Msg, db *database.DB) {
	var event dto.LambdaScaleEvent
	if err := json.Unmarshal(m.Data, &event); err != nil {
		logger.Log.Error("Failed to unmarshal scale event", zap.Error(err))
		return
	}

	l := logger.Log.With(
		zap.String("function_id", event.FunctionID),
		zap.String("tenant_id", event.TenantID),
		zap.String("action", event.Action),
		zap.String("reason", event.Reason),
		zap.Float64("metric_value", event.Value),
	)

	l.Info("Received Lambda scale event")

	fn, err := db.GetFunction(event.FunctionID, event.TenantID)
	if err != nil {
		l.Error("Failed to fetch function for scaling event", zap.Error(err))
		return
	}

	currentConcurrency := fn.ProvisionedConcurrency
	newConcurrency := currentConcurrency

	if event.Action == "INCREASE_PROVISIONED_CONCURRENCY" {
		newConcurrency++
	} else if event.Action == "DECREASE_PROVISIONED_CONCURRENCY" {
		newConcurrency--
		if newConcurrency < 0 {
			newConcurrency = 0
		}
	} else {
		l.Warn("Unknown scaling action received")
		return
	}

	if currentConcurrency != newConcurrency {
		err = db.UpdateProvisionedConcurrency(fn.Name, fn.UserID, newConcurrency)
		if err != nil {
			l.Error("Failed to update provisioned concurrency in database", zap.Error(err))
			return
		}
		l.Info("Successfully updated provisioned concurrency",
			zap.Int("old_concurrency", currentConcurrency),
			zap.Int("new_concurrency", newConcurrency),
		)
	} else {
		l.Info("Provisioned concurrency unchanged", zap.Int("concurrency", currentConcurrency))
	}
}
