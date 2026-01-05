# Lambda Server

A Mini-Lambda serverless function service written in Go that integrates with NATS-based IAM system.

## Features

- Deploy and manage serverless functions
- Support for JavaScript (Goja) and Docker runtimes
- IAM authentication via NATS
- Function invocation with timeout and memory controls
- Event publishing for audit and monitoring
- PostgreSQL persistence

## Architecture

Following clean architecture principles:

```
internal/
├── domain/              # Business entities and interfaces
├── application/         # Use cases (FunctionService, InvocationService)
├── infrastructure/      # External adapters
│   ├── database/       # PostgreSQL
│   ├── event/          # NATS publisher
│   ├── repository/     # Data access
│   ├── runtime/        # JavaScript & Docker executors
│   └── storage/        # Code storage
├── middleware/         # Auth & logging
└── transport/http/     # Gin handlers & routes
```

## Prerequisites

- Go 1.23+
- PostgreSQL 14+
- NATS Server (for IAM integration)
- Docker (optional, for docker runtime)

## Setup

1. Copy environment file:
```bash
cp .env.example .env
```

2. Configure your database and NATS in `.env`

3. Install dependencies:
```bash
go mod download
```

4. Run migrations (automatic on startup) or create database:
```bash
createdb lambda_db
```

5. Start the server:
```bash
go run main.go
```

## API Endpoints

All endpoints require `x-api-key` header with format `accessKeyId:secretAccessKey`

### Create Function
```http
POST /api/v1/lambda/functions
Content-Type: application/json

{
  "name": "my-function",
  "runtime": "javascript",
  "code": "const result = event.x + event.y; result;"
}
```

### List Functions
```http
GET /api/v1/lambda/functions
```

### Invoke Function
```http
POST /api/v1/lambda/functions/:name/invoke
Content-Type: application/json

{
  "payload": {
    "x": 10,
    "y": 20
  }
}
```

### Delete Function
```http
DELETE /api/v1/lambda/functions/:name
```

## Runtimes

### JavaScript (Goja)
- Runs JavaScript code in an embedded VM
- Timeout and memory controls
- Access to `event` variable with payload

Example:
```javascript
const result = event.x + event.y;
console.log("Sum:", result);
result;
```

### Docker
- Runs code in Node.js Alpine container
- Isolated execution environment
- Automatic cleanup

## IAM Integration

The server validates API keys via NATS on the `iam.auth.validate` subject. Ensure your IAM service is running and listening on this subject.

## Event Publishing

On every function invocation, an event is published to `lambda.events.invoked` with:
- invocationId
- functionId
- functionName
- status (SUCCESS/FAILED/TIMEOUT)
- durationMs
- error (if any)

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8083 | Server port |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_NAME | lambda_db | Database name |
| NATS_URL | nats://localhost:4222 | NATS server URL |
| CODE_STORAGE_PATH | ./lambda-code | Code storage directory |
