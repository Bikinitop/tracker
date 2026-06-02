# Rate Limiting & Circuit Breaker Design

**Date:** 2026-06-02
**Status:** Approved (pending spec review)
**Branch:** `feat/rate-limiting`

## Goal

Protect the tracker from two distinct failure modes:

1. **Abusive clients** — a single client IP flooding the `/track` endpoint.
2. **A failing downstream** — NATS becoming slow or unavailable, so the
   tracker hammers a sick dependency instead of failing fast.

These are different problems and get different mechanisms at different layers:

| Concern            | Mechanism             | Layer                       | Response |
|--------------------|-----------------------|-----------------------------|----------|
| Per-IP abuse       | Token-bucket limiter  | HTTP middleware             | `429`    |
| NATS publish health| Circuit breaker       | `EventPublisher` decorator  | `503`    |

Both features are individually toggleable and ship enabled with safe defaults.

## Non-Goals

- Distributed / cross-instance limiting (no Redis). The limiter sits behind an
  interface so a shared backend can be added later, but in-memory per-instance
  is what we build now.
- Per-site (`idsite`) fairness limiting. Out of scope for this iteration.
- A global request-rate ceiling. "Global" protection here is the circuit
  breaker on NATS health, not a volume cap.

## Architecture

### Component 1: Per-IP rate limiting (→ 429)

New package `internal/ratelimit`.

```go
// Limiter is the seam that lets a distributed backend replace the
// in-memory implementation later without touching handlers.
type Limiter interface {
    Allow(key string) bool
}
```

`IPRateLimiter` implements `Limiter`:

- Holds `map[string]*rate.Limiter` (`golang.org/x/time/rate`), guarded by a
  `sync.Mutex`. Each key (client IP) gets its own token bucket.
- Token bucket allows bursts — appropriate because one page load fires several
  tracking events nearly simultaneously.
- A background cleanup goroutine evicts entries idle longer than `IP_TTL`.
  This bounds memory under IP churn (a public tracker sees unbounded distinct
  IPs over time). Each entry records a `lastSeen` timestamp; cleanup runs on a
  ticker. A `Stop()` method shuts the goroutine down for clean test teardown.
- Time is injectable (`now func() time.Time`, defaults to `time.Now`) so
  eviction is testable without real sleeps.

**Middleware** (`internal/api/middleware.go`):

`RateLimitMiddleware(limiter, trustProxy, next)`:

1. Extract client IP (see below).
2. `limiter.Allow(ip)` → if false, write `429 Too Many Requests` with a
   `Retry-After` header and stop.
3. Otherwise call `next`.

Only `/track` is wrapped. `/health` is exempt so liveness/readiness probes are
never throttled.

**Client IP extraction** respects `TRACKER_TRUST_PROXY`:

- `false` (default): use `r.RemoteAddr` (host portion). Safe by default —
  clients cannot spoof.
- `true`: parse the leftmost address of `X-Forwarded-For`, falling back to
  `RemoteAddr` if absent. Only enable when running behind a trusted proxy/LB
  that sets XFF, because XFF is client-controllable otherwise.

### Component 2: Circuit breaker on NATS publish failures (→ 503)

New package `internal/circuitbreaker`.

Classic 3-state breaker:

```
        failures ≥ ratio (over ≥ minRequests in window)
 CLOSED ───────────────────────────────────────────────▶ OPEN
   ▲                                                       │
   │ probes succeed (≥ halfOpenProbes)                     │ openDuration elapsed
   │                                                       ▼
   └──────────────────── HALF-OPEN ◀──────────────────────┘
                              │
                              │ any probe fails
                              └──────────────▶ OPEN
```

- **Closed:** all requests allowed. Track successes/failures in a rolling
  window. Trip to Open when failures / total ≥ `FAILURE_RATIO` and
  total ≥ `MIN_REQUESTS` (the min-requests floor prevents tripping on a
  single early failure).
- **Open:** all requests fast-fail. After `OPEN_DURATION`, transition to
  Half-Open.
- **Half-Open:** allow a limited number of probe requests. Each success counts
  toward `HALF_OPEN_PROBES`; reaching it closes the breaker. Any failure
  reopens immediately.
- Injectable clock (`now func() time.Time`) for deterministic transition tests.
- Thread-safe (`sync.Mutex`); the breaker is shared across all request
  goroutines.

API:

```go
func (b *Breaker) Allow() bool       // false when open (or half-open quota spent)
func (b *Breaker) Record(success bool) // feed each publish outcome
```

**Integration — decorator over `EventPublisher`:**

```go
type breakerPublisher struct {
    inner   api.EventPublisher
    breaker *circuitbreaker.Breaker
}

func (p *breakerPublisher) PublishEvent(e *tracker.Event) error {
    if !p.breaker.Allow() {
        return ErrCircuitOpen
    }
    err := p.inner.PublishEvent(e)
    p.breaker.Record(err == nil)
    return err
}
```

**Handler change** (`internal/api/handler.go`): in both `processSingleRequest`
and `publishEvent`, map the sentinel:

```go
if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
    http.Error(w, "service unavailable", http.StatusServiceUnavailable)
    return
}
```

Other publish errors keep returning `500`.

### Wiring (`cmd/tracker/main.go` + `internal/api`)

- `config.Load()` reads the new env vars.
- In `newServer`: if circuit breaker enabled, wrap the NATS publisher in
  `breakerPublisher` before passing to the router.
- `NewRouter` gains options (limiter + trustProxy) and wraps `/track` in
  `RateLimitMiddleware` when the limiter is non-nil.

## Configuration

| Env var                       | Default | Meaning                                            |
|-------------------------------|---------|----------------------------------------------------|
| `TRACKER_RATELIMIT_ENABLED`   | `true`  | Enable per-IP limiting                             |
| `TRACKER_RATELIMIT_RPS`       | `50`    | Sustained requests/sec per IP                      |
| `TRACKER_RATELIMIT_BURST`     | `100`   | Burst bucket size per IP                           |
| `TRACKER_RATELIMIT_IP_TTL`    | `10m`   | Evict per-IP buckets idle longer than this         |
| `TRACKER_TRUST_PROXY`         | `false` | Trust `X-Forwarded-For` for client IP              |
| `TRACKER_CB_ENABLED`          | `true`  | Enable NATS-publish circuit breaker                |
| `TRACKER_CB_FAILURE_RATIO`    | `0.5`   | Failure fraction that trips the breaker            |
| `TRACKER_CB_MIN_REQUESTS`     | `20`    | Min samples in window before the breaker can trip  |
| `TRACKER_CB_WINDOW`           | `10s`   | Rolling window for failure accounting              |
| `TRACKER_CB_OPEN_DURATION`    | `5s`    | Time in Open before probing (Half-Open)            |
| `TRACKER_CB_HALF_OPEN_PROBES` | `5`     | Successful probes needed to close the breaker      |

Invalid/unparseable values fall back to defaults (don't crash the server).

## Testing (TDD, table-driven, `go test -race`)

- **`ratelimit`**: under limit allowed; over limit blocked; burst consumed then
  refilled; distinct keys isolated; idle eviction (injected clock); concurrent
  `Allow` under `-race`.
- **`circuitbreaker`**: Closed→Open at threshold; min-requests floor prevents
  premature trip; Open fast-fails; Open→Half-Open after duration; Half-Open→
  Closed after N probe successes; Half-Open→Open on probe failure; concurrency
  under `-race`. All transitions driven by injected clock — no `time.Sleep`.
- **`api` middleware**: 429 + `Retry-After` on breach; allowed requests pass
  through; IP extraction (RemoteAddr default vs XFF when trusted); `/health`
  exempt.
- **breaker publisher decorator**: returns `ErrCircuitOpen` when open without
  calling inner; records outcomes; handler maps `ErrCircuitOpen` → 503 and
  other errors → 500.
- **Benchmarks**: `/track` hot path with limiter + breaker enabled to confirm
  negligible overhead vs the existing benchmarks.

## Risks / Trade-offs

- **In-memory limiter behind a load balancer** limits per-instance, so the
  effective per-IP limit is ~N× with N instances. Accepted; the `Limiter`
  interface leaves room for a shared backend later.
- **`TRACKER_TRUST_PROXY`** must only be enabled behind a trusted proxy;
  documented and off by default.
- **Breaker shared mutex** is on the hot path. Kept minimal (counter
  increments) and benchmarked.
