# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

# Copy both go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o labubu .

# Final stage
FROM debian:bullseye-slim

# Install ca-certificates
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /app/labubu /app/

# Set working directory
WORKDIR /app

# Run the binary
CMD ["./labubu"]
