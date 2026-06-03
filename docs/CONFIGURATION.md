# Configuration

All configuration is read from environment variables at startup
(`internal/config`). Every value has a safe default, and **an unparseable value
falls back to its default** (it does not crash the server).

## Server

| Variable | Default | Meaning |
|----------|---------|---------|
| `TRACKER_PORT` | `8080` | HTTP listen port |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL. Set to `disabled` (or empty) to run without NATS — events are parsed but not published. |

## Per-IP rate limiting

Token-bucket limiter applied to `/track` (only). When a client IP exceeds its
quota the request gets `429 Too Many Requests` with a `Retry-After` header.
`/health` is never rate limited.

| Variable | Default | Meaning |
|----------|---------|---------|
| `TRACKER_RATELIMIT_ENABLED` | `true` | Enable per-IP rate limiting |
| `TRACKER_RATELIMIT_RPS` | `50` | Sustained requests/sec per client IP |
| `TRACKER_RATELIMIT_BURST` | `100` | Burst bucket size per client IP |
| `TRACKER_RATELIMIT_IP_TTL` | `10m` | Evict per-IP buckets idle longer than this (bounds memory) |
| `TRACKER_TRUST_PROXY` | `false` | Trust `X-Forwarded-For` for the client IP |

### `TRACKER_TRUST_PROXY`

- **`false` (default):** the client IP is taken from the connection's
  `RemoteAddr`. Safe when clients connect directly.
- **`true`:** the client IP is taken from the **rightmost** `X-Forwarded-For`
  entry — the address appended by your own trusted proxy. Only enable this when
  the tracker sits behind a proxy/load balancer that sets `X-Forwarded-For`;
  otherwise a client could spoof the header to dodge the per-IP limit.

Invalid values are clamped to safe defaults (`burst < 1` → `1`, non-positive
`rps` → unlimited, non-positive TTL → `1m`) so a misconfiguration can't disable
the limiter or leak memory.

## NATS-publish circuit breaker

A 3-state breaker (Closed → Open → Half-Open) wraps NATS publishing. When the
publish failure ratio crosses the threshold the breaker opens and `/track`
fast-fails with `503` instead of hammering an unhealthy NATS; after a cooldown
it admits a few probes and closes again on success.

| Variable | Default | Meaning |
|----------|---------|---------|
| `TRACKER_CB_ENABLED` | `true` | Enable the circuit breaker |
| `TRACKER_CB_FAILURE_RATIO` | `0.5` | Failure fraction (failures ÷ total) that trips the breaker |
| `TRACKER_CB_MIN_REQUESTS` | `20` | Minimum samples in the window before the breaker can trip |
| `TRACKER_CB_WINDOW` | `10s` | Tumbling window for failure accounting (counts reset when it elapses) |
| `TRACKER_CB_OPEN_DURATION` | `5s` | Time the breaker stays open before probing (Half-Open) |
| `TRACKER_CB_HALF_OPEN_PROBES` | `5` | Successful probes required to close the breaker again |

Invalid values are clamped to safe defaults in the breaker constructor (e.g. a
`FailureRatio` outside `(0, 1]` → `0.5`, `MinRequests < 1` → `1`) so a
misconfiguration can't make the breaker trip under healthy load or never
recover.

## Durations

Duration variables (`*_TTL`, `*_WINDOW`, `*_OPEN_DURATION`) use Go duration
syntax: `300ms`, `10s`, `5m`, `1h`, etc.

## Example

```bash
docker run --rm -p 8080:8080 \
  -e NATS_URL=nats://nats.internal:4222 \
  -e TRACKER_TRUST_PROXY=true \
  -e TRACKER_RATELIMIT_RPS=200 \
  -e TRACKER_RATELIMIT_BURST=400 \
  -e TRACKER_CB_WINDOW=30s \
  ghcr.io/bikinitop/tracker:latest
```
