# Step 1: Build the application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for private dependencies if needed
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go

# Step 2: Final minimal image
FROM alpine:latest

# Add non-root user for security
RUN adduser -D -u 10001 appuser

WORKDIR /app

# Install certificates
RUN apk --no-cache add ca-certificates
RUN mkdir -p /app/docs
# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Copy migrations so they are available for RunMigrations()
COPY --from=builder /app/internal/infrastructure/migrations ./internal/infrastructure/migrations

# Copy docs from builder stage
COPY --from=builder /app/docs ./docs

# Create storage directory and set permissions
RUN mkdir -p storage && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose the application port
EXPOSE 8089

# Run the binary
ENTRYPOINT ["./main"]
