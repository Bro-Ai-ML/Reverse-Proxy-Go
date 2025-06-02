# syntax=docker/dockerfile:1
# Build context is project root for workspace compatibility
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY services/usage-engine/. .
RUN go mod download
RUN CGO_ENABLED=0 go build -o usage-engine main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/usage-engine .
EXPOSE 8082
ENTRYPOINT ["./usage-engine"] 