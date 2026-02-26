package com.serwinsys.lambda.exceptions;

public class LambdaException extends RuntimeException {
    private final int statusCode;
    private final String responseBody;

    public LambdaException(String message) {
        this(message, -1, null);
    }

    public LambdaException(String message, int statusCode, String responseBody) {
        super(message);
        this.statusCode = statusCode;
        this.responseBody = responseBody;
    }

    public int getStatusCode() {
        return statusCode;
    }

    public String getResponseBody() {
        return responseBody;
    }

    @Override
    public String toString() {
        return "LambdaException{" +
                "message='" + getMessage() + '\'' +
                ", statusCode=" + statusCode +
                ", responseBody='" + responseBody + '\'' +
                '}';
    }
}
