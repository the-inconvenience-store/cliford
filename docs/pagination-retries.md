# Pagination, Retries, and Error Handling Reference

This is the technical reference for pagination, retry, timeout, and error
handling behavior in generated CLI applications.

## Pagination

### Supported patterns

| Pattern | Detection | Extension Type |
|---------|-----------|---------------|
| Offset/Limit | `offset`/`limit` parameters | `x-cliford-pagination: {type: offset}` |
| Page Number | `page`/`per_page` parameters | `x-cliford-pagination: {type: page}` |
| Cursor | `cursor` parameter + response field | `x-cliford-pagination: {type: cursor}` |
| Link Header | RFC 5988 `Link: <url>; rel="next"` | `x-cliford-pagination: {type: link}` |
| URL-based | Response includes `next_url` field | `x-cliford-pagination: {type: url}` |

### CLI flags

Paginated operations receive these additional flags:

| Flag | Description |
|------|-------------|
| `--all` | Fetch all pages and output combined results |
| `--max-pages N` | Maximum pages to fetch with `--all` (default: 1000) |
| `--cursor` / `--page-token` | Cursor value for the next page (cursor pagination) |
| `--offset` | Offset value (offset pagination) |
| `--page` | Page number (page-based pagination) |
| `--limit` | Number of items per page (default from spec or 20) |

The specific pagination parameter names depend on the `x-cliford-pagination`
configuration in the OpenAPI spec.

### Behavior of --all

When `--all` is passed, the generated command:

1. Sends the initial request
2. Extracts the results array from the response using the configured JSONPath
3. Extracts the next cursor/offset from the response
4. Repeats until no next cursor is returned or `--max-pages` is reached
5. Outputs the combined results as a single array

### SDK helpers

The generated SDK includes these pagination utilities:

- **`PageIterator[T]`**: Lazy iterator that fetches pages on demand.
  Call `Next()` for one item at a time, or `All()` to collect everything.
- **`PaginateAll[T]`**: Convenience function that fetches all pages and
  returns a combined slice.
- **`ExtractJSONPath`**: Extracts values from JSON using simple path
  expressions (`$.data`, `$.meta.next`).
- **Link header parser**: Extracts the `rel="next"` URL from RFC 5988
  Link headers.

## Retries

### Strategy

Exponential backoff with jitter:

```
delay = min(initialInterval * exponent^attempt + jitter, maxInterval)
```

### Default configuration

| Setting | Default |
|---------|---------|
| Max attempts | 3 |
| Initial interval | 1 second |
| Max interval | 30 seconds |
| Max elapsed time | 0 (no limit) |
| Exponent | 2.0 |
| Jitter | 25% randomization |
| Retryable status codes | 408, 429, 500, 502, 503, 504 |
| Retry connection errors | Yes |

A `MaxElapsedTime` of 0 means there is no time-based limit on retries. The
retry loop stops only when `MaxAttempts` is reached.

### Non-retryable status codes

Status codes not in the retryable list (such as 400, 401, 403, 404) return
immediately without retrying. This prevents wasting time on client errors
that will not succeed on retry.

### CLI flags

All commands include retry flags:

| Flag | Description |
|------|-------------|
| `--no-retries` | Disable retries for this request |
| `--retry-max-attempts N` | Override max retry attempts |
| `--retry-max-elapsed STR` | Override max elapsed time (e.g., `5m`) |

### Runtime configuration

Retry parameters can be overridden at runtime via the generated app's config
file or environment variables:

```yaml
# ~/.config/<app>/config.yaml
features:
  retry:
    enabled: true
    max_attempts: 5
    initial_interval: 2s
```

Or via environment variables:

```bash
export MYAPP_FEATURES_RETRY_MAX_ATTEMPTS=5
```

### Header respect

The retry middleware respects these response headers:

- **`Retry-After`**: Both integer seconds (`Retry-After: 30`) and HTTP date
  (`Retry-After: Thu, 10 Apr 2026 12:00:00 GMT`) formats.
- **`X-RateLimit-Reset`**: Unix timestamp indicating when the rate limit
  resets.

When these headers are present, the retry delay uses the server-specified
value instead of the calculated backoff.

### Idempotency

If the request includes an `Idempotency-Key` header, the same key is
preserved across all retry attempts.

## Timeouts

### Default timeout

All requests have a default timeout of 30 seconds, set on the shared HTTP
client.

### Per-operation timeout

If an operation has a `timeout` configured in the registry (via
`x-cliford-retries` or `cliford.yaml`), a `context.WithTimeout` is applied
to that request, overriding the global default.

### Runtime override

The timeout can be overridden at runtime:

```yaml
# ~/.config/<app>/config.yaml
request_timeout: 60s
```

Or via environment variable:

```bash
export MYAPP_REQUEST_TIMEOUT=60s
```

## Error handling

### Error types

The generated SDK produces typed errors:

| Type | HTTP Code | Extra Fields |
|------|-----------|--------------|
| `APIError` | Any 4xx/5xx | StatusCode, Body, RequestID, Operation |
| `ValidationError` | 422 | Field-level errors (`[{field, message}]`) |
| `RateLimitError` | 429 | RetryAfter, Limit, Remaining, Reset |
| `NetworkError` | N/A | Connection and DNS failure details |

### Error display

In the default output format, errors appear as human-readable messages:

```
Error: HTTP 422: Validation failed (request-id: req_abc123)
  - email: must be a valid email address
  - name: is required
```

In JSON mode (`-o json`), the error is structured:

```json
{"error": {"statusCode": 422, "message": "Validation failed", "fields": [...]}}
```

In verbose mode (`--verbose`), the full request and response are printed to
stderr with sensitive headers redacted.
