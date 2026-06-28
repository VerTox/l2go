# Build stage
FROM golang:1.23-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the loginserver binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o loginserver ./cmd/loginserver

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S l2go && \
    adduser -u 1001 -S l2go -G l2go

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/loginserver .

# Copy migration files
COPY --from=builder /app/internal/loginserver/schema/ ./migrations/

# Set proper ownership
RUN chown -R l2go:l2go /app

# Switch to non-root user
USER l2go

# Set environment variables with defaults
ENV POSTGRES_HOST=postgres
ENV POSTGRES_PORT=5432
ENV POSTGRES_USER=postgres
ENV POSTGRES_PASSWORD=postgres
ENV POSTGRES_DB=l2go_login
ENV POSTGRES_MIGRATION_DIR=./migrations
ENV LOGIN_SERVER_HOST=0.0.0.0
ENV LOGIN_SERVER_PORT=2106
ENV GAME_SERVER_PORT=9014
ENV LOG_LEVEL=info
ENV AUTO_CREATE_ACCOUNTS=true

# Expose ports
EXPOSE 2106 9014

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD nc -z localhost 2106 || exit 1

# Run the binary
ENTRYPOINT ["./loginserver"]