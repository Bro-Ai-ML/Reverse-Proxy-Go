# Auth Service

Authentication and user management microservice with JWT-based authentication.

## Features

- User registration and login
- JWT-based authentication
- Refresh token mechanism
- Password hashing using bcrypt
- User profile management
- Password change functionality
- Health check endpoint
- Database migrations

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Docker (optional, for containerized deployment)

## Environment Variables

Copy `.env.example` to `.env` and update the values:

```bash
cp .env.example .env
```

## Running Locally

1. Start PostgreSQL database:

```bash
docker run --name auth-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=auth -p 5432:5432 -d postgres:15-alpine
```

2. Run migrations:

```bash
go run cmd/server/main.go migrate
```

3. Start the service:

```bash
go run cmd/server/main.go
```

The service will be available at `http://localhost:8080`

## API Endpoints

### Authentication

- `POST /api/v1/auth/register` - Register a new user
- `POST /api/v1/auth/login` - Login and get access/refresh tokens
- `POST /api/v1/auth/refresh` - Refresh access token using refresh token
- `GET /api/v1/auth/verify` - Verify access token

### User

- `GET /api/v1/users/me` - Get current user profile (protected)
- `PUT /api/v1/users/me` - Update current user profile (protected)
- `PUT /api/v1/users/me/password` - Change password (protected)

### Health

- `GET /health` - Health check

## Testing

Run the tests:

```bash
go test -v ./...
```

## Building

Build the Docker image:

```bash
docker build -t auth-service .
```

## Deployment

Run the service using Docker Compose:

```bash
docker-compose up -d
```

## License

MIT
