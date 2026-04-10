# Research: OpenAPI CLI & TUI Code Generation

**Date**: 2026-04-10
**Source**: `/research/` directory (6 research documents + architecture plan)

This document consolidates all decisions from the research phase, resolving
every technical unknown identified during planning.

---

## R1: SDK Generation Approach

**Decision**: Use oapi-codegen v2 as a Go library (not CLI binary), generating
types + client code.

**Rationale**: oapi-codegen produces idiomatic Go, supports OpenAPI 3.0 with
experimental 3.1, and can be invoked programmatically for full control over
the generation pipeline. Using it as a library lets us intercept output,
apply post-processing (retry wrappers, pagination helpers, auth middleware),
and avoid an external binary dependency.
(Source: research/05-oapi-codegen-integration.md)

**Alternatives considered**:
- **ogen** (github.com/ogen-go/ogen): More opinionated, less flexible for
  custom post-processing. Viable future alternative via config switch.
- **openapi-generator**: Java-based, heavier runtime, cross-language focus
  is unnecessary for a Go-only tool.
- **Custom codegen from scratch**: Maximum flexibility but enormous effort
  for diminishing returns vs. wrapping oapi-codegen.

**Post-processing pipeline** (applied after oapi-codegen output):
1. Retry wrappers with exponential backoff + jitter
2. Pagination helpers (ListAll + iterator variants)
3. Auth middleware injection
4. Enhanced error types (APIError, RateLimitError, ValidationError, etc.)
5. Custom code region markers

---

## R2: CLI/TUI Architecture Split

**Decision**: Adapter Pattern with a shared Operation Registry. Three adapters
(CLI, TUI, headless) read from the same registry and present different UIs.

**Rationale**: The Operation Registry is the central metadata store that maps
each API operation to its parameters, auth requirements, pagination config,
CLI aliases, TUI display mode, etc. Each adapter consumes this registry
independently. This enables pure CLI, pure TUI, and hybrid modes from one
codebase without conditional compilation or separate generation passes.
(Source: research/02-cli-tui-architecture.md)

**Mode detection logic** (highest to lowest precedence):
1. Explicit flags: `--tui`, `--no-interactive`, `-y`
2. Environment variable: `<PREFIX>_MODE=tui|cli|headless`
3. User config file: `mode: tui`
4. Auto-detection: agent environment -> headless; no TTY -> headless; TTY -> hybrid

**Hybrid mode behavior**: Cobra commands as skeleton. When TTY is detected
and required params are missing, launch inline Bubbletea components for
prompts. Full TUI explorer available as default when no subcommand given.

**Alternatives considered**:
- **Separate generation targets**: Would require maintaining two codegen
  paths and double the template count. Rejected.
- **TUI wrapping CLI**: Would couple TUI to CLI's text-based output.
  Rejected in favor of both reading from the same SDK.

---

## R3: Configuration System Design

**Decision**: Hybrid three-layer config. `cliford.yaml` (primary) +
`x-cliford-*` OpenAPI extensions (per-operation) + optional `cliford.go`
(Go DSL for power users). All parsed via Viper.

**Rationale**: YAML config covers 90% of use cases and is familiar to all
developers. OpenAPI extensions allow per-operation hints to live with the
spec they modify. The Go DSL enables complex conditional logic and typed
hooks for power users. The three layers compose cleanly with a defined
precedence: cliford.yaml operation-level > x-cliford-* > cliford.yaml
globals > built-in defaults.
(Source: research/04-configuration-systems.md)

**Generated app config**: Also Viper-based. Resolution: CLI flags > env
vars > config file (`~/.config/<app>/config.yaml`) > defaults. XDG Base
Directory spec for config location. Supports project-level overrides
(`./<app>.yaml` in cwd).

**Alternatives considered**:
- **OpenAPI-only** (Speakeasy approach): Too limiting for complex
  scenarios; pollutes spec with CLI concerns.
- **YAML-only**: Misses the ergonomic benefit of inline per-operation
  config in the spec.
- **Go DSL only**: Too high a barrier to entry for most users.

---

## R4: Authentication Architecture

**Decision**: Support all standard OpenAPI security schemes with layered
credential storage: OS keychain > encrypted file > env vars > config file.

**Rationale**: Speakeasy's auth approach is well-proven (research/01). We
adopt their credential resolution hierarchy (flags > env > keychain > config)
and extend it with multi-profile support, interactive TUI login flows, and
proactive OAuth token refresh.
(Source: research/03-authentication-systems.md)

**Supported auth methods** (from OpenAPI securitySchemes):
- Tier 1 (automatic): API Key, HTTP Basic, Bearer Token
- Tier 2 (configured): OAuth 2.0 (AuthCode, ClientCreds, DeviceCode), OIDC
- Tier 3 (via hooks): mTLS, custom schemes

**Credential storage backends** (priority order):
1. OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
2. Encrypted file (AES-256-GCM, for containers/CI/headless)
3. Environment variables (runtime only, no persistence)
4. Config file (with security warning)

**Libraries**: `github.com/zalando/go-keyring`, `golang.org/x/oauth2`

**Alternatives considered**:
- **Keychain only**: Not viable in containers/CI where no keychain exists.
- **Config file only**: Unacceptable security posture for credentials.
- **External secret managers (Vault, etc.)**: Out of scope for v1; can be
  supported via hooks.

---

## R5: Pagination Strategy

**Decision**: Support five pagination patterns, auto-detected from spec or
configured via `x-cliford-pagination` extension. Generate both `ListAll`
(fetch all pages) and iterator (memory-efficient streaming) variants.

**Rationale**: Real-world APIs use diverse pagination. Supporting offset,
page-number, cursor, Link header, and URL-based patterns covers the vast
majority. CLI surfaces this via `--all` and `--max-pages` flags. TUI uses
infinite scroll with cached back-navigation.
(Source: research/06-runtime-features.md, research/01-speakeasy-analysis.md)

**Patterns**:
| Pattern | Detection | CLI Flags |
|---------|-----------|-----------|
| Offset/Limit | `offset`/`limit` params | `--all`, `--limit`, `--offset` |
| Page Number | `page`/`per_page` params | `--all`, `--page`, `--per-page` |
| Cursor | `cursor` param + response field | `--all`, `--cursor` |
| Link Header | Standard `Link: <url>; rel="next"` | `--all` |
| URL-based | Response includes next URL | `--all` |

---

## R6: Retry Strategy

**Decision**: Exponential backoff with jitter. Configurable globally and
per-operation. Respects Retry-After and X-RateLimit-Reset headers.

**Rationale**: Industry standard approach. Speakeasy uses the same pattern
(research/01). Default retryable status codes: 408, 429, 500, 502, 503, 504.
Idempotency keys preserved across retries.
(Source: research/06-runtime-features.md)

**Defaults**:
- Initial interval: 500ms
- Max interval: 60s
- Max elapsed time: 5m
- Exponent: 1.5
- Jitter: enabled
- Connection errors: retried

**CLI flags**: `--no-retries`, `--retry-max-attempts`, `--retry-max-elapsed`

---

## R7: Error Handling Architecture

**Decision**: Typed error hierarchy with structured CLI output and styled
TUI error display. Auto-redact sensitive fields.

**Rationale**: Users need actionable errors. The hierarchy (APIError >
StructuredError, ValidationError, RateLimitError, NetworkError) allows
both programmatic handling in SDK consumers and clear display in CLI/TUI.
(Source: research/06-runtime-features.md)

**Error types**: APIError (base), StructuredError (JSON body), ValidationError
(field-level), RateLimitError (with retry info), NetworkError (connection/DNS).

**Display modes**: Pretty (human, default), JSON (structured), debug
(full request/response with redacted secrets).

---

## R8: TUI Component Mapping

**Decision**: Map API operation types to specific Bubbles components based
on the shape of the request and response.

**Rationale**: Different operations need different UI treatments. Lists need
tables with filtering. Single-resource views need detail panels. Create/update
operations need forms. This mapping is configurable via `x-cliford-tui`
extensions.
(Source: research/02-cli-tui-architecture.md)

**Default component mapping**:
| Operation Pattern | TUI Component | Bubble |
|-------------------|---------------|--------|
| List operations (returns array) | Filterable table | Table + List |
| Get single resource | Detail view | Viewport |
| Create/Update | Parameter form | TextInput + TextArea |
| Delete | Confirmation dialog | Custom (confirm bubble) |
| Long-running | Progress indicator | Spinner + Progress |
| File upload | File picker | FilePicker |

**Theme engine**: Developers define semantic colors (primary, accent, error,
success, etc.) and component preferences in cliford.yaml. Cliford translates
to Lipgloss styles. End users can override via their own config file.

---

## R9: Documentation Generation

**Decision**: Auto-generate Markdown docs (Cobra doc package) + llms.txt
(custom LLM-optimized format) + shell completions + JSON Schema for config.

**Rationale**: Cobra's doc package handles Markdown/man page generation.
The llms.txt format (research/02) provides flat, context-window-friendly
documentation for AI agents. JSON Schema enables IDE autocompletion for
config files.
(Source: research/02-cli-tui-architecture.md)

**Key practices from Cobra docs research**:
- Populate `Example` fields with concrete input/output demos
- Add `Long` descriptions with context and rationale
- Set `DisableAutoGenTag = true` for reproducible output
- Structure with H2/H3 for vector index chunking
- One file per command path

---

## R10: Custom Code Regions

**Decision**: Marker-based regions with `--- CUSTOM CODE START: <name> ---`
/ `--- CUSTOM CODE END: <name> ---` comments. Preserved via text extraction
before regeneration and reinsertion after.

**Rationale**: This is the industry standard pattern (Speakeasy uses it).
Simple, language-agnostic, and debuggable. Regions placed at strategic
extension points: imports, pre/post operation, error handling, flag init,
TUI model init, root command init, config init.
(Source: research/06-runtime-features.md)

**Safety mechanisms**:
- `cliford diff` previews changes before applying
- Backup stored in `.cliford/backup/` before each regeneration
- Warning emitted if a region would be lost (e.g., operation removed)
- Enabled via `features.customCodeRegions: true` in cliford.yaml

---

## R11: Distribution & Versioning

**Decision**: GoReleaser for cross-platform builds. SemVer injected via
ldflags. Optional Homebrew, WinGet, nFPM distribution channels.

**Rationale**: GoReleaser is the de facto standard for Go CLI distribution.
It handles cross-compilation, checksums, changelogs, and integrates with
GitHub Actions. Speakeasy uses the same approach (research/01).
(Source: research/01-speakeasy-analysis.md, research/06-runtime-features.md)

**Auto-version bumping** based on OpenAPI spec diff:
- Operations added -> minor
- Operations removed/signature changed -> major
- Metadata/descriptions only -> patch

---

## R12: Agent Mode

**Decision**: Auto-detect AI agent environments and switch to structured,
non-interactive output. Explicit `--agent` flag as override.

**Rationale**: Speakeasy pioneered this (research/01), auto-detecting Claude
Code, Cursor, Codex, Aider, and others. We adopt the same pattern and add
an explicit flag for environments not yet in the detection list.
(Source: research/01-speakeasy-analysis.md)

**Agent mode behavior**: JSON-only output, structured errors, no interactive
prompts, no TUI surfaces, no pager.
