# tracker

A Matomo-compatible tracking API server written in Go. It accepts tracking
requests on a Matomo-style endpoint and publishes each event to
[NATS](https://nats.io/) — it never writes to a database directly. It is built
for high concurrency and ships with per-IP rate limiting and a circuit breaker
that protects against a failing NATS.

- **Matomo-compatible** — supports the Matomo Tracking API parameter set
  (single requests and bulk). Matomo SDKs (incl. the JS tracker) work once
  pointed at the `/track` endpoint (the default `matomo.php` path is not served).
- **NATS-native** — every event is published to `tracker.{site_id}.{action_type}`.
- **Resilient** — per-IP token-bucket rate limiting (`429`) and a NATS-publish
  circuit breaker (`503`), both configurable and on by default.
- **Small & portable** — distroless multi-arch image (`linux/amd64`, `linux/arm64`).

## Quick start

### Run the published image (recommended)

```bash
docker run --rm -p 8080:8080 \
  -e NATS_URL=nats://your-nats:4222 \
  ghcr.io/bikinitop/tracker:latest
```

The server listens on `:8080`. Send a test pageview:

```bash
curl "http://localhost:8080/track?idsite=1&rec=1&url=http://example.com&action_name=Home"
# → 1x1 GIF (HTTP 200)
curl http://localhost:8080/health
# → ok
```

To run without NATS (handy for local smoke tests — events are parsed but not
published), set `NATS_URL=disabled`.

### Run from source

```bash
go run ./cmd/tracker            # uses defaults: :8080, nats://localhost:4222
```

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET/POST | `/track` | Matomo-compatible tracking (single or bulk). Returns a 1x1 GIF by default. |
| GET | `/health` | Liveness/readiness probe. Returns `ok` (200). Never rate limited. |

`idsite` and a non-empty `rec` (Matomo sends `rec=1`) are the minimum required
parameters. See
**[docs/API.md](docs/API.md)** for the full parameter set, bulk format, and the
response/status-code matrix.

## Configuration

All configuration is via environment variables with safe defaults. The most
common:

| Variable | Default | Purpose |
|----------|---------|---------|
| `TRACKER_PORT` | `8080` | HTTP listen port |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL (`disabled` to run without NATS) |

Rate-limit and circuit-breaker knobs (`TRACKER_RATELIMIT_*`, `TRACKER_CB_*`,
`TRACKER_TRUST_PROXY`) are documented in
**[docs/CONFIGURATION.md](docs/CONFIGURATION.md)**.

## How it works

```
client ──HTTP──▶ /track ──▶ [rate limit] ──▶ parse Matomo params ──▶ [circuit breaker] ──▶ NATS
                                │ 429 over quota                          │ 503 if NATS unhealthy
                                ▼                                         ▼
                          per-IP token bucket                    tracker.{site_id}.{action_type}
```

- The request is rate limited per client IP (token bucket). Over-quota requests
  get `429` with a `Retry-After` header.
- Parameters are parsed into an event and the **action type** is derived
  (`pageview`, `event`, `goal`, `ecommerce`, `search`, `outlink`, `download`,
  `content_impression`, `content_interaction`, `heartbeat`).
- The event is published to NATS subject `tracker.{site_id}.{action_type}` as
  JSON. If NATS is unhealthy, the circuit breaker fast-fails with `503`.

## Development

Requires Go (see the `go` directive in [go.mod](go.mod)).

```bash
go build ./...
go vet ./...
go test -race ./...      # race detector is required; keep coverage > 90%
```

The project follows TDD with table-driven tests and `httptest` for handlers; NATS
is mocked in unit tests. See [CLAUDE.md](CLAUDE.md) for project conventions.

## Releasing

Pushing a semver tag (`vX.Y.Z`) builds and publishes a multi-arch image to
`ghcr.io/bikinitop/tracker`. See **[docs/RELEASING.md](docs/RELEASING.md)**.

## License

[Apache License 2.0](LICENSE).
