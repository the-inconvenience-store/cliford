# Runtime Features Research

> Pagination, retries, error handling, server configuration, global parameters, and SemVer.

## Pagination

### Patterns to Support

Based on Speakeasy's approach and common API patterns:

| Pattern | Description | Detection |
|---------|-------------|-----------|
| **Offset/Limit** | `?offset=0&limit=20` | `x-cliford-pagination: offset` or auto-detect `offset`/`limit` params |
| **Page Number** | `?page=1&per_page=20` | Auto-detect `page`/`per_page` params |
| **Cursor-based** | `?cursor=abc123` | `x-cliford-pagination: cursor` or `cursor`/`next_cursor` in response |
| **Link Header** | `Link: <url>; rel="next"` | Standard HTTP Link header parsing |
| **URL-based** | Response includes `next_url` field | Configure via JSONPath to next URL |

### CLI Pagination Flags

Generated commands for paginated endpoints get:

```
--all              Fetch all pages (auto-paginate)
--max-pages N      Maximum pages to fetch (default: unlimited with --all)
--page N           Specific page number
--limit N          Items per page
--cursor STR       Start cursor (for cursor-based)
```

### TUI Pagination

In TUI mode, pagination is handled through the **Paginator** and **List** bubbles:

- Infinite scroll: Load next page when user scrolls to bottom
- Page indicator: Show current page / total (if known)
- Loading spinner while fetching next page
- Cached pages for instant back-navigation

### SDK Pagination Helpers

```go
// Generated for each paginated endpoint
func (c *Client) ListUsersAll(ctx context.Context, params *ListUsersParams) ([]User, error) {
    var all []User
    page := params
    for {
        resp, err := c.ListUsers(ctx, page)
        if err != nil {
            return all, err
        }
        all = append(all, resp.Users...)
        if resp.NextCursor == "" {
            break
        }
        page = &ListUsersParams{Cursor: resp.NextCursor, Limit: params.Limit}
    }
    return all, nil
}

// Iterator pattern for memory-efficient streaming
func (c *Client) ListUsersIter(ctx context.Context, params *ListUsersParams) *PageIterator[User] {
    return &PageIterator[User]{
        fetch: func(cursor string) ([]User, string, error) {
            p := *params
            p.Cursor = cursor
            resp, err := c.ListUsers(ctx, &p)
            if err != nil {
                return nil, "", err
            }
            return resp.Users, resp.NextCursor, nil
        },
    }
}
```

### Configuration

```yaml
# cliford.yaml
features:
  pagination:
    enabled: true
    defaultLimit: 20
    maxLimit: 100

# Per-operation in OpenAPI spec
x-cliford-pagination:
  type: cursor           # offset | page | cursor | link | url
  input:
    cursor:
      name: cursor
      in: query
    limit:
      name: limit
      in: query
      default: 20
  output:
    results: "$.data"           # JSONPath to results array
    nextCursor: "$.meta.next"   # JSONPath to next cursor
    totalCount: "$.meta.total"  # JSONPath to total (optional)
```

---

## Retries

### Strategy

Exponential backoff with jitter (industry standard):

```
delay = min(initialInterval * exponent^attempt + jitter, maxInterval)
```

### Configuration

```yaml
# Global defaults in cliford.yaml
features:
  retries:
    enabled: true
    strategy: exponential-backoff
    initialInterval: 500ms
    maxInterval: 60s
    maxElapsedTime: 5m
    exponent: 1.5
    jitter: true
    retryableStatusCodes:
      - 408  # Request Timeout
      - 429  # Too Many Requests
      - 500  # Internal Server Error
      - 502  # Bad Gateway
      - 503  # Service Unavailable
      - 504  # Gateway Timeout
    retryConnectionErrors: true
```

```yaml
# Per-operation override in OpenAPI
x-cliford-retries:
  enabled: true
  maxAttempts: 5
  statusCodes: [429, 503]
```

### CLI Retry Flags

```
--no-retries              Disable retries for this request
--retry-max-attempts N    Override max retry attempts
--retry-max-elapsed STR   Override max elapsed time (e.g., "5m")
```

### Retry-After Header Compliance

When server responds with `Retry-After` header:
- If it's a date: wait until that date
- If it's seconds: wait that many seconds
- Respect it if within `maxElapsedTime`, otherwise fail

### Rate Limit Awareness

For 429 responses:
- Check `Retry-After` header first
- Check `X-RateLimit-Reset` header
- Fall back to exponential backoff
- Show progress in TUI (spinner with "Rate limited, retrying in Xs...")

### Idempotency

For endpoints with idempotency keys:
- Preserve the same idempotency key across all retry attempts
- Generate idempotency keys automatically for POST/PUT if configured

---

## Error Handling

### Error Type Hierarchy

```go
// Base error type
type APIError struct {
    StatusCode int
    Message    string
    Body       []byte
    RequestID  string
    Headers    http.Header
    Operation  string
    Retried    int  // Number of retries attempted
}

// Structured error (when API returns JSON error body)
type StructuredError struct {
    APIError
    Code    string           `json:"code"`
    Details []ErrorDetail    `json:"details,omitempty"`
}

// Validation error
type ValidationError struct {
    APIError
    Fields []FieldError `json:"fields"`
}

// Rate limit error
type RateLimitError struct {
    APIError
    RetryAfter time.Duration
    Limit      int
    Remaining  int
    Reset      time.Time
}

// Network error (connection failed, DNS, timeout)
type NetworkError struct {
    Operation string
    Cause     error
    Retried   int
}
```

### CLI Error Display

```
# Pretty mode (default)
Error: Failed to create user (HTTP 422)
  Validation errors:
    - email: must be a valid email address
    - name: is required
  Request ID: req_abc123

# JSON mode
{"error": {"status": 422, "message": "Validation failed", "fields": [...]}}

# Debug mode (--debug)
[DEBUG] POST https://api.example.com/users
[DEBUG] Request Body: {"email": "invalid"}
[DEBUG] Response: 422 Unprocessable Entity
[DEBUG] Response Body: {"error": ...}
[DEBUG] Request ID: req_abc123
```

### TUI Error Display

Errors show as styled notification bars at the bottom of the TUI:

```
┌─────────────────────────────────────┐
│ Create User                          │
│                                      │
│ Email: invalid-email█                │
│                                      │
│ ┌─ Error ──────────────────────────┐ │
│ │ email: must be a valid address   │ │
│ └──────────────────────────────────┘ │
└─────────────────────────────────────┘
```

---

## Server Configuration

### Multiple Server Support

OpenAPI specs can define multiple servers:

```yaml
servers:
  - url: https://api.example.com
    description: Production
  - url: https://staging.api.example.com
    description: Staging
  - url: http://localhost:8080
    description: Local development
```

### Server Selection

```
# CLI flag
myapp users list --server https://staging.api.example.com
myapp users list --server staging  # By description/alias

# Environment variable
MYAPP_SERVER_URL=https://staging.api.example.com

# Config file
server:
  url: https://api.example.com
  
# Profile-based
profiles:
  production:
    server:
      url: https://api.example.com
  staging:
    server:
      url: https://staging.api.example.com
```

### Server Variables

OpenAPI server templates with variables:

```yaml
servers:
  - url: https://{tenant}.api.example.com/{version}
    variables:
      tenant:
        default: acme
      version:
        default: v1
        enum: [v1, v2]
```

Generated as:
```
--server-tenant acme
--server-version v2
# Or env: MYAPP_SERVER_TENANT=acme
```

---

## Global Parameters

Some parameters apply to every request:

### Sources of Global Params

1. **OpenAPI `parameters` at path level** - Apply to all operations on that path
2. **OpenAPI `components/parameters`** - Reusable across operations
3. **`x-cliford-global-params`** - Custom global params
4. **Built-in** - User-Agent, Accept, Content-Type

### Configuration

```yaml
# cliford.yaml
globalParams:
  - name: X-Request-ID
    in: header
    generate: uuid  # Auto-generate UUID per request
  - name: X-Tenant-ID
    in: header
    source: config  # Read from config/env
    envVar: MYAPP_TENANT_ID
  - name: Accept-Language
    in: header
    default: en-US
    flag: --lang
```

### CLI Global Flags

```
# Persistent flags available on all commands
--server URL           API server URL
--timeout DURATION     Request timeout
--output-format FMT    Output format (json/yaml/table/pretty)
--no-interactive       Disable interactive prompts
--debug                Enable debug logging
--dry-run              Show request without executing
-y, --yes              Skip all confirmations
```

---

## SemVer Automation

### Version Management

Generated apps use SemVer (Semantic Versioning) automatically:

```yaml
# cliford.yaml
app:
  version: 1.2.3  # Current version
  
versioning:
  strategy: semver
  source: git-tag  # git-tag | config | auto
```

### Version Injection

Version is injected at build time via ldflags:

```go
// Generated in cmd/root.go
var (
    version = "dev"     // Set via -ldflags
    commit  = "none"
    date    = "unknown"
)

func init() {
    rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
}
```

GoReleaser handles this automatically:

```yaml
# .goreleaser.yaml (generated)
builds:
  - ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
```

### Version Bumping

Cliford can auto-bump versions based on changes:

```
cliford version bump patch    # 1.2.3 -> 1.2.4
cliford version bump minor    # 1.2.3 -> 1.3.0
cliford version bump major    # 1.2.3 -> 2.0.0
cliford version bump auto     # Analyze changes, decide automatically
```

Auto-bump logic:
- New operations added -> minor bump
- Operations removed/changed -> major bump
- Bug fixes / config changes -> patch bump
- Based on diff between current and previous OpenAPI spec

---

## Custom Code Regions

### How They Work

Custom code regions are specially marked sections in generated files that Cliford preserves across regeneration:

```go
// --- CUSTOM CODE START: imports ---
import "github.com/myorg/mylib"
// --- CUSTOM CODE END: imports ---

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
    // --- CUSTOM CODE START: ListUsers:pre ---
    ctx = mylib.InjectTracing(ctx)
    // --- CUSTOM CODE END: ListUsers:pre ---
    
    // Generated code...
    
    // --- CUSTOM CODE START: ListUsers:post ---
    metrics.RecordLatency(time.Since(start))
    // --- CUSTOM CODE END: ListUsers:post ---
}
```

### Region Placement

Cliford generates custom code regions at these points:

| Location | Region Name | Purpose |
|----------|-------------|---------|
| File header | `imports` | Additional imports |
| Before each operation | `<OpID>:pre` | Pre-request logic |
| After each operation | `<OpID>:post` | Post-request logic |
| Error handlers | `<OpID>:error` | Custom error handling |
| CLI command init | `<OpID>:flags` | Additional flags |
| TUI model init | `<OpID>:init` | Additional TUI state |
| Root command | `root:init` | Global initialization |
| Config init | `config:init` | Custom config logic |

### Enable/Disable

```yaml
# cliford.yaml
features:
  customCodeRegions: true  # Default: false for clean first-gen
```

When disabled, no region markers are generated (cleaner code). When enabled, markers appear at all extension points.

### Safety

- Cliford warns if a custom code region would be lost (region exists in old file but not in new template)
- Backup of previous generation stored in `.cliford/backup/`
- `cliford diff` command shows what would change before regenerating
