package com.serwinsys.lambda.config;

/**
 * Holds Serwin authentication credentials: Access Key ID and Secret Access Key.
 */
public class SerwinCredentials {
    private final String accessKeyId;
    private final String secretAccessKey;

    public SerwinCredentials(String accessKeyId, String secretAccessKey) {
        if (accessKeyId == null || accessKeyId.isBlank()) {
            throw new IllegalArgumentException("Access Key ID cannot be null or empty");
        }
        if (secretAccessKey == null || secretAccessKey.isBlank()) {
            throw new IllegalArgumentException("Secret Access Key cannot be null or empty");
        }
        this.accessKeyId = accessKeyId;
        this.secretAccessKey = secretAccessKey;
    }

    public String getAccessKeyId() {
        return accessKeyId;
    }

    public String getSecretAccessKey() {
        return secretAccessKey;
    }
}
