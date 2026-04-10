# Pagination & Retries

Generated apps handle pagination, retries, and errors automatically.

## Pagination

### Supported Patterns

| Pattern | Detection | Config Extension |
|---------|-----------|-----------------|
| Offset/Limit | `offset`/`limit` params | `x-cliford-pagination: {type: offset}` |
| Page Number | `page`/`per_page` params | `x-cliford-pagination: {type: page}` |
| Cursor | `cursor` param + response field | `x-cliford-pagination: {type: cursor}` |
| Link Header | Standard RFC 5988 `Link: <url>; rel="next"` | `x-cliford-pagination: {type: link}` |
| URL-based | Response includes `next_url` field | `x-cliford-pagination: {type: url}` |

### CLI Flags

Paginated operations get these additional flags:

```
--all              Fetch all pages automatically
--max-pages N      Limit pages when using --all (0 = unlimited)
```

Standard pagination params (offset, limit, cursor, page) remain as regular flags.

### Usage

```bash
# Fetch page 1 (default)
myapp items list --limit 20

# Fetch all pages
myapp items list --all

# Fetch up to 5 pages
myapp items list --all --max-pages 5

# Cursor-based
myapp items list --cursor "abc123" --limit 50
```

### OpenAPI Extension

Configure pagination per-operation in your OpenAPI spec:

```yaml
paths:
  /items:
    get:
      x-cliford-pagination:
        type: cursor
        input:
          cursor:
            name: cursor
            in: query
          limit:
            name: limit
            in: query
            default: 20
        output:
          results: "$.data"
          nextCursor: "$.meta.nextCursor"
          totalCount: "$.meta.total"
```

### SDK Helpers

The generated SDK includes:

- **`PageIterator[T]`** — lazy, memory-efficient iteration. Call `Next()` to
  get items one at a time, or `All()` to collect everything.
- **`PaginateAll[T]`** — convenience function that fetches all pages and
  returns a combined slice.
- **`ExtractJSONPath`** — extract values from JSON responses using simple
  JSONPath expressions (`$.data`, `$.meta.next`).
- **Link header parser** — RFC 5988 `rel="next"` extraction.

## Retries

### Strategy

Exponential backoff with jitter (industry standard):

```
delay = min(initialInterval * exponent^attempt + jitter, maxInterval)
```

### Defaults

| Setting | Default |
|---------|---------|
| Max attempts | 3 |
| Initial interval | 500ms |
| Max interval | 60s |
| Max elapsed time | 5 minutes |
| Exponent | 1.5 |
| Jitter | 25% randomization |
| Retryable status codes | 408, 429, 500, 502, 503, 504 |
| Retry connection errors | Yes |

### CLI Flags

All commands get retry flags:

```
--no-retries              Disable retries for this request
--retry-max-attempts N    Override max retry attempts
--retry-max-elapsed STR   Override max elapsed time (e.g., "5m")
```

### Header Respect

The retry middleware automatically respects:

- **`Retry-After`** — both seconds (`Retry-After: 30`) and HTTP date
  (`Retry-After: Thu, 10 Apr 2026 12:00:00 GMT`) formats.
- **`X-RateLimit-Reset`** — Unix timestamp indicating when the rate limit
  resets.

### Idempotency

If the request includes an `Idempotency-Key` header, the same key is
preserved across all retry attempts.

### Configuration

Global defaults in `cliford.yaml`:

```yaml
features:
  retries:
    enabled: true
    maxAttempts: 3
```

Per-operation in OpenAPI spec:

```yaml
x-cliford-retries:
  enabled: true
  maxAttempts: 5
  statusCodes: [429, 503]
```

## Error Handling

### Error Types

The generated SDK produces typed errors:

| Type | HTTP Code | Extra Fields |
|------|-----------|--------------|
| `APIError` | Any 4xx/5xx | StatusCode, Body, RequestID, Operation |
| `ValidationError` | 422 | Field-level errors (`[{field, message}]`) |
| `RateLimitError` | 429 | RetryAfter, Limit, Remaining, Reset |
| `NetworkError` | N/A | Connection/DNS failure details |

### Display

**Pretty mode** (default):
```
Error: HTTP 422: Validation failed (request-id: req_abc123)
  - email: must be a valid email address
  - name: is required
```

**JSON mode** (`-o json`):
```json
{"error": {"statusCode": 422, "message": "Validation failed", "fields": [...]}}
```

**Debug mode** (`--debug`):
```
[DEBUG] POST https://api.example.com/users
[DEBUG] Content-Type: application/json
[DEBUG] Authorization: Bear...cdef
[DEBUG] Response: 422
[DEBUG] Request-ID: req_abc123
[DEBUG] Body: {"error": ...}
```

All sensitive headers are redacted in debug output.
