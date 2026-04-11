# Data Model: Complete Generated App Wiring

**Branch**: `002-002-complete-generated` | **Date**: 2026-04-11

---

## Entity: SecurityCredential

Represents a resolved auth credential value loaded from the credential resolution chain for a specific security scheme.

**Fields**:
- `SchemeName string` — matches `SecurityScheme.Name` from the registry
- `Type SecuritySchemeType` — apiKey, http, oauth2, openIdConnect
- `Value string` — the credential value (API key, bearer token, client secret, etc.)
- `ExpiresAt *time.Time` — for tokens; nil for static credentials
- `Source CredentialSource` — flags | env | keychain | encrypted_file | config_file

**Lifecycle**: Resolved at startup (auth init); cached for the process lifetime; refreshed proactively if `ExpiresAt` is within a configurable window (default: 60s before expiry).

**Validation rules**:
- `Value` must not be empty after resolution; error if empty for a required scheme
- `ExpiresAt` must be set for OAuth2 tokens (logged as warning if missing)

---

## Entity: RetryPolicy

Encapsulates retry behavior resolved from the registry `RetryConfig` with defaults applied.

**Fields**:
- `MaxAttempts int` — total attempts including first try; default 3
- `InitialInterval time.Duration` — default 1s
- `MaxInterval time.Duration` — default 30s
- `Exponent float64` — default 2.0
- `Jitter bool` — default true (25% random jitter)
- `StatusCodes []int` — default [408, 429, 500, 502, 503, 504]
- `RetryConnectionErrors bool` — default true
- `MaxElapsedTime *time.Duration` — nil means no cap

**State transitions**: Stateless per-request; a new attempt counter is created per HTTP call.

---

## Entity: PaginationState

Tracks multi-page fetch progress for a single `--all` invocation.

**Fields**:
- `Type PaginationType` — offset | page | cursor | link | url
- `CurrentCursor string` — current position (cursor/URL) or empty
- `CurrentPage int` — current page number (page/offset modes)
- `CurrentOffset int` — current offset (offset mode)
- `TotalFetched int` — count of items fetched so far
- `Done bool` — true when last page detected

**State transitions**:
```
Initial → Fetching → (more pages?) → Fetching
                                   → Done
```
Done when: `OutputNextKey` JSONPath returns empty/null, or fetched count < limit, or total reached.

---

## Entity: HookContext

Data passed to before/after hooks. Serialised as JSON for shell hooks; passed as gRPC message for go-plugin hooks.

**Before-request fields**:
- `OperationID string`
- `Method string` — HTTP method
- `URL string` — fully resolved URL with path params substituted
- `Headers map[string]string` — auth headers with values as `[REDACTED]` for sensitive keys
- `Body []byte` — request body (nil for GET/DELETE)
- `Timestamp time.Time`

**After-response fields** (extends before-request):
- `StatusCode int`
- `ResponseHeaders map[string]string`
- `ResponseBody []byte`
- `ElapsedMs int64`
- `Error string` — empty on success

**Abort protocol (shell hooks)**: exit code != 0 aborts the request; stderr message is shown to user.

---

## Entity: FeaturesConfig

Top-level config struct governing optional feature enablement per generated app. Stored in the generated app's config file.

**Fields**:
- `Retry RetryFeatureConfig`
  - `Enabled bool` — default true
  - `Override *RetryPolicy` — per-app defaults overriding registry values
- `Pagination PaginationFeatureConfig`
  - `Enabled bool` — default true
  - `DefaultPageSize int` — default 20
- `Hooks HooksFeatureConfig`
  - `Enabled bool` — default false
  - `BeforeRequest []HookDef`
  - `AfterResponse []HookDef`
- `Verbose VerboseFeatureConfig`
  - `RedactHeaders []string` — headers to always redact (default: Authorization, X-Api-Key)

**HookDef fields**:
- `Type HookType` — shell | go-plugin
- `Command string` — for shell hooks: the command string
- `PluginPath string` — for go-plugin: path to plugin binary

---

## Entity: GlobalParams

Injected into every request without per-call flags.

**Fields**:
- `Headers map[string]string` — e.g., `{"X-Tenant-ID": "acme"}`
- `QueryParams map[string]string` — appended to every request URL

**Resolution**: Read from Viper config under `global_params` key; per-operation param with the same name takes precedence.

---

## Generated Code Entities (in generated app)

These types appear in the **generated app's source code**, not in Cliford itself.

### CredentialResolver

Generated into `internal/auth/resolver.go` in the generated app.

**Responsibilities**: Implements the 5-tier credential resolution chain (flags → env → keychain → encrypted file → config file) for each security scheme.

### HTTPClientFactory

Generated into `internal/client/factory.go` in the generated app.

**Responsibilities**: Assembles the layered `http.Client`:
```
http.Client{Transport: AuthTransport{
    Base: RetryTransport{
        Base: http.DefaultTransport,
        Config: retryPolicy,
    },
    Credentials: resolvedCreds,
}}
```

### TableRenderer

Generated into `internal/output/table.go` in the generated app.

**Responsibilities**: Renders `[]map[string]any` as a `text/tabwriter` table using the operation's display column list (from `x-cliford-display` or all properties).
