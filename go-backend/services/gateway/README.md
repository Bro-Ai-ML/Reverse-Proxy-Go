# Gateway Service

API gateway: JWT (RS256) authentication, role-based access control, and
health-aware **reverse-proxy routing** to upstream services.

## Endpoints

- `GET /` — welcome
- `GET /health` — liveness heartbeat
- `POST /auth/login` — obtain access + refresh tokens
- `POST /auth/refresh` — rotate the refresh token (reuse detection built in)
- `POST /auth/logout` — revoke a refresh token
- `GET /api/user/profile` — authenticated demo route
- `GET /api/admin/users` — requires role `admin`
- `GET /api/reports/` — requires role `admin` or `reporter`
- `ANY /api/v1/{upstream}/...` — proxied to the configured upstream

## Upstreams

```sh
UPSTREAMS=payments=http://payment-service:8001,customers=http://customer-service:8002
```

Each upstream is probed on `/health` every `HEALTH_INTERVAL` (default 30s).
Unhealthy upstreams receive `503 upstream unhealthy` until they recover;
proxy failures return `502 bad gateway`.

## JWT keys

The gateway signs/verifies with an RSA keypair:

```sh
../../../scripts/gen-keys.sh   # writes secrets/keys/{private,public}.pem
```

Paths are configurable via `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH`.
The Docker image ships a generated demo keypair; mount real keys in
production.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `UPSTREAMS` | *(empty)* | `name=url` pairs, comma-separated |
| `HEALTH_INTERVAL` | `30s` | Upstream probe interval |
| `TOKEN_DURATION` | `15m` | Access-token TTL |
| `REFRESH_DURATION` | `168h` | Refresh-token TTL |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` / `IDLE_TIMEOUT` | `10s`/`30s`/`60s` | HTTP server timeouts |
| `GATEWAY_DEMO_USERNAME` / `GATEWAY_DEMO_PASSWORD` | *(unset)* | Opt-in demo login; when unset, `/auth/login` fails closed |

## Run

```sh
# Local
go run .

# Docker (build from the go-backend root)
docker build -f services/gateway/Dockerfile -t gateway ../..
docker run -p 8080:8080 gateway
```
