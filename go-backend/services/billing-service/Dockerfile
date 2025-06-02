# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.work ./
COPY services/billing-service/go.mod ./
COPY services/billing-service/go.sum ./
RUN go work sync
RUN go mod download
COPY services/billing-service/. .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/server

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/app .
COPY --from=builder /app/.env* ./
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
CMD ["./app"] 