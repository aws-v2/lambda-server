package com.serwinsys.lambda.models;

/**
 * Represents a Serwin Lambda ARN (Amazon Resource Name equivalent).
 * <p>
 * Format:
 * {@code arn:serwin:lambda:<region>:<accountId>:function:<functionName>}
 * <p>
 * Example:
 * {@code arn:serwin:lambda:us-east-1:123456789012:function:my-function}
 */
public class LambdaArn {

    private static final String PREFIX = "arn:serwin:lambda:";
    private static final String FUNCTION_SEGMENT = ":function:";

    private final String region;
    private final String accountId;
    private final String functionName;
    private final String raw;

    private LambdaArn(String raw, String region, String accountId, String functionName) {
        this.raw = raw;
        this.region = region;
        this.accountId = accountId;
        this.functionName = functionName;
    }

    /**
     * Parses a raw ARN string into a {@link LambdaArn}.
     *
     * @param arn the raw ARN string
     * @return parsed LambdaArn
     * @throws IllegalArgumentException if the ARN format is invalid
     */
    public static LambdaArn parse(String arn) {
        if (arn == null || !arn.startsWith(PREFIX)) {
            throw new IllegalArgumentException(
                    "Invalid Lambda ARN – must start with '" + PREFIX + "'. Got: " + arn);
        }

        // After "arn:serwin:lambda:" we expect: <region>:<accountId>:function:<name>
        String remainder = arn.substring(PREFIX.length()); // region:accountId:function:name
        int funcIdx = remainder.indexOf(FUNCTION_SEGMENT);
        if (funcIdx < 0) {
            throw new IllegalArgumentException(
                    "Invalid Lambda ARN – missing ':function:' segment. Got: " + arn);
        }

        String regionAndAccount = remainder.substring(0, funcIdx);
        int colonIdx = regionAndAccount.indexOf(':');
        if (colonIdx < 0) {
            throw new IllegalArgumentException(
                    "Invalid Lambda ARN – missing region or accountId. Got: " + arn);
        }

        String region = regionAndAccount.substring(0, colonIdx);
        String accountId = regionAndAccount.substring(colonIdx + 1);
        String functionName = remainder.substring(funcIdx + FUNCTION_SEGMENT.length());

        if (region.isBlank() || accountId.isBlank() || functionName.isBlank()) {
            throw new IllegalArgumentException(
                    "Invalid Lambda ARN – region, accountId and functionName must all be non-empty. Got: " + arn);
        }

        return new LambdaArn(arn, region, accountId, functionName);
    }

    /**
     * Returns true if the given string is a valid Serwin Lambda ARN.
     */
    public static boolean isValid(String arn) {
        try {
            parse(arn);
            return true;
        } catch (IllegalArgumentException e) {
            return false;
        }
    }

    public String getRegion() {
        return region;
    }

    public String getAccountId() {
        return accountId;
    }

    public String getFunctionName() {
        return functionName;
    }

    /** Returns the raw ARN string. */
    @Override
    public String toString() {
        return raw;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o)
            return true;
        if (!(o instanceof LambdaArn))
            return false;
        return raw.equals(((LambdaArn) o).raw);
    }

    @Override
    public int hashCode() {
        return raw.hashCode();
    }
}
