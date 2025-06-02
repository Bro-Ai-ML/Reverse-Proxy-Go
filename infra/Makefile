# Makefile for monorepo Go microservices

.PHONY: all build test lint run migrate test-all test-integration

all: build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build all services
build:
	@echo "🔨 Building all services..."
	@cd services/customer-service && $(GOBUILD) -v .
	@cd services/payment-service && $(GOBUILD) -v .
	@cd services/usage-service && $(GOBUILD) -v .
	@cd services/webhook-service && $(GOBUILD) -v .
	@echo "✅ All services built successfully"

# Run tests with coverage
test:
	@echo "🧪 Running tests with coverage and race detector..."
	@$(GOTEST) -race -v -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

# Lint code
lint:
	@echo "🔍 Running linters..."
	@golangci-lint run ./...
	@echo "✅ Linting complete"

# Security scan
security:
	@echo "🔒 Running security scan..."
	@gosec ./...
	@echo "✅ Security scan complete"

# Tidy dependencies
deps:
	@echo "📦 Tidying dependencies..."
	@$(GOMOD) tidy
	@cd shared/stripe-client && $(GOMOD) tidy
	@cd shared/contracts && $(GOMOD) tidy
	@cd services/customer-service && $(GOMOD) tidy
	@cd services/payment-service && $(GOMOD) tidy
	@cd services/usage-service && $(GOMOD) tidy
	@cd services/webhook-service && $(GOMOD) tidy
	@echo "✅ Dependencies updated"

# Generate documentation
docs:
	@echo "📚 Generating documentation..."
	@godoc -http=:6060 &
	@echo "📖 Documentation server running on http://localhost:6060"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@$(GOCLEAN)
	@rm -rf bin/ dist/ coverage.out coverage.html
	@find . -name "*.test" -delete
	@echo "✅ Clean complete"

# Docker build
docker-build:
	@echo "🐳 Building Docker images..."
	@docker-compose build
	@echo "✅ Docker images built"

# Docker run
docker-run:
	@echo "🚀 Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "✅ Services running"

# Performance benchmarks
bench:
	@echo "⚡ Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./...

# Run
run:
	@echo "🚀 Starting services with Docker Compose..."
	@docker-compose up --build
	@echo "✅ Services running"

# Migrate
migrate:
	@echo "Run DB migrations for all services"
	@for svc in services/*; do \
	  if [ -d "$$svc/migrations" ]; then \
	    migrate -path "$$svc/migrations" -database "postgres://$$DB_USER:$$DB_PASSWORD@$$DB_HOST:$$DB_PORT/$$DB_NAME?sslmode=$$DB_SSLMODE" up; \
	  fi; \
	done

# Test all
test-all: test test-integration

# Test integration
test-integration:
	go test -tags=integration ./...

# Help
help:
	@echo "🚀 Stripe Microservices - Available Commands:"
	@echo ""
	@echo "  build      - Build all microservices"
	@echo "  test       - Run tests with coverage report"
	@echo "  lint       - Run code linting"
	@echo "  security   - Run security scan"
	@echo "  deps       - Update dependencies"
	@echo "  docs       - Generate and serve documentation"
	@echo "  bench      - Run performance benchmarks"
	@echo "  clean      - Clean build artifacts"
	@echo "  docker-build - Build Docker images"
	@echo "  docker-run - Start services with Docker"
	@echo "  run        - Start services with Docker Compose"
	@echo "  migrate    - Run DB migrations for all services"
	@echo "  test-all   - Run all tests"
	@echo "  test-integration - Run integration tests"
	@echo ""
	@echo "🎯 Example: make build test lint" 