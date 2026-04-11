# Research: Complete Generated App Wiring

**Branch**: `002-002-complete-generated` | **Date**: 2026-04-11  
**Resolves**: All NEEDS CLARIFICATION items from Technical Context

---

## Decision 1: Credential Storage Backend

**Decision**: Use `zalando/go-keyring` as the primary credential storage library. For CI/headless environments where the system keychain is unavailable (`go-keyring` returns `keyring.ErrUnsupportedPlatform` or a connection error), fall back to a custom AES-256-GCM encrypted file at `~/.config/<app-name>/credentials.enc`.

**Rationale**: `zalando/go-keyring` is actively maintained, has a simple API, and covers all three required platforms (macOS Keychain via `/usr/bin/security`, Linux Secret Service via D-Bus, Windows Credential Manager). `99designs/keyring` was considered but is no longer actively maintained. The missing encrypted-file fallback in `zalando/go-keyring` is addressed by a thin custom fallback layer generated alongside the credential resolver — this is straightforward to implement and keeps the dependency count low.

**Alternatives considered**:
- `99designs/keyring`: Has built-in encrypted-file fallback but is unmaintained — rejected.
- `99designs/keyring` fork: No authoritative community fork exists — rejected.
- Custom AES-GCM file only (no keychain): Violates Constitution Principle VIII — rejected.

**Key code pattern**:
```go
import "github.com/zalando/go-keyring"

// Store
err := keyring.Set(serviceName, accountName, secret)

// Retrieve — fall back to encrypted file on error
secret, err := keyring.Get(serviceName, accountName)
if err != nil {
    secret, err = encryptedFileStore.Get(serviceName, accountName)
}
```

**Credential key structure**: service=`<app-name>`, account=`<SCHEME_NAME>_<CREDENTIAL_TYPE>` (e.g., service=`petstore`, account=`BEARERAUTH_TOKEN`).

---

## Decision 2: Hook Mechanism

**Decision**: Shell command hooks (JSON on stdin, exit-code protocol) for simple cases. `hashicorp/go-plugin` (gRPC subprocess) for advanced cross-platform Go/polyglot plugins. Native Go `.so` plugins are **not used**.

**Rationale**: Native Go plugins (`plugin.Open`) are excluded from Windows — a required target platform. They also require identical Go toolchain versions and CGO, making distribution and reproduction fragile. `hashicorp/go-plugin` solves all these problems: it communicates via gRPC over a child process, works on all platforms, supports any language, and isolates plugins from the host process. It is the pattern used by Terraform, Vault, and Nomad.

**Alternatives considered**:
- Native Go `.so` plugins: Fast, but Windows-incompatible and build-fragile — rejected.
- Shell hooks only: Simpler, but can't share typed Go data structures efficiently — excluded as the only mechanism.

**Hook protocol**:
```
Shell hooks: stdin = JSON({operation_id, method, url, headers, body, response?}), stdout ignored, exit != 0 aborts
go-plugin hooks: gRPC interface defined in proto, plugin binary discovered via config path
```

---

## Decision 3: Retry Implementation

**Decision**: Custom `RetryTransport` implementing `http.RoundTripper` using stdlib only (no external retry library).

**Rationale**: The codebase already has `internal/sdk/retry_enhancer.go` that generates a `RetryTransport`. It already supports configurable `MaxAttempts`, `InitialInterval`, `MaxInterval`, `Exponent`, jitter, retriable status codes, and `Retry-After` header respect. The work is to **wire** this existing generator into the pipeline and ensure defaults (3 attempts, 1s initial, 30s max) are applied when no explicit `RetryConfig` is present in the registry.

**Alternatives considered**:
- `cenkalti/backoff`: Battle-tested but adds external dependency unnecessarily — rejected.
- `avast/retry-go`: Adds abstraction overhead — rejected.

---

## Decision 4: OAuth2 Client Credentials

**Decision**: `golang.org/x/oauth2/clientcredentials` for token exchange and refresh, combined with an in-memory token cache protected by `sync.Mutex`.

**Rationale**: `golang.org/x/oauth2` is the canonical Go OAuth2 library, handles all token lifecycle details, and provides `clientcredentials.Config.TokenSource()` which automatically refreshes tokens. The `oauth2.Transport` wrapper injects tokens into every HTTP request. The existing `internal/cli/auth.go` already generates auth command scaffolding; this work wires the OAuth2 client credentials flow into the generated `http.Client`.

**Token caching pattern**:
```go
config := &clientcredentials.Config{
    ClientID: ..., ClientSecret: ..., TokenURL: ..., Scopes: ...,
}
tokenSource := oauth2.ReuseTokenSource(nil, config.TokenSource(ctx))
client := &http.Client{
    Transport: &oauth2.Transport{Source: tokenSource, Base: retryTransport},
}
```

**Cached tokens stored in**: OS keychain (via zalando/go-keyring, with encrypted-file fallback) with `<SCHEME>_TOKEN` key.

---

## Decision 5: Table Output Column Selection

**Decision**: `x-cliford-display: true` on response schema properties selects default columns; all properties shown if none marked. `--fields` flag provides override. Use `text/tabwriter` from stdlib (no external dependency).

**Rationale**: `text/tabwriter` is in the standard library and produces clean tab-aligned output. The `x-cliford-display` extension is already the established Cliford pattern for per-operation overrides (Principle I: "Configuration that extends behavior MUST be layered on top of the spec"). No new extension mechanism needed.

**Alternatives considered**:
- `olekukonko/tablewriter`: Popular but adds external dependency for something stdlib can handle — rejected.
- First-N-properties heuristic: Non-deterministic for users — rejected.

---

## Decision 6: Interactive Prompts

**Decision**: `bufio.Scanner` on stdin for simple missing-arg prompts. `survey/v2` or equivalent NOT introduced — stdlib is sufficient for single-value prompts.

**Rationale**: The spec (Assumption) already states `bufio.Scanner` is sufficient. The prompts are simple `"flag-name: "` text prompts, not full TUI forms. Adding `survey/v2` would be an unnecessary dependency for the use case.

---

## Codebase Status: Existing Generators

The following generators already exist and need **wiring/completion**, not creation from scratch:

| Component | File | Status |
|-----------|------|--------|
| Auth generator | `internal/cli/auth.go` | Exists; needs pipeline wiring |
| Retry enhancer | `internal/sdk/retry_enhancer.go` | Exists; needs default wiring |
| Pagination enhancer | `internal/sdk/pagination_enhancer.go` | Exists; needs CLI flag generation |
| Hooks | `internal/hooks/` | Exists; needs runtime wiring |
| Distribution | `internal/distribution/` | Exists; placeholder replacement needed |
| Docs | `internal/docs/` | Exists; needs man page + llms.txt |
| Pipeline | `internal/pipeline/` | Exists; orchestration gaps |

**Key insight**: The majority of this feature is about completing partially-implemented generators and wiring them through the pipeline, not building new systems from scratch.
