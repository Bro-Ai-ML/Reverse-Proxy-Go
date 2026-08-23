# Build from the go-backend root:
#   docker build -f services/gateway/Dockerfile .
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache openssl
ENV GOWORK=off
WORKDIR /src
COPY . .
WORKDIR /src/services/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway .
# Generate a demo RSA keypair so the image boots out of the box. For real
# deployments, mount your own keys and point JWT_PRIVATE_KEY_PATH /
# JWT_PUBLIC_KEY_PATH at them.
RUN mkdir -p /out/secrets/keys \
    && openssl genrsa -out /out/secrets/keys/private.pem 2048 \
    && openssl rsa -in /out/secrets/keys/private.pem -pubout -out /out/secrets/keys/public.pem

FROM alpine:latest
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/gateway ./gateway
COPY --from=builder /out/secrets ./secrets
EXPOSE 8080
ENTRYPOINT ["./gateway"]
