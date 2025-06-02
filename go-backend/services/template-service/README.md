# Template Service

A template for Go microservices.

## Environment Variables

Copy `.env.example` to `.env` and update the values:

```bash
cp .env.example .env
```

## Running Locally

```bash
# Start dependencies
docker-compose up -d db

# Run service
go run cmd/server/main.go
```

## API Endpoints

- `GET /health` - Health check
- `GET /api/resource` - Get resources
- `POST /api/resource` - Create resource

## Running Tests

```bash
go test -v ./...
```

## Building

```bash
docker build -t template-service .
```

## License

MIT
