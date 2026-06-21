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

If `POLICY_FILE` is not set, GoBouncer starts with built-in policies: `default`, `login`, and `public-api`.

## Dependencies

- [go-redis/v9](https://github.com/redis/go-redis) - Redis client for Go
- [godotenv](https://github.com/joho/godotenv) - Port of Ruby's dotenv library
- [atomic](https://github.com/uber-go/atomic) - Wrapper types for sync/atomic

## License

This project is open-source and available under the MIT License.
