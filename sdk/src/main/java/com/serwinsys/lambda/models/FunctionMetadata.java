package com.serwinsys.lambda.models;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

@JsonIgnoreProperties(ignoreUnknown = true)
public class FunctionMetadata {
    @JsonProperty("Name")
    private String name;

    @JsonProperty("ARN")
    private String arn;

    @JsonProperty("Type")
    private String type;

    @JsonProperty("Image")
    private String image;

    @JsonProperty("Execution")
    private ExecutionDetails execution;

    @JsonProperty("Resources")
    private ResourceDetails resources;

    @JsonProperty("Env")
    private Map<String, String> env;

    @JsonProperty("TimeoutMS")
    private int timeoutMs;

    @JsonProperty("Description")
    private String description;

    public FunctionMetadata() {
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getArn() {
        return arn;
    }

    public void setArn(String arn) {
        this.arn = arn;
    }

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    public String getImage() {
        return image;
    }

    public void setImage(String image) {
        this.image = image;
    }

    public ExecutionDetails getExecution() {
        return execution;
    }

    public void setExecution(ExecutionDetails execution) {
        this.execution = execution;
    }

    public ResourceDetails getResources() {
        return resources;
    }

    public void setResources(ResourceDetails resources) {
        this.resources = resources;
    }

    public Map<String, String> getEnv() {
        return env;
    }

    public void setEnv(Map<String, String> env) {
        this.env = env;
    }

    public int getTimeoutMs() {
        return timeoutMs;
    }

    public void setTimeoutMs(int timeoutMs) {
        this.timeoutMs = timeoutMs;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }
}
