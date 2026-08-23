# Build from the go-backend root:
#   docker build -f services/auth-service/Dockerfile .
FROM golang:1.24-alpine AS builder
ENV GOWORK=off
WORKDIR /src
COPY . .
WORKDIR /src/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /out/app /app/service
 8080
ENTRYPOINT ["/app/service"]
