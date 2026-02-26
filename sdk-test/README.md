# Serwin Lambda SDK – Integration Test Suite

A standalone test program that exercises every aspect of the `serwin-lambda-sdk` against a live Lambda server.

## Prerequisites

| Requirement | Version |
|-------------|---------|
| Java        | 17+     |
| Maven       | 3.8+    |
| Lambda server | running |

## Setup

### 1. Install the SDK into your local Maven repository

```bash
cd ../sdk
mvn install -q
```

### 2. Set environment variables

| Variable | Required | Example |
|----------|----------|---------|
| `LAMBDA_BASE_URL` | ✅ | `http://localhost:8080` |
| `LAMBDA_API_KEY`  | ✅ | `your-api-key-here` |
| `LAMBDA_TEST_FUNCTION` | optional | `hello-world` |
| `LAMBDA_TEST_ARN` | optional | `arn:serwin:lambda:us-east-1:123456789012:function:hello-world` |

If `LAMBDA_TEST_FUNCTION` / `LAMBDA_TEST_ARN` are not set, tests that require a live function are skipped gracefully.

### 3. Run

```bash
cd sdk-test
mvn compile exec:java
```

## Test Sections

| # | Section | What it tests |
|---|---------|---------------|
| 1 | Health Check | Raw HTTP GET `/api/v1/lambda/health` |
| 2 | List Functions | `client.listFunctions()` |
| 3 | Get Function by Name | `client.getFunction(name)` |
| 4 | Get Function by ARN | `client.getFunctionByArn(arn)` |
| 5 | Invoke by Name | `client.invoke(name, payload)` |
| 6 | Invoke by ARN | `client.invokeByArn(arn, payload)` |
| 7 | Metrics by Name | `client.getMetrics(name)` |
| 8 | Metrics by ARN | `client.getMetricsByArn(arn)` |
| 9 | Update Config by Name | `client.updateConfig(name, config)` |
| 10 | Update Config by ARN | `client.updateConfigByArn(arn, config)` |
| 11 | ARN Parsing & Validation | `LambdaArn.parse()`, `LambdaArn.isValid()` |
| 12 | Error Handling | 404 on missing function, local ARN validation rejection |

## ARN Format

```
arn:serwin:lambda:<region>:<accountId>:function:<functionName>
```

Example: `arn:serwin:lambda:us-east-1:123456789012:function:my-function`

Use `LambdaArn.parse(rawArn)` to validate and decompose an ARN, or `LambdaArn.isValid(rawArn)` for a boolean check.
