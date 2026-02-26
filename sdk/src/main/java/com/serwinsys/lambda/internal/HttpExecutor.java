package com.serwinsys.lambda.internal;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.serwinsys.lambda.config.ClientConfig;
import com.serwinsys.lambda.exceptions.LambdaException;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.Map;

public class HttpExecutor {
    private final HttpClient httpClient;
    private final ClientConfig config;
    private final ObjectMapper objectMapper;

    public HttpExecutor(ClientConfig config) {
        this.config = config;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(config.getTimeout())
                .build();
        this.objectMapper = new ObjectMapper()
                .registerModule(new JavaTimeModule())
                .configure(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS, false);
    }

    public <T> T get(String path, Class<T> responseClass) {
        HttpRequest request = buildBaseRequest(path)
                .GET()
                .build();
        return execute(request, responseClass);
    }

    public <T> T post(String path, Object body, Class<T> responseClass) {
        try {
            String jsonBody = objectMapper.writeValueAsString(body);
            HttpRequest request = buildBaseRequest(path)
                    .POST(HttpRequest.BodyPublishers.ofString(jsonBody))
                    .build();
            return execute(request, responseClass);
        } catch (IOException e) {
            throw new LambdaException("Failed to serialize request body", -1, e.getMessage());
        }
    }

    public void patch(String path, Object body) {
        try {
            String jsonBody = objectMapper.writeValueAsString(body);
            HttpRequest request = buildBaseRequest(path)
                    .method("PATCH", HttpRequest.BodyPublishers.ofString(jsonBody))
                    .build();
            execute(request, Void.class);
        } catch (IOException e) {
            throw new LambdaException("Failed to serialize request body", -1, e.getMessage());
        }
    }

    public com.serwinsys.lambda.models.InvokeResponse executeInvoke(String path, Object payload) {
        try {
            String jsonBody = objectMapper.writeValueAsString(Map.of("payload", payload));
            HttpRequest request = buildBaseRequest(path)
                    .POST(HttpRequest.BodyPublishers.ofString(jsonBody))
                    .header("Accept", "text/event-stream")
                    .build();

            HttpResponse<java.util.stream.Stream<String>> response = httpClient.send(request,
                    HttpResponse.BodyHandlers.ofLines());

            if (response.statusCode() >= 400) {
                throw new LambdaException("Invocation failed", response.statusCode(), "Check backend logs");
            }

            for (String line : (Iterable<String>) response.body()::iterator) {
                if (line.startsWith("data:")) {
                    String data = line.substring(5).trim();
                    if (data.equals("DONE"))
                        break;

                    try {
                        com.serwinsys.lambda.models.InvokeResponse invokeResponse = objectMapper.readValue(data,
                                com.serwinsys.lambda.models.InvokeResponse.class);

                        // If it's a terminal status, return it immediately
                        if ("success".equalsIgnoreCase(invokeResponse.getStatus()) ||
                                "error".equalsIgnoreCase(invokeResponse.getStatus())) {
                            return invokeResponse;
                        }
                    } catch (Exception e) {
                        // Might be a progress update or other non-json data, ignore
                    }
                }
            }

            throw new LambdaException("Invoke connection closed without a result", response.statusCode(), "");
        } catch (IOException | InterruptedException e) {
            throw new LambdaException("Failed to execute invocation", -1, e.getMessage());
        }
    }

    private HttpRequest.Builder buildBaseRequest(String path) {
        var credentials = config.getCredentialsProvider().getCredentials();
        return HttpRequest.newBuilder()
                .uri(URI.create(config.getBaseUrl() + path))
                .header("X-Access-Key-Id", credentials.getAccessKeyId())
                .header("X-Secret-Access-Key", credentials.getSecretAccessKey())
                .header("Content-Type", "application/json")
                .header("Accept", "application/json")
                .timeout(config.getTimeout());
    }

    private <T> T execute(HttpRequest request, Class<T> responseClass) {
        try {
            HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() >= 300) {
                throw new LambdaException("API request failed", response.statusCode(), response.body());
            }
            if (responseClass == Void.class || response.body() == null || response.body().isEmpty()) {
                return null;
            }
            return objectMapper.readValue(response.body(), responseClass);
        } catch (IOException | InterruptedException e) {
            throw new LambdaException("Failed to execute request", -1, e.getMessage());
        }
    }
}
