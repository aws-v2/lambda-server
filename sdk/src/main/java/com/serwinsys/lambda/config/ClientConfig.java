package com.serwinsys.lambda.config;

import java.time.Duration;

public class ClientConfig {
    private final String baseUrl;
    private final CredentialsProvider credentialsProvider;
    private final Duration timeout;

    public ClientConfig(String baseUrl, CredentialsProvider credentialsProvider, Duration timeout) {
        this.baseUrl = baseUrl;
        this.credentialsProvider = credentialsProvider;
        this.timeout = timeout;
    }

    public String getBaseUrl() {
        return baseUrl;
    }

    public CredentialsProvider getCredentialsProvider() {
        return credentialsProvider;
    }

    public Duration getTimeout() {
        return timeout;
    }
}
