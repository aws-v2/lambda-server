package com.serwinsys.lambda.models;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonIgnoreProperties(ignoreUnknown = true)
public class InvokeResponse {
    @JsonProperty("task_id")
    private String taskId;

    private String status;

    @JsonProperty("execution_result")
    private String executionResult;

    private String stdout;
    private String stderr;

    @JsonProperty("exit_code")
    private int exitCode;

    public InvokeResponse() {
    }

    public String getTaskId() {
        return taskId;
    }

    public void setTaskId(String taskId) {
        this.taskId = taskId;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getExecutionResult() {
        return executionResult;
    }

    public void setExecutionResult(String executionResult) {
        this.executionResult = executionResult;
    }

    public String getStdout() {
        return stdout;
    }

    public void setStdout(String stdout) {
        this.stdout = stdout;
    }

    public String getStderr() {
        return stderr;
    }

    public void setStderr(String stderr) {
        this.stderr = stderr;
    }

    public int getExitCode() {
        return exitCode;
    }

    public void setExitCode(int exitCode) {
        this.exitCode = exitCode;
    }

    @Override
    public String toString() {
        return "InvokeResponse{" +
                "taskId='" + taskId + '\'' +
                ", status='" + status + '\'' +
                ", executionResult='" + executionResult + '\'' +
                ", stdout='" + (stdout != null ? stdout.trim() : null) + '\'' +
                ", exitCode=" + exitCode +
                '}';
    }
}
