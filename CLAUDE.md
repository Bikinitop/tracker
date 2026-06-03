# CLAUDE.md - Tracker Service

## Project Context
This repository implements a tracker service for the Bikinitop project. It is a Matomo-compatible tracking API server written in Go.

## Core Requirements
1. **Matomo API Compatibility**: Must be fully compatible with existing Matomo SDKs and the Matomo Tracking API
2. **NATS Integration**: All tracking data is published to NATS subjects; no direct database writes
3. **High Performance**: Designed for high concurrency and throughput
4. **Test Coverage**: Maintain >90% test coverage using TDD

## Reference APIs
- Tracking API: https://developer.matomo.org/api-reference/tracking-api
- Integration Guide: https://developer.matomo.org/integration

## Development Rules
- Use feature branches for all work
- Never commit directly to `main`
- Write tests before implementation (TDD)
- Ensure race detector passes: `go test -race ./...`

## Tech Stack
- Go (latest stable)
- NATS (nats.go client)
- Standard library HTTP server

## Package Structure
- `internal/api` - HTTP handlers, routing, rate-limit middleware, circuit-breaker publisher
- `internal/tracker` - Core tracking logic and request parsing
- `internal/nats` - NATS connection and publishing
- `internal/config` - Environment configuration
- `internal/ratelimit` - In-memory per-IP token-bucket limiter (behind a `Limiter` interface)
- `internal/circuitbreaker` - 3-state circuit breaker for NATS publish health

## Configuration (Environment Variables)
All config is read in `internal/config`. Invalid values fall back to the default.

| Variable | Default | Purpose |
|----------|---------|---------|
| `TRACKER_PORT` | `8080` | HTTP listen port |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL; set to `disabled` (or empty) to run without NATS |
| `TRACKER_RATELIMIT_ENABLED` | `true` | Enable per-IP rate limiting on `/track` |
| `TRACKER_RATELIMIT_RPS` | `50` | Sustained requests/sec per client IP |
| `TRACKER_RATELIMIT_BURST` | `100` | Burst bucket size per client IP |
| `TRACKER_RATELIMIT_IP_TTL` | `10m` | Evict per-IP buckets idle longer than this |
| `TRACKER_TRUST_PROXY` | `false` | Trust leftmost `X-Forwarded-For` for client IP (only behind a trusted proxy) |
| `TRACKER_CB_ENABLED` | `true` | Enable the NATS-publish circuit breaker |
| `TRACKER_CB_FAILURE_RATIO` | `0.5` | Failure fraction that trips the breaker |
| `TRACKER_CB_MIN_REQUESTS` | `20` | Min samples in the window before the breaker can trip |
| `TRACKER_CB_WINDOW` | `10s` | Tumbling window for failure accounting |
| `TRACKER_CB_OPEN_DURATION` | `5s` | Time in Open before probing (Half-Open) |
| `TRACKER_CB_HALF_OPEN_PROBES` | `5` | Successful probes needed to close the breaker |

Limit breaches return `429` (with `Retry-After`); an open circuit returns `503`.

## NATS Subject Convention
- Format: `tracker.{site_id}.pageview`
- Payload: JSON with all Matomo tracking parameters

## Testing Approach
- Table-driven tests for handlers
- Mock NATS connection for unit tests
- Benchmark tests for hot paths
- Use `httptest` for HTTP handler testing
