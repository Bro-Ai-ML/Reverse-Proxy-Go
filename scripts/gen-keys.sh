#!/usr/bin/env sh
# Generate an RSA keypair for the gateway's JWT signing (dev/local use).
set -eu
DIR="${1:-go-backend/services/gateway/secrets/keys}"
mkdir -p "$DIR"
openssl genrsa -out "$DIR/private.pem" 2048
openssl rsa -in "$DIR/private.pem" -pubout -out "$DIR/public.pem"
echo "Keys written to $DIR"
