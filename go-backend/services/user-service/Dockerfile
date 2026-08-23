# Build from the go-backend root:
#   docker build -f services/user-service/Dockerfile .
FROM golang:1.24-alpine AS builder
ENV GOWORK=off
WORKDIR /src
COPY . .
WORKDIR /src/services/user-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/user-service ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /out/user-service /app/service
# RunMigrations resolves the migrations dir from the *compile-time* source
# path (runtime.Caller), so mirror that layout at runtime.
COPY services/user-service/migrations /src/services/user-service/migrations
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/service"]
