# 🚀 Ultra-Lean Stripe Microservices Ecosystem

A modular, scalable Stripe payment system built with Go microservices. Each service is **< 1000 LOC** for maximum maintainability and AI-assisted development.

## 🏗️ Architecture

```
stripe-ecosystem/
├── services/                    # Microservices (each < 1000 LOC)
│   ├── payment-service/         # Payment processing
│   ├── customer-service/        # Customer management
│   ├── webhook-service/         # Stripe webhooks
│   ├── subscription-service/    # Recurring billing
│   ├── invoice-service/         # Invoice management
│   ├── auth-service/           # Authentication
│   └── gateway-service/        # API Gateway
├── shared/                     # Shared libraries
│   ├── contracts/              # Type definitions
│   ├── stripe-client/          # Stripe wrapper
│   └── middleware/             # Common middleware
└── infrastructure/             # Deployment configs
```

## ✨ Key Features

- **🎯 Ultra-Lean**: Each service < 1000 LOC
- **⚡ Fast**: Go's performance + minimal overhead
- **🔄 Scalable**: Independent horizontal scaling
- **🛡️ Secure**: Stripe webhook verification
- **🔧 Modular**: Clear service boundaries
- **📦 Containerized**: Docker + Docker Compose ready

## 🚦 Services

| Service | Port | Purpose | LOC |
|---------|------|---------|-----|
| Gateway | 8000 | API routing & auth | ~600 |
| Payment | 8001 | Process payments | ~800 |
| Customer | 8002 | Manage customers | ~600 |
| Webhook | 8003 | Handle Stripe events | ~500 |
| Subscription | 8004 | Recurring billing | ~700 |
| Invoice | 8005 | Invoice generation | ~600 |
| Auth | 8006 | JWT authentication | ~400 |

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- Stripe account with API keys

### Environment Setup
```bash
# Create .env file
cp .env.example .env

# Add your Stripe keys
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
```

### Run with Docker Compose
```bash
# Start all services
docker-compose up -d

# Check service health
curl http://localhost:8000/health
```

### Run Individual Services (Development)
```bash
# Initialize modules
go work sync

# Run payment service
cd services/payment-service
STRIPE_SECRET_KEY=sk_test_... go run main.go

# Run customer service
cd services/customer-service  
STRIPE_SECRET_KEY=sk_test_... go run main.go
```

## 📡 API Endpoints

### Gateway Service (Port 8000)
```bash
# Health check
GET /health

# Route to services
POST /api/payments      → payment-service
GET  /api/customers     → customer-service
POST /webhook          → webhook-service
```

### Payment Service (Port 8001)
```bash
# Create payment
POST /payments
{
  "customer_id": "cus_...",
  "amount": 2000,
  "currency": "usd",
  "description": "Test payment"
}

# Get payment
GET /payments/{id}
```

### Customer Service (Port 8002)
```bash
# Create customer
POST /customers
{
  "email": "test@example.com",
  "name": "Test User"
}

# Get customer
GET /customers/{id}
```

## 🔧 Development

### Adding a New Service
1. Create service directory: `services/new-service/`
2. Add to `go.work` file
3. Create `main.go` with < 1000 LOC
4. Add to `docker-compose.yml`

### Service Template
```go
package main

import (
    "github.com/gorilla/mux"
    "github.com/stripe-ecosystem/shared/contracts"
)

type NewService struct {
    // Keep it simple
}

func main() {
    service := NewService{}
    r := mux.NewRouter()
    
    // Add routes
    r.HandleFunc("/health", service.healthCheck)
    
    // Start server
    http.ListenAndServe(":8007", r)
}
```

## 🧪 Testing

### Tests Unitaires
```bash
make test
```

### Tests d'Intégration
```bash
# Configurer d'abord les variables d'environnement
export TEST_DB_HOST=...
make test-integration
```

### Couverture de Code
```bash
make test-cover
```

### Exemple CI (GitHub Actions)
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:13
        env:
          POSTGRES_USER: testuser
          POSTGRES_PASSWORD: testpass
          POSTGRES_DB: testdb
        ports: ["5432:5432"]
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with: { go-version: '1.21' }
      - run: make test-all
        env:
          TEST_DB_HOST: localhost
          TEST_DB_USER: testuser
          TEST_DB_PASSWORD: testpass
          TEST_DB_NAME: testdb
```

## 🏗️ Deployment

### Kubernetes
```bash
# Deploy to K8s
kubectl apply -f infrastructure/k8s/
```

### Production Environment
- Use managed databases (PostgreSQL)
- Implement proper logging (structured logs)
- Add monitoring (Prometheus/Grafana)
- Set up CI/CD pipelines

## 🔒 Security

- Stripe webhook signature verification
- JWT authentication (auth-service)
- HTTPS in production
- Environment variable secrets
- Rate limiting per service

## 📈 Scaling

Each service can scale independently:
- **Payment Service**: Scale for high transaction volume
- **Webhook Service**: Scale for event processing
- **Customer Service**: Scale for user management load

## 🤝 Contributing

1. Keep services under 1000 LOC
2. Use shared contracts for types
3. Follow Go best practices
4. Add tests for new functionality
5. Update this README

## 📝 License

MIT License - feel free to use for your projects!

---

**🎯 Perfect for AI-assisted development** - Each service is small enough to understand completely, while building something that scales infinitely! 

# usage-engine: Predictive Usage-Driven Pricing Engine

The `usage-engine` microservice ingests API call events, tracks per-customer usage in Redis, and computes a rolling 7-day average. When usage crosses thresholds, it can trigger Stripe metered billing or tier upgrades (integration WIP).

## Endpoints
- `POST /event` — Ingest a usage event
  - Body: `{ "customer_id": string, "timestamp": RFC3339 }`
- `GET /health` — Health check

## Redis Usage
- Stores per-customer, per-hour usage as hashes: `usage:<customer_id>` {`<hour>`: count}
- Rolling average computed over last 7 days (168 hours)

## Configuration (Environment Variables)
- `PORT` — HTTP port (default: 8082)
- `REDIS_URL` — Redis connection URL (default: redis://localhost:6379)
- `STRIPE_API_KEY` — Stripe secret key (default: sk_test_placeholder)
- `SHUTDOWN_TIMEOUT` — Graceful shutdown timeout (default: 10s)

## Local Development
- The service is included in `docker-compose.yml` and uses the local `redis` service.
- Example event ingestion:
  ```sh
  curl -X POST http://localhost:8082/event \
    -H 'Content-Type: application/json' \
    -d '{"customer_id":"cus_123","timestamp":"2024-06-01T12:00:00Z"}'
  ```

## Next Steps
- Integrate with Stripe for metered billing and tier upgrades
- Add real tests for usage tracking and rolling average
- Add metrics and alerting

---
See the top of `services/usage-engine/main.go` for inline documentation. 

# Go Microservices Monorepo

## Prérequis
- Go 1.24+
- Docker & Docker Compose
- (Optionnel) golangci-lint, migrate

## Setup rapide

```bash
git clone ...
cp .env.example .env
make build
make test
make run
```

## Services principaux
- user-service : gestion des utilisateurs
- billing-service : facturation
- auth-service : authentification
- gateway : API Gateway
- ...

## Variables d'environnement
Voir `.env.example` pour la liste complète.

## Endpoints principaux
- Voir les README de chaque service dans `services/<service>/README.md`

## Tests
```bash
make test
```

## Lint
```bash
make lint
```

## Migrations
```bash
make migrate
```

## Documentation API
- Swagger/OpenAPI : à venir dans chaque service

## CI/CD
- GitHub Actions : build, test, lint, docker build (voir `.github/workflows/`)

## Observabilité
- Prometheus, logs JSON, healthchecks (voir docker-compose.yml)

## Contact
- Mainteneur : ... 

## Quickstart

### Deployment
- Build & run: `docker compose up -d --build`
- API: http://localhost:8000
- Streamlit UI: http://localhost:8501
- Prometheus: http://localhost:9090

### Security
- Set `API_KEY` env variable for production
- Use `x-api-key` and `x-role` headers for all API calls

### Health & Monitoring
- Liveness: `/api/health`
- Readiness: `/api/ready`
- Metrics: `/metrics` (Prometheus)
- Logs: `/api/logs` (admin)

### Testing
- Unit & integration tests: `pytest tests/`

### API Docs
- OpenAPI: `/docs`

--- 