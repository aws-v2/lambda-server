package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"lambda/internal/domain"
	"time"

	"github.com/dop251/goja"
)

type JavaScriptExecutor struct{}

func NewJavaScriptExecutor() *JavaScriptExecutor {
	return &JavaScriptExecutor{}
}

func (e *JavaScriptExecutor) GetRuntime() domain.Runtime {
	return domain.RuntimeJavaScript
}

func (e *JavaScriptExecutor) Execute(ctx context.Context, function *domain.Function, payload map[string]interface{}) (interface{}, error) {
	vm := goja.New()

	done := make(chan error, 1)
	var result goja.Value
	var execErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("runtime panic: %v", r)
				done <- execErr
			}
		}()

		payloadJSON, _ := json.Marshal(payload)
		eventScript := fmt.Sprintf("var event = %s;", string(payloadJSON))
		if _, err := vm.RunString(eventScript); err != nil {
			execErr = fmt.Errorf("failed to inject event: %w", err)
			done <- execErr
			return
		}

		result, execErr = vm.RunString(function.Code)
		done <- execErr
	}()

	timeout := time.Duration(function.TimeoutMs) * time.Millisecond
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("execution cancelled")
	case <-time.After(timeout):
		return nil, fmt.Errorf("execution timeout after %dms", function.TimeoutMs)
	case err := <-done:
		if err != nil {
			return nil, err
		}
	}

	if result == nil || goja.IsUndefined(result) {
		return nil, nil
	}

	return result.Export(), nil
}
