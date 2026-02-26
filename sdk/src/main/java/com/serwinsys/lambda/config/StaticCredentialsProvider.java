package com.serwinsys.lambda.config;

/**
 * A simple implementation of CredentialsProvider that holds static credentials.
 */
public class StaticCredentialsProvider implements CredentialsProvider {
    private final SerwinCredentials credentials;

    public StaticCredentialsProvider(SerwinCredentials credentials) {
        this.credentials = credentials;
    }

    @Override
    public SerwinCredentials getCredentials() {
        return credentials;
    }
}
