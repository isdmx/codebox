# Use the official Golang image as the base image
FROM golang:1.23-alpine as builder

# Install system dependencies needed for building
RUN apk add --no-cache git bash curl

# Set the working directory inside the container
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o codebox-server cmd/server/main.go

# Use a minimal base image for the final stage
FROM alpine:latest

# Install Docker CLI to allow the application to interact with Docker daemon
RUN apk add --no-cache docker-cli

# Create a non-root user
RUN adduser -D -s /bin/sh appuser

# Copy the binary from the builder stage
COPY --from=builder /app/codebox-server /usr/local/bin/codebox-server

# Create config directory and copy default config
RUN mkdir -p /etc/codebox
COPY config.yaml /etc/codebox/config.yaml

# Switch to non-root user
USER appuser

# Expose port for HTTP transport (if enabled)
EXPOSE 8080

# Set the entrypoint to the application
ENTRYPOINT ["codebox-server"]