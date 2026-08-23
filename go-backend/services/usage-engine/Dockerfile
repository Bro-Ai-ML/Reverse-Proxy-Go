# Build from the go-backend root:
#   docker build -f services/usage-engine/Dockerfile .
FROM golang:1.24-alpine AS builder
ENV GOWORK=off
WORKDIR /src
COPY . .
WORKDIR /src/services/usage-engine
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/usage-engine .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/usage-engine /app/usage-engine
EXPOSE 8082
ENTRYPOINT ["/app/usage-engine"]
