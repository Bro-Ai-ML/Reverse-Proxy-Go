# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/server

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary and config files
COPY --from=builder /app/app .
COPY --from=builder /app/.env* ./
COPY --from=builder /app/migrations ./migrations

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
CMD ["./app"]
