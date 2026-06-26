# GoBouncer

GoBouncer is a fast, scalable rate-limiting API service written in Go. It uses Redis as a backing store and implements the Sliding Window rate-limiting algorithm to ensure smooth and accurate traffic control.

## Features

- **Sliding Window Algorithm**: Provides a more accurate and smooth rate limiting mechanism compared to fixed window counters.
- **Redis Integration**: Uses Redis for high-performance and distributed state management.
- **Environment Driven**: Configurable using `.env` files for easy deployment and local testing.
- **Atomic Operations**: Thread-safe implementations using `go.uber.org/atomic`.

## Project Structure

```text
.
├── cmd/
│   └── api/
│       └── main.go               # Entry point of the API server
├── internal/
│   ├── handlers/
│   │   └── check.go              # HTTP handlers for checking rate limits
│   └── limiter/
│       └── sliding_window.go     # Core sliding window rate-limiting logic
├── .env                          # Environment variables configuration
├── go.mod                        # Go module dependencies
└── go.sum                        # Go module checksums
```

## Prerequisites

- [Go](https://golang.org/dl/) 1.26 or higher
- [Redis](https://redis.io/download) server running locally or remotely

## Getting Started

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ritik-kharya/gobouncer.git
   cd gobouncer
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Configure Environment Variables:**
   Create or modify the `.env` file in the root directory to match your Redis configuration:
   ```env
   REDIS_ADDR=localhost:6379
   REDIS_PASSWORD=
   REDIS_DB=0
   PORT=8080
   POLICY_FILE=config/policies.example.json
   FAIL_OPEN=true
   ```

4. **Run the Application:**
   ```bash
   go run cmd/api/main.go
   ```

## Running with Docker

You can run both GoBouncer and Redis inside Docker containers using Docker Compose. This is the recommended way to run the service locally without manual setup.

1. **Start all services:**
   ```bash
   docker compose up --build -d
   ```
   This command:
   - Builds the GoBouncer application image.
   - Starts a Redis container.
   - Configures the GoBouncer application to connect to the Redis container using environment variables (`REDIS_ADDR=redis:6379`).
   - Starts the GoBouncer API service on port `8080`.

2. **Check logs:**
   ```bash
   docker compose logs -f
   ```

3. **Stop all services:**
   ```bash
   docker compose down
   ```

## Policy-Based Rate Limits

GoBouncer supports named policies so application code does not need to hardcode limits everywhere.

Check a request with a named policy:

```bash
curl -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{"key":"user:123","policy":"login"}'
```

List available policies:

```bash
curl http://localhost:8080/policies
```

Policy file format:

```json
{
  "policies": [
    {
      "name": "login",
      "limit": 5,
      "window_ms": 300000,
      "algorithm": "gcra"
    }
  ]
}
```

If `POLICY_FILE` is not set, GoBouncer starts with built-in policies: `default`, `ip-basic`, `login`, `login-route`, `public-api`, and `user-free`.

## Multi-Dimensional Limits

Use `checks` when one request must satisfy multiple limits, such as IP + user + route.

```bash
curl -X POST http://localhost:8080/check \
  -H "Content-Type: application/json" \
  -d '{
    "checks": [
      {"name":"ip","policy":"ip-basic","key":"ip:1.2.3.4"},
      {"name":"user","policy":"user-free","key":"user:123"},
      {"name":"login","policy":"login-route","key":"route:/login:user:123"}
    ]
  }'
```

Go middleware example:

```go
limited := gobouncer.RateLimit(client, gobouncer.WithCheckFunc(func(r *http.Request) []gobouncer.Check {
    userID := r.Header.Get("X-User-ID")
    return []gobouncer.Check{
        gobouncer.PolicyCheck("ip", gobouncer.IPKey(r), "ip-basic"),
        gobouncer.PolicyCheck("user", "user:"+userID, "user-free"),
        gobouncer.PolicyCheck("login", "route:"+r.URL.Path+":user:"+userID, "login-route"),
    }
}))(handler)
```

The request is allowed only if every dimension is allowed.

## Dependencies

- [go-redis/v9](https://github.com/redis/go-redis) - Redis client for Go
- [godotenv](https://github.com/joho/godotenv) - Port of Ruby's dotenv library
- [atomic](https://github.com/uber-go/atomic) - Wrapper types for sync/atomic

## License

This project is open-source and available under the MIT License.
