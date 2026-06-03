# API Reference

The tracker implements the [Matomo Tracking API](https://developer.matomo.org/api-reference/tracking-api)
parameter set. Matomo SDKs (including the JS tracker) work once their tracker
URL is pointed at this server's **`/track`** endpoint — the default
`matomo.php` path is intentionally not served (it returns `404`).

## `GET` / `POST` `/track`

Records a tracking request. Parameters may be sent as a query string (GET) or a
URL-encoded form body (POST). The minimum required parameters are `idsite` and
`rec=1`.

### Response behavior

| Condition | Status | Body |
|-----------|--------|------|
| Tracked (default) | `200` | 1x1 transparent GIF (`image/gif`) |
| `send_image=0` or `ping=1` | `204` | empty |
| `debug=1` | `200` | JSON debug echo (`idsite`, `action_type`) instead of the GIF |
| Missing `idsite`/`rec`, or unparsable | `400` | error text |
| Per-IP rate limit exceeded | `429` | `rate limit exceeded` (+ `Retry-After` header) |
| NATS publish circuit open | `503` | `service unavailable` |
| Other publish failure | `500` | `failed to publish event` |

CORS is permissive (`Access-Control-Allow-Origin: *`), and `OPTIONS` preflight
returns `200`.

### Common parameters

The parser recognizes the broad subset of Matomo parameters listed below and
maps them into the published event. Parameters outside this set (and outside the
`dimension{N}` / plugin-flag conventions) are accepted by the request but are
**not** carried into the NATS payload. The most-used recognized parameters:

**Required**

| Param | Meaning |
|-------|---------|
| `idsite` | Site ID. Becomes the `{site_id}` token in the NATS subject. |
| `rec` | Must be `1` for the request to be recorded. |

**Page / visit**

| Param | Meaning |
|-------|---------|
| `url` | Full URL of the page |
| `action_name` | Page/action title |
| `urlref` | Referrer URL |
| `_id` | Visitor ID (16-char hex) |
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
| `ping` | Heartbeat (`1`) — updates visit duration, no new action; returns `204` |
| `send_image` | `0` to receive `204` instead of the GIF |
| `debug` | `1` to receive a JSON echo instead of tracking output |

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

NATS wildcard characters in the site ID / action type are sanitized so they
cannot break subject routing. The payload is the full parsed event as JSON.
