# Gateway Service

A simple API Gateway for routing requests to auth, billing, and user services.

## Endpoints
- `/auth/*` → auth-service
- `/billing/*` → billing-service
- `/users/*` → user-service
- `/health` → health check

## Usage

### Build
```sh
docker build -t gateway .
```

### Run (Docker)
```sh
docker run -p 8080:8080 gateway
```

### Run (Local)
```sh
go run main.go
```

## Environment Variables
- `PORT` (default: 8080)

---

This gateway uses static routing and can be extended for more services or dynamic discovery later. 