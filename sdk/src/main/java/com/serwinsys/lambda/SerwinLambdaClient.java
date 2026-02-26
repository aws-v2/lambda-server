package com.serwinsys.lambda;

import com.serwinsys.lambda.config.ClientConfig;
import com.serwinsys.lambda.exceptions.LambdaException;
import com.serwinsys.lambda.internal.HttpExecutor;
import com.serwinsys.lambda.config.*;
import com.serwinsys.lambda.models.*;

import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

/**
 * Main client for interacting with the Serwin Lambda platform.
 *
 * <p>
 * Every operation is available in two flavours:
 * <ul>
 * <li><b>By name</b> – use the function's short name, e.g.
 * {@code "my-function"}</li>
 * <li><b>By ARN</b> – use the full ARN, e.g.
 * {@code "arn:serwin:lambda:us-east-1:123456789012:function:my-function"}</li>
 * </ul>
 */
public class SerwinLambdaClient {
    private final HttpExecutor executor;

    public SerwinLambdaClient(String baseUrl, SerwinCredentials credentials) {
        this(baseUrl, new StaticCredentialsProvider(credentials), Duration.ofSeconds(30));
    }

    public SerwinLambdaClient(String baseUrl, CredentialsProvider provider) {
        this(baseUrl, provider, Duration.ofSeconds(30));
    }

    public SerwinLambdaClient(String baseUrl, SerwinCredentials credentials, Duration timeout) {
        this(baseUrl, new StaticCredentialsProvider(credentials), timeout);
    }

    public SerwinLambdaClient(String baseUrl, CredentialsProvider provider, Duration timeout) {
        ClientConfig config = new ClientConfig(baseUrl, provider, timeout);
        this.executor = new HttpExecutor(config);
    }

    // ──────────────────────────────────────────────────────────────────────────
    // Name-based operations (original API – unchanged)
    // ──────────────────────────────────────────────────────────────────────────

    /**
     * Invokes a lambda function by name synchronously, streaming SSE until
     * completion.
     *
     * @param functionName Name of the function to invoke
     * @param payload      Payload to send to the function
     * @return The invocation result as a String (raw JSON)
     */
    public InvokeResponse invoke(String functionName, Object payload) {
        return executor.executeInvoke("/api/v1/lambda/functions/" + functionName + "/invoke", payload);
    }

    /**
     * Lists all functions available to the authenticated user.
     */
    public List<FunctionMetadata> listFunctions() {
        FunctionMetadata[] functions = executor.get("/api/v1/lambda/functions", FunctionMetadata[].class);
        return functions != null ? Arrays.asList(functions) : List.of();
    }

    /**
     * Retrieves metadata for a specific function by name.
     */
    public FunctionMetadata getFunction(String name) {
        return executor.get("/api/v1/lambda/functions/" + name, FunctionMetadata.class);
    }

    /**
     * Retrieves metrics for a specific function by name (last 24h).
     */
    public MetricsResponse getMetrics(String name) {
        return executor.get("/api/v1/lambda/functions/" + name + "/metrics", MetricsResponse.class);
    }

    /**
     * Updates the code of an existing function by name.
     *
     * @param name       Name of the function
     * @param sourceCode New source code (for script-based lambdas)
     */
    public void updateCode(String name, String sourceCode) {
        executor.patch("/api/v1/lambda/functions/" + name + "/code", Map.of("code", sourceCode));
    }

    /**
     * Updates the configuration (memory, timeout, description) of an existing
     * function by name.
     */
    public void updateConfig(String name, Map<String, Object> config) {
        executor.patch("/api/v1/lambda/functions/" + name + "/config", config);
    }

    /**
     * Registers a new function.
     * Note: This SDK currently supports registration via metadata.
     * Binary uploads should be handled via multipart/form-data.
     */
    public void registerFunction(FunctionMetadata metadata) {
        executor.post("/api/v1/lambda/functions", metadata, Void.class);
    }

    // ──────────────────────────────────────────────────────────────────────────
    // ARN-based operations
    // ──────────────────────────────────────────────────────────────────────────

    /**
     * Invokes a lambda function by ARN synchronously.
     *
     * @param arn     Full Lambda ARN, e.g.
     *                {@code arn:serwin:lambda:us-east-1:123:function:myFn}
     * @param payload Payload to send to the function
     * @return The invocation result as a String (raw JSON)
     * @throws IllegalArgumentException if the ARN is invalid
     */
    public InvokeResponse invokeByArn(String arn, Object payload) {
        validateArn(arn);
        return executor.executeInvoke("/api/v1/lambda/functions/arn/" + arn + "/invoke", payload);
    }

    /**
     * Retrieves metadata for a function identified by ARN.
     *
     * @param arn Full Lambda ARN
     * @return Function metadata
     * @throws IllegalArgumentException if the ARN is invalid
     */
    public FunctionMetadata getFunctionByArn(String arn) {
        validateArn(arn);
        return executor.get("/api/v1/lambda/functions/arn/" + arn, FunctionMetadata.class);
    }

    /**
     * Retrieves metrics for a function identified by ARN (last 24h).
     *
     * @param arn Full Lambda ARN
     * @return Metrics response
     * @throws IllegalArgumentException if the ARN is invalid
     */
    public MetricsResponse getMetricsByArn(String arn) {
        validateArn(arn);
        return executor.get("/api/v1/lambda/functions/arn/" + arn + "/metrics", MetricsResponse.class);
    }

    /**
     * Updates the code of a function identified by ARN.
     *
     * @param arn        Full Lambda ARN
     * @param sourceCode New source code (for script-based lambdas)
     * @throws IllegalArgumentException if the ARN is invalid
     */
    public void updateCodeByArn(String arn, String sourceCode) {
        validateArn(arn);
        executor.patch("/api/v1/lambda/functions/arn/" + arn + "/code", Map.of("code", sourceCode));
    }

    /**
     * Updates the configuration of a function identified by ARN.
     *
     * @param arn    Full Lambda ARN
     * @param config Configuration map (memory, timeout, description)
     * @throws IllegalArgumentException if the ARN is invalid
     */
    public void updateConfigByArn(String arn, Map<String, Object> config) {
        validateArn(arn);
        executor.patch("/api/v1/lambda/functions/arn/" + arn + "/config", config);
    }

    // ──────────────────────────────────────────────────────────────────────────
    // Convenience overloads accepting a LambdaArn value object
    // ──────────────────────────────────────────────────────────────────────────

    /** @see #invokeByArn(String, Object) */
    public InvokeResponse invokeByArn(LambdaArn arn, Object payload) {
        return invokeByArn(arn.toString(), payload);
    }

    /** @see #getFunctionByArn(String) */
    public FunctionMetadata getFunctionByArn(LambdaArn arn) {
        return getFunctionByArn(arn.toString());
    }

    /** @see #getMetricsByArn(String) */
    public MetricsResponse getMetricsByArn(LambdaArn arn) {
        return getMetricsByArn(arn.toString());
    }

    /** @see #updateCodeByArn(String, String) */
    public void updateCodeByArn(LambdaArn arn, String sourceCode) {
        updateCodeByArn(arn.toString(), sourceCode);
    }

    /** @see #updateConfigByArn(String, Map) */
    public void updateConfigByArn(LambdaArn arn, Map<String, Object> config) {
        updateConfigByArn(arn.toString(), config);
    }

    // ──────────────────────────────────────────────────────────────────────────
    // Internal helpers
    // ──────────────────────────────────────────────────────────────────────────

    private void validateArn(String arn) {
        if (!LambdaArn.isValid(arn)) {
            throw new LambdaException("Invalid Lambda ARN: '" + arn
                    + "'. Expected format: arn:serwin:lambda:<region>:<accountId>:function:<name>");
        }
    }
}
