# Build from the go-backend root:
#   docker build -f services/customer-service/Dockerfile .
FROM golang:1.24-alpine AS builder
ENV GOWORK=off
WORKDIR /src
COPY . .
WORKDIR /src/services/customer-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app .

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /out/app /app/service
 8080
ENTRYPOINT ["/app/service"]
