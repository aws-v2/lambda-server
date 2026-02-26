package com.serwinsys.lambda.config;

/**
 * Interface for providing Serwin credentials.
 */
public interface CredentialsProvider {
    /**
     * @return The credentials to use for authentication.
     */
    SerwinCredentials getCredentials();
}
