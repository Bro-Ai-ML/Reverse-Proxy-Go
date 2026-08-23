# 🚀 Reverse-Proxy-Go — Stripe Microservices Ecosystem

A modular Stripe billing platform in Go: an authenticating **API gateway with
reverse-proxy routing**, payment/customer/webhook/usage services, and shared
middleware (rate limiting, circuit breaker, JWT, RBAC).

> 📋 A full audit of this codebase (issues found, severity, scores) lives in
> [REVIEW.md](./REVIEW.md).

## 🏗️ Architecture

```
go-backend/
├── services/
│   ├── gateway/            # Auth + reverse proxy to the backends (port 8080)
│   ├── payment-service/    # Stripe PaymentIntents (8001, internal)
│   ├── customer-service/   # Stripe customers (8002, internal)
│   ├── webhook-service/    # Stripe webhook verification (8003, internal)
│   ├── usage-service/      # Metered usage ingestion -> Stripe (8080, internal)
│   ├── usage-engine/       # Redis-backed rolling usage analytics (8082)
│   ├── auth-service/       # Register/login/refresh (JWT HS256)
│   ├── billing-service/    # Billing API surface (stubs being built out)
│   ├── user-service/       # Users + RGPD export/consent/deletion
│   └── template-service/   # Skeleton for new services
├── shared/
│   ├── contracts/          # Cross-service DTOs
│   ├── middleware/         # Rate limiter, circuit breaker, JWT, RBAC, headers
│   ├── stripe-client/      # Stripe SDK wrapper (idempotent creates, metered billing)
│   ├── config/ httpserver/ logger/
infra/
└── docker-compose.yml      # Full local stack
```

## 🚦 Services

| Service | Port | Purpose |
|---|---|---|
| Gateway | 8080 | JWT auth (RS256), RBAC, reverse proxy to upstreams |
| Payment | 8001 | Create/get PaymentIntents (via gateway only) |
| Customer | 8002 | Create/get Stripe customers (via gateway only) |
| Webhook | 8003 | Stripe signature-verified webhooks |
| Usage | 8080 | Batched metered usage reporting to Stripe |
| Usage engine | 8082 | Redis rolling-average analytics |
| Auth | 8080 | Register/login/refresh (HS256) |
| User | 8080 | User profiles + RGPD endpoints |

## 🚀 Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- Stripe keys (test mode is fine)

### Local development (single service)

```bash
cd go-backend

# Generate JWT keys for the gateway
../scripts/gen-keys.sh

# Run any service (workspace mode builds everything):
go build ./...
```

### Full stack with Docker

```bash
cd infra
cat > .env <<'EOF'
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
JWT_SECRET_KEY=change-me-auth
AUTH_JWT_SECRET=change-me-billing
USER_SERVICE_JWT_SECRET=change-me-user
EOF
docker compose up --build
```

Only the gateway publishes a port; backend services are reachable through
`/api/v1/{upstream}/...` once authenticated:

```bash
# Log in (demo account is opt-in via GATEWAY_DEMO_USERNAME/PASSWORD in compose)
curl -s localhost:8080/auth/login -d '{"username":"demo","password":"demo"}'

# Call an upstream through the proxy
curl -s localhost:8080/api/v1/payments/health -H "Authorization: Bearer $TOKEN"
```

### Gateway upstreams

Configure proxy targets with `UPSTREAMS`:

```
UPSTREAMS=payments=http://payment-service:8001,customers=http://customer-service:8002
```

Unhealthy upstreams (failed `/health` probes) are ejected automatically and
return `503 upstream unhealthy` until they recover.

## 🧪 Tests

```bash
cd go-backend
go test ./...           # workspace mode
go test -race ./...     # race detector (recommended)
```

CI builds, vets and tests **every module** individually (see
`.github/workflows/ci.yml`) — a broken module graph fails the build instead
of hiding.

## 🔐 Security notes

- RBAC decisions come from validated JWT claims in the request context — never
  from client-supplied headers.
- Stripe mutations from `shared/stripe-client` carry idempotency keys; the
  usage batcher retries failed batches instead of dropping them.
- Refresh tokens rotate on use; replaying a rotated token revokes all of the
  user's sessions.
- The rate limiter trusts `X-Forwarded-For` only from configured
  `TrustedProxies` (spoof-proof mode) — set them when deploying behind a LB.
