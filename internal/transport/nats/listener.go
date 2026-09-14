package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	normalLog "log"

	"lambda/internal/application"
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
		fmt.Sprintf("%s.lambda.scale.out", env),
		fmt.Sprintf("%s.lambda.scale.in", env),
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
func StartInvokeEventServer(nc *event.NatsClient, invokeService *application.InvokeService) error {
	subject := fmt.Sprintf("%s.lambda.task.invoke", nc.NatsPrefix)

	queueGroup := "lambda-invoke-listeners"

	// Use QueueSubscribe to ensure only one instance of lambda-server processes each scaling event.
	_, err := nc.Conn.QueueSubscribe(subject, queueGroup, func(m *github_nats.Msg) {
		handleInvokeEvent(nc, m, invokeService)
	})
	if err != nil {
		logger.Log.Error("Failed to subscribe to invoke event subject", zap.String("subject", subject), zap.Error(err))
		return err
	}
	logger.Log.Info("Subscribed to invoke event subject", zap.String("subject", subject), zap.String("queue", queueGroup))

	return nil
}
// type InvokeFunctionRequest struct {
//     UserId    string      `json:"user_id"`
//     RequestId string      `json:"request_id"`
//     Logger    *zap.Logger `json:"logger"`
//     TaskID    string      `json:"task_id"`

//     FunctionId string `json:"function_id"`
// }
func handleInvokeEvent(nc *event.NatsClient, m *github_nats.Msg, invokeService *application.InvokeService) {

	var event dto.InvokeFunctionRequest
	if err := json.Unmarshal(m.Data, &event); err != nil {
		logger.Log.Error("Failed to unmarshal scale event", zap.Error(err))
		return
	}
	cont := context.Background()


	log := logger.WithContext(cont).With(
		zap.String(logger.F.Action, "lambda.invoke"),
		zap.String(logger.F.Domain, "lambda"),
	)

	event.Logger=log


	resp, err := invokeService.Invoke(&cont, event)

	if err != nil {
		return
	}
	jsonData, err := json.Marshal(resp)

	if err != nil {
		return
	}

	nc.Conn.Publish(m.Reply, jsonData)
	normalLog.Printf("\n[NATS-SUB] Successfully replied: %v",resp)

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
