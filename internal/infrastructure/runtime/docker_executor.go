package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"lambda/internal/domain"
	"os/exec"
	"strings"
	"time"
)

type DockerExecutor struct{}

func NewDockerExecutor() *DockerExecutor {
	return &DockerExecutor{}
}

func (e *DockerExecutor) GetRuntime() domain.Runtime {
	return domain.RuntimeDocker
}

func (e *DockerExecutor) Execute(ctx context.Context, function *domain.Function, payload map[string]interface{}) (interface{}, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	containerName := fmt.Sprintf("lambda-%s-%d", function.ID, time.Now().Unix())

	script := fmt.Sprintf(`
const event = %s;
%s
	`, string(payloadJSON), function.Code)

	timeout := time.Duration(function.TimeoutMs) * time.Millisecond
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx,
		"docker", "run", "--rm",
		"--name", containerName,
		"--memory", fmt.Sprintf("%dm", function.MemoryMB),
		"node:18-alpine",
		"node", "-e", script,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			exec.Command("docker", "rm", "-f", containerName).Run()
			return nil, fmt.Errorf("execution timeout after %dms", function.TimeoutMs)
		}
		return nil, fmt.Errorf("execution failed: %s - %w", string(output), err)
	}

	resultStr := strings.TrimSpace(string(output))
	if resultStr == "" {
		return nil, nil
	}

	var result interface{}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		return resultStr, nil
	}

	return result, nil
}
