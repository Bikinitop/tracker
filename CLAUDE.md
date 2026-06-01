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
- Standard library HTTP server (or lightweight router)

## Package Structure
- `internal/api` - HTTP handlers and routing
- `internal/tracker` - Core tracking logic and request parsing
- `internal/nats` - NATS connection and publishing
- `internal/config` - Environment configuration

## NATS Subject Convention
- Format: `tracker.events.{site_id}`
- Payload: JSON with all Matomo tracking parameters

## Testing Approach
- Table-driven tests for handlers
- Mock NATS connection for unit tests
- Benchmark tests for hot paths
- Use `httptest` for HTTP handler testing
