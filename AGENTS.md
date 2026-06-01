# Tracker Service - Project Guidelines

## Project Overview
Tracker service for the Bikinitop project. A high-performance Go service fully compatible with Matomo tracking APIs.

## Architecture
- **Language**: Go (latest stable)
- **Protocol**: HTTP REST API (Matomo-compatible)
- **Message Broker**: NATS (write tracking data to subjects, not DB)
- **Concurrency**: Designed for high throughput and concurrent request handling

## API Compatibility
- Matomo Tracking API: https://developer.matomo.org/api-reference/tracking-api
- Matomo Integration Guide: https://developer.matomo.org/integration
- Must be fully compatible with existing Matomo SDKs

## Development Practices
- **Methodology**: Test-Driven Development (TDD)
- **Coverage Requirement**: >90% test coverage
- **Branching**: Feature branches only; direct commits to `main` are **FORBIDDEN**
- All changes must go through PR review

## Project Structure
```
/
├── cmd/                  # Application entrypoints
├── internal/             # Private application code
│   ├── api/              # HTTP handlers (Matomo-compatible endpoints)
│   ├── tracker/          # Core tracking logic
│   ├── nats/             # NATS publisher/client
│   └── config/           # Configuration management
├── pkg/                  # Public libraries (if any)
├── tests/                # Integration and e2e tests
└── docs/                 # Additional documentation
```

## Key Components
1. **HTTP API Layer**: Matomo-compatible tracking endpoints
2. **NATS Publisher**: Async publishing to NATS subjects
3. **Request Validator**: Validate incoming tracking requests
4. **Event Builder**: Construct NATS messages from tracking data

## NATS Subject Design
- Subject pattern: `tracker.{site_id}.{event_type}`
- JSON payload with standard Matomo fields

## Testing
- Unit tests for all packages
- Integration tests for NATS publishing
- API contract tests for Matomo compatibility
- Benchmark tests for performance-critical paths

## Commands
- `go test ./... -race -coverprofile=coverage.out`
- `go tool cover -func=coverage.out`

## Environment
- Go version: latest stable
- NATS server for local development
- No direct database dependencies
