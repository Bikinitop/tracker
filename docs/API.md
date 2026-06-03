# API Reference

The tracker accepts requests in the
[Matomo Tracking API](https://developer.matomo.org/api-reference/tracking-api)
format and recognizes a broad subset of its parameters (see
[Common parameters](#common-parameters)). Matomo SDKs (including the JS tracker)
work once their tracker URL is pointed at this server's **`/track`** endpoint —
the default `matomo.php` path is intentionally not served (it returns `404`).

## `GET` / `POST` `/track`

Records a tracking request. Parameters may be sent as a query string (GET) or a
URL-encoded form body (POST). The minimum required parameters are `idsite` and
`rec` — both must be present and non-empty. Matomo sends `rec=1`, but the value
is not validated (e.g. `rec=0` is still recorded); a missing/empty `idsite` or
`rec` returns `400`.

### Response behavior

| Condition | Status | Body |
|-----------|--------|------|
| Tracked (default) | `200` | 1x1 transparent GIF (`image/gif`) |
| `send_image=0` or `ping=1` | `204` | empty |
| `debug=1` | `200` | JSON debug echo (`idsite`, `action_type`) instead of the GIF; the event is still published |
| Missing `idsite`/`rec`, or unparsable | `400` | error text |
| Per-IP rate limit exceeded | `429` | `rate limit exceeded` (+ `Retry-After` header) |
| NATS publish circuit open | `503` | `service unavailable` |
| Other publish failure | `500` | `failed to publish event` |

CORS is permissive (`Access-Control-Allow-Origin: *`), and an `OPTIONS` preflight
returns `200` — **as long as the client IP is within its rate-limit quota**.
`/track` (including `OPTIONS`) passes through the per-IP limiter first, so a
preflight from an over-quota IP receives `429`. Only `/health` is exempt from
rate limiting.

### Common parameters

The parser maps the parameters listed below into named fields of the published
event. Any parameter outside this set (and outside the `dimension{N}`
convention) is still forwarded — it is collected verbatim into an `extra` object
on the event, so nothing the client sends is dropped. The most-used named
parameters:

**Required**

| Param | Meaning |
|-------|---------|
| `idsite` | Site ID. Becomes the `{site_id}` token in the NATS subject. |
| `rec` | Must be present and non-empty. Matomo sends `1`; the value isn't otherwise validated. |

**Page / visit**

| Param | Meaning |
|-------|---------|
| `url` | Full URL of the page |
| `action_name` | Page/action title |
| `urlref` | Referrer URL |
| `_id` | Visitor ID (Matomo uses a 16-char hex string; not validated here) |
| `uid` | User ID |
| `cid` | Visitor UUID override |
| `new_visit` | Force a new visit (`1`) |
| `res` | Screen resolution (e.g. `1920x1080`) |
| `ua` | User agent override |
| `lang` | Accept-Language |
| `_cvar` / `cvar` | Visit / page custom variables (JSON) |
| `cs` | Page charset |
| `pv_id` | Page-view ID |
| `rand` | Cache-buster — recorded but has no effect on processing |
| `apiv` | Tracking API version |

**Events** (`e_c` + `e_a` required for an event)

| Param | Meaning |
|-------|---------|
| `e_c` | Event category |
| `e_a` | Event action |
| `e_n` | Event name |
| `e_v` | Event value (numeric) |

**Goals & ecommerce**

| Param | Meaning |
|-------|---------|
| `idgoal` | Goal ID (`0` = ecommerce) |
| `revenue` | Revenue |
| `ec_id` | Ecommerce order ID |
| `ec_items` | Ecommerce items (JSON) |
| `ec_st` / `ec_tx` / `ec_sh` / `ec_dt` | Subtotal / tax / shipping / discount |

**Site search / links / content**

| Param | Meaning |
|-------|---------|
| `search` / `search_cat` / `search_count` | Search keyword / category / result count |
| `link` | Outlink URL |
| `download` | Download URL |
| `c_n` / `c_p` / `c_t` / `c_i` | Content name / piece / target / interaction |

**Performance**

| Param | Meaning |
|-------|---------|
| `pf_net` / `pf_srv` / `pf_tfr` | Network / server / transfer time (ms) |
| `pf_dm1` / `pf_dm2` / `pf_onl` | DOM processing / completion / onload (ms) |

**Custom dimensions & plugins**

| Param | Meaning |
|-------|---------|
| `dimension{N}` | Custom dimension N (visit-scoped, or action-scoped when `ca=1`) |
| `fla` `java` `dir` `qt` `realp` `pdf` `wma` `gears` `ag` | Plugin availability flags |

**Override params** (forwarded as-is — see security note)

| Param | Meaning |
|-------|---------|
| `token_auth` | Auth token (forwarded on the event; **not** validated by this service) |
| `cip` | Override visitor IP |
| `cdt` | Override datetime |
| `country` / `region` / `city` / `lat` / `long` | Geolocation overrides |

> **Security note:** In Matomo these overrides require a valid `token_auth`.
> This service does **not** enforce that — it copies `token_auth`, `cip`, `cdt`,
> and the geo params onto the event and publishes them regardless. Any client
> can set them. If downstream consumers act on these fields, they must validate
> `token_auth` themselves (or you must enforce it in front of the tracker).

**Control**

| Param | Meaning |
|-------|---------|
| `ping` | Heartbeat (`1`) — classified as `action_type=heartbeat`; returns `204` (no GIF) |
| `send_image` | `0` to receive `204` instead of the GIF |
| `debug` | `1` to receive a JSON echo response instead of the GIF (the event is still published) |

For any parameter not listed here, see the
[Matomo Tracking API reference](https://developer.matomo.org/api-reference/tracking-api).

### Bulk tracking

Send multiple requests in one call with a JSON `POST` body and
`Content-Type: application/json`:

```json
{
  "requests": [
    "?idsite=1&rec=1&url=http://example.com/a&action_name=A",
    "?idsite=1&rec=1&url=http://example.com/b&action_name=B"
  ],
  "token_auth": "optional-shared-token"
}
```

Response (`200`):

```json
{ "status": "success", "tracked": 2, "failed": 0 }
```

`errors` is included when individual requests fail. If the circuit breaker is
open, the whole batch fast-fails with `503` (consistent with the single-request
path) rather than reporting partial success.

## `GET` `/health`

Returns `ok` with `200`. Used for liveness/readiness probes. **Not** rate
limited, so probes are never throttled.

## NATS output

Each tracked event is published to:

```
tracker.{site_id}.{action_type}
```

`action_type` is derived from the parameters:

| action_type | Triggered by |
|-------------|--------------|
| `heartbeat` | `ping=1` |
| `event` | `e_c` and `e_a` set |
| `goal` | `idgoal` set and ≠ `0` |
| `ecommerce` | `idgoal=0` |
| `search` | `search` set |
| `outlink` | `link` set |
| `download` | `download` set |
| `content_interaction` | `c_n` and `c_i` set |
| `content_impression` | `c_n` set |
| `pageview` | default |

NATS wildcard characters (`.`, `*`, `>`) in the site ID / action type are
sanitized so they cannot break subject routing. The payload is the parsed event
serialized as JSON: recognized parameters appear as named fields, and any other
parameters the client sent appear under an `extra` object, e.g.:

```json
{ "idsite": "1", "rec": "1", "action_type": "pageview",
  "extra": { "_idvc": "3", "_ref": "https://google.com" } }
```
