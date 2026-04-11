# Feature Specification: Complete Generated App Wiring

**Feature Branch**: `002-002-complete-generated`  
**Created**: 2026-04-10  
**Status**: Draft  

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Auth Credentials Applied to Requests (Priority: P1)

A developer using a generated CLI or TUI connects to a protected API. They run the app and provide credentials (API key, Bearer token, or Basic auth) through environment variables or a config file. Every request the app makes automatically includes the correct `Authorization` or custom header — no manual header flags required per call.

**Why this priority**: Without working auth, the generated app cannot communicate with real APIs. All other functionality depends on this.

**Independent Test**: Set an env var for an API key scheme, run a list operation, observe the Authorization header is present in verbose output. Delivers a fully working generated client for any authenticated API.

**Acceptance Scenarios**:

1. **Given** a spec with `apiKey` in header security, **When** the user sets the corresponding env var, **Then** every request includes that key in the correct header
2. **Given** a spec with `http bearer` security, **When** the user provides a token via config or env, **Then** every request includes `Authorization: Bearer <token>`
3. **Given** a spec with `http basic` security, **When** credentials are provided, **Then** requests include the correct Base64-encoded `Authorization: Basic` header
4. **Given** no credentials are configured, **When** a protected endpoint is called, **Then** the app exits with a clear "authentication required" message naming the missing credential

---

### User Story 2 - Retry on Transient Failures (Priority: P1)

A developer runs a generated CLI against a flaky or rate-limited API. Instead of hard-failing on a 429, 503, or network timeout, the app automatically retries with exponential backoff. The developer does not need to write retry logic or wrap the command in a shell loop.

**Why this priority**: Retry wiring is declared in the registry but never called — generated apps silently fail on transient errors today.

**Independent Test**: Point the generated CLI at a mock server that returns 503 twice then 200. Run a GET operation. Confirm the command succeeds after retries.

**Acceptance Scenarios**:

1. **Given** an operation with retry config enabled, **When** the server returns 503, **Then** the app retries up to the configured maximum before reporting failure
2. **Given** exponential backoff is configured, **When** retries occur, **Then** wait intervals double between attempts (with optional jitter)
3. **Given** a non-retriable status code (e.g., 400, 401), **When** the server responds, **Then** the app does not retry and exits immediately with the error

---

### User Story 3 - Per-Request Timeout Enforcement (Priority: P1)

A developer calls a generated CLI command. If the API server does not respond within the configured timeout, the command exits promptly with a clear timeout error rather than hanging indefinitely.

**Why this priority**: Without timeouts, generated apps hang forever on unresponsive servers, breaking CI pipelines and scripting.

**Independent Test**: Configure a 1-second timeout, call an operation against a server that delays 5 seconds, confirm the command exits with a timeout error in ~1 second.

**Acceptance Scenarios**:

1. **Given** a per-operation timeout is configured, **When** the request exceeds it, **Then** the app cancels the request and prints a timeout error
2. **Given** a global timeout in the root config, **When** no operation-level override exists, **Then** the global timeout applies

---

### User Story 4 - Debug/Verbose Request Tracing (Priority: P2)

A developer is troubleshooting an API call. They pass `--verbose` (or `-v`) to any generated command and see the full request (method, URL, headers, body) and response (status, headers, body) printed to stderr before the formatted output.

**Why this priority**: Essential for debugging auth issues, payload problems, and API errors. Does not block core functionality but dramatically improves developer experience.

**Independent Test**: Run any generated list command with `--verbose`. Confirm request URL, method, and headers appear on stderr.

**Acceptance Scenarios**:

1. **Given** `--verbose` flag is passed, **When** a request is made, **Then** request line, headers (with auth redacted), and body are printed to stderr
2. **Given** `--verbose` is passed, **When** a response is received, **Then** status code, response headers, and body are printed to stderr
3. **Given** verbose mode is not enabled, **When** a request is made, **Then** no request/response details appear on stderr

---

### User Story 5 - Interactive Prompts for Missing Required Args (Priority: P2)

A developer runs a generated CLI command without providing all required arguments. Rather than printing a terse usage error, the app interactively prompts them for each missing required flag value in the terminal.

**Why this priority**: Eliminates the frustrating cycle of trial-and-error with required flags. Improves discoverability and usability.

**Independent Test**: Run a DELETE command without the required `--id` flag. Confirm the app prompts "id: " and accepts input, then proceeds.

**Acceptance Scenarios**:

1. **Given** a required path/query param is missing, **When** the command is run interactively, **Then** the app prompts the user for each missing value
2. **Given** the user provides a value at the prompt, **When** the form is submitted, **Then** the request proceeds with the provided value
3. **Given** the command is run non-interactively (stdin is not a TTY), **When** a required arg is missing, **Then** the app prints the usage error and exits non-zero

---

### User Story 6 - Cursor/Offset Pagination in CLI (Priority: P2)

A developer runs a generated list command against a paginated API. They can fetch all pages with a `--all` flag, or navigate page-by-page with `--page-token` / `--page` / `--offset` flags. Results are printed in order across pages.

**Why this priority**: Most real-world list endpoints are paginated. Without working pagination, developers only see the first page of results.

**Independent Test**: Use a paginated mock API. Run `list --all`. Confirm all items across multiple pages appear in stdout.

**Acceptance Scenarios**:

1. **Given** an operation with cursor pagination config, **When** `--all` is passed, **Then** the app fetches all pages and outputs all results
2. **Given** a cursor is returned in a response, **When** `--page-token <cursor>` is passed on the next call, **Then** the correct next page is fetched
3. **Given** offset pagination is configured, **When** `--offset` and `--limit` are passed, **Then** the correct page is fetched

---

### User Story 7 - OAuth 2.0 Client Credentials Flow (Priority: P2)

A developer using a generated app with an OAuth 2.0 client credentials spec provides their `client_id` and `client_secret` via config or env vars. The app automatically exchanges them for a token, caches it, refreshes it before expiry, and attaches it to every request.

**Why this priority**: OAuth client credentials is the most common machine-to-machine auth pattern. The flow is defined in the spec but the token exchange is not implemented.

**Independent Test**: Configure a `client_id`/`client_secret`, run a protected operation, confirm the app fetches and caches a token and uses it.

**Acceptance Scenarios**:

1. **Given** a spec with OAuth2 client credentials, **When** credentials are configured, **Then** the app fetches a token before the first protected request
2. **Given** a token is cached, **When** it is still valid, **Then** the app reuses the cached token
3. **Given** a cached token is expired or near expiry, **When** a request is made, **Then** the app refreshes the token automatically

---

### User Story 8 - Before/After Request Hooks (Priority: P3)

A developer wants to add custom logic to every request — e.g., inject a custom header, log requests to a file, or transform the response. They register a before-request or after-response hook in a config file. The generated app calls the hook on every operation.

**Why this priority**: Enables extensibility without re-running the generator. Valuable for power users and enterprise integrations.

**Independent Test**: Register a before-request hook that adds a custom header. Run any operation and confirm the header appears in verbose output.

**Acceptance Scenarios**:

1. **Given** a before-request hook is configured, **When** any operation runs, **Then** the hook is called with the request context before sending
2. **Given** an after-response hook is configured, **When** any operation completes, **Then** the hook is called with the response before outputting results

---

### User Story 9 - Semantic Version Stamped into Generated Binary (Priority: P3)

A developer runs `myapp version` on a generated CLI. Instead of a hardcoded placeholder, the output shows the real version from the spec's `info.version` field and a build timestamp.

**Why this priority**: Operators need to know what version of the client they are running for debugging and auditing.

**Independent Test**: Generate a CLI from a spec with `info.version: "2.1.0"`. Run `myapp version`. Confirm "2.1.0" appears in output.

**Acceptance Scenarios**:

1. **Given** the spec has an `info.version` field, **When** `myapp version` is run, **Then** output includes the spec version
2. **Given** the spec has `info.title` and `info.description`, **When** `myapp version` or `myapp --help` is run, **Then** title and description are shown

---

### User Story 10 - Distribution Placeholder Removal (Priority: P3)

A developer uses the generated `Makefile` and `Dockerfile` to build and distribute their CLI. All `OWNER` and `PROJECT` placeholders have been replaced with the correct values derived from the spec title or the generator invocation, so the build works without manual edits.

**Why this priority**: Placeholder values cause build failures immediately after generation. The generated artifacts should be usable without modification.

**Independent Test**: Generate a project, run `make build`. Confirm no `OWNER` or `PROJECT` placeholder strings remain in generated files.

**Acceptance Scenarios**:

1. **Given** the generator is invoked with a project name, **When** `Makefile` and `Dockerfile` are generated, **Then** all `OWNER`/`PROJECT` placeholders are replaced
2. **Given** no project name is provided, **When** generation runs, **Then** the spec title (slugified) is used as the project name

---

### User Story 11 - Table Output for List Operations (Priority: P3)

A developer runs a generated list command. Instead of raw JSON, results are displayed as a formatted ASCII table with column headers derived from the response schema property names.

**Why this priority**: Raw JSON is unreadable at a glance. Table output is the expected default for list operations.

**Independent Test**: Run a list operation with a JSON array response. Confirm aligned column headers and rows appear.

**Acceptance Scenarios**:

1. **Given** a GET operation returning an array, **When** results are displayed, **Then** output is a table with column headers from response schema properties
2. **Given** `--output json` is passed, **When** results are displayed, **Then** raw JSON is output instead
3. **Given** the response array is empty, **When** displayed, **Then** the table headers are shown with a "no results" message

---

### User Story 12 - Body Flag Name Collision Fix (Priority: P3)

A developer uses a generated CLI for an endpoint that has both a path parameter `id` and a body property `id`. Both map to flags without collision. One gets `--id` (the path param), the other gets `--body-id` (the body property), and both flags work correctly.

**Why this priority**: A bug — today, flag name collision causes a panic or silent incorrect behavior for endpoints with overlapping parameter and body property names.

**Independent Test**: Generate a CLI for an endpoint with `id` in path and `id` in body. Confirm `--id` sets the path param and `--body-id` sets the body property.

**Acceptance Scenarios**:

1. **Given** a path param `id` and body property `id`, **When** the command is generated, **Then** `--id` maps to the path param and `--body-id` maps to the body property
2. **Given** no collision exists, **When** the command is generated, **Then** body properties use their natural kebab-case names without a `body-` prefix

---

### User Story 13 - Global Parameters via FeaturesConfig (Priority: P3)

A developer wants to inject a custom header (e.g., `X-Tenant-ID`) into every request without passing it as a flag every time. They set it once in the config file under a `global_params` section. The generated app reads it and applies it to all requests.

**Why this priority**: Many APIs require tenant or workspace headers on every call. Without global params support, developers must repeat flags on every invocation.

**Independent Test**: Set `global_params.headers.X-Tenant-ID = "acme"` in config. Run any operation in verbose mode. Confirm the header is present.

**Acceptance Scenarios**:

1. **Given** a global header is configured, **When** any operation runs, **Then** the header is included in every request
2. **Given** a global query param is configured, **When** any operation runs, **Then** the query param is appended to every request URL

---

### User Story 14 - Server URL Override via Config (Priority: P3)

A developer runs a generated CLI against a staging environment instead of production. They set `server_url` in the config file or `MYAPP_SERVER_URL` env var. All requests go to the overridden URL without needing to rebuild or regenerate.

**Why this priority**: Essential for multi-environment workflows. The TUI already reads `BaseURL` but gets it from spec at generation time; the CLI doesn't support override.

**Independent Test**: Set `MYAPP_SERVER_URL=http://localhost:8080`. Run any operation. Confirm requests go to `localhost:8080`.

**Acceptance Scenarios**:

1. **Given** `server_url` is set in config, **When** any operation runs, **Then** the configured URL is used instead of the spec default
2. **Given** the env var `<APP>_SERVER_URL` is set, **When** any operation runs, **Then** the env var takes precedence over config file

---

### User Story 15 - Man Page Generation (Priority: P3)

A developer installs a generated CLI on a Linux/macOS system. They run `man myapp` or `man myapp-list-users` and see a proper man page with the command description, flags, and examples.

**Why this priority**: Man pages are the standard documentation mechanism for CLI tools on Unix systems. The `doc/` Cobra command is stubbed.

**Independent Test**: Run `myapp generate-docs --format man`. Confirm `.1` man page files are created in the output directory.

**Acceptance Scenarios**:

1. **Given** the `generate-docs` command is run with `--format man`, **When** it completes, **Then** one `.1` file per command is created in the output directory
2. **Given** `--format markdown` is passed, **When** it completes, **Then** one `.md` file per command is created

---

### User Story 16 - Operation Descriptions in Help Text (Priority: P3)

A developer runs `myapp list-users --help`. The help output includes the full description from the OpenAPI spec's `description` field, not just the summary. Long descriptions are word-wrapped at 80 characters.

**Why this priority**: Summary text alone is often insufficient. Full descriptions help developers understand edge cases and required context.

**Independent Test**: Generate a CLI from a spec where an operation has a multi-sentence `description`. Run `--help`. Confirm the full description appears.

**Acceptance Scenarios**:

1. **Given** an operation has a `description` field in the spec, **When** `--help` is shown, **Then** the full description is displayed below the summary
2. **Given** a description exceeds 80 characters per line, **When** displayed, **Then** it is word-wrapped at 80 characters

---

### User Story 17 - Confirm Prompt for Destructive Operations (Priority: P3)

A developer runs a generated DELETE or other destructive operation. Before executing, the app displays a confirmation prompt: "Are you sure you want to delete user abc123? [y/N]". The operation proceeds only if confirmed.

**Why this priority**: Prevents accidental destructive operations, especially in production environments.

**Independent Test**: Run a DELETE operation without `--yes`. Confirm a confirmation prompt appears. Respond `n`. Confirm the request is not made.

**Acceptance Scenarios**:

1. **Given** an operation is marked `x-cliford-confirm: true` or is a DELETE, **When** run interactively, **Then** a confirmation prompt appears before the request
2. **Given** `--yes` flag is passed, **When** the operation runs, **Then** the prompt is skipped
3. **Given** stdin is not a TTY, **When** a confirm operation runs without `--yes`, **Then** the app exits with an error

---

### User Story 18 - TUI Server URL from Config (Priority: P4)

A developer uses a generated TUI and wants it to connect to a staging server. They set `server_url` in the config file. The TUI uses the configured URL for all API calls rather than the hardcoded URL embedded at generation time.

**Why this priority**: P4 because the TUI already reads `BaseURL` from the generated `OperationItem` struct; this is about making that value dynamic at runtime.

**Independent Test**: Override `server_url` in config. Launch the TUI, execute an operation, verify the request goes to the override URL in the TUI's output.

**Acceptance Scenarios**:

1. **Given** `server_url` is overridden in config, **When** the TUI makes a request, **Then** the configured URL is used
2. **Given** no override exists, **When** the TUI makes a request, **Then** the URL from the spec servers list is used

---

### Edge Cases

- What happens when the spec has no security schemes but an operation requires auth? The app should print a clear error at startup.
- What happens when a retry config has `MaxElapsedTime` of 0? Treat as no time limit (retry indefinitely up to max attempts).
- What happens when a paginated response does not include the expected `next` key? Treat as the last page and stop.
- What happens when both `--body` JSON and individual body flags are provided? Individual flags override corresponding keys in `--body`.
- What happens when verbose mode is used and the response body is binary? Print `[binary response, N bytes]` instead of body content.
- What happens when a confirm prompt is shown but the app receives SIGINT? Exit cleanly without making the request.
- What happens when a global param conflicts with a per-operation param? Per-operation value takes precedence.
- What happens when `server_url` override is an invalid URL? The app exits at startup with a clear validation error.

## Requirements *(mandatory)*

### Functional Requirements

#### Auth & Security

- **FR-001**: The generated app MUST read auth credentials from environment variables and config files according to each security scheme type defined in the spec; env var names follow the pattern `<APP>_<SCHEME_NAME>_<CREDENTIAL_TYPE>` (e.g., `PETSTORE_APIKEYAUTH_API_KEY`, `PETSTORE_OAUTH_CLIENT_SECRET`), derived entirely from spec metadata
- **FR-002**: The generated app MUST apply the correct `Authorization` header format for `http bearer`, `http basic`, and `apiKey` header schemes automatically on every request
- **FR-003**: The generated app MUST report a clear, actionable error message when required credentials are not configured, naming the missing credential
- **FR-004**: For OAuth 2.0 client credentials, the generated app MUST exchange `client_id`/`client_secret` for an access token before the first protected request
- **FR-005**: For OAuth 2.0 client credentials, the generated app MUST cache the access token and refresh it before expiry

#### Reliability

- **FR-006**: The generated app MUST retry requests on configured retriable status codes using exponential backoff; when no explicit retry config is present, defaults are: 3 total attempts, initial interval 1s, max interval 30s, no elapsed time cap; when `MaxElapsedTime` is explicitly set to 0, treat as no time limit (retry indefinitely up to max attempts); retry parameters MUST be overridable at runtime via FeaturesConfig
- **FR-007**: The generated app MUST enforce per-operation and global request timeouts, cancelling requests that exceed the configured duration
- **FR-008**: The generated app MUST NOT retry on non-retriable status codes (e.g., 400, 401, 403, 404)

#### Interactivity & UX

- **FR-009**: The generated CLI MUST include a `--verbose` / `-v` flag on every command that prints the full request and response to stderr
- **FR-010**: In verbose mode, the generated app MUST redact secret header values (auth tokens, API keys) replacing them with `[REDACTED]`
- **FR-011**: The generated CLI MUST interactively prompt for missing required arguments when run in a TTY
- **FR-012**: The generated CLI MUST skip interactive prompts and print usage errors when stdin is not a TTY
- **FR-013**: Operations marked for confirmation (DELETE or `x-cliford-confirm`) MUST display a confirmation prompt before executing
- **FR-014**: A `--yes` / `-y` flag MUST be available on confirmation operations to skip the prompt

#### Output & Display

- **FR-015**: GET operations returning arrays MUST display results as formatted tables; default columns are properties marked `x-cliford-display: true` in the spec, or all properties if none are marked; a `--fields` flag accepts a comma-separated list to override the column set
- **FR-016**: An `--output` flag MUST be available on all operations with values `table` (default for arrays), `json`, and `detail`
- **FR-017**: Operation descriptions from the spec MUST appear in `--help` output, word-wrapped at 80 characters
- **FR-018**: The generated `version` command MUST output the spec's `info.version`, `info.title`, and build timestamp

#### Pagination

- **FR-019**: GET operations with pagination config MUST accept pagination flags (`--page-token`, `--offset`, `--page`, `--limit`)
- **FR-020**: An `--all` flag on paginated operations MUST automatically fetch all pages and output combined results

#### Config & Extensibility

- **FR-021**: The generated app MUST support a `server_url` config key and corresponding env var to override the spec's server URL at runtime
- **FR-022**: The generated app MUST support `global_params` in config to inject headers and query params into every request
- **FR-023**: The generated app MUST support `before_request` and `after_response` hooks in two forms: shell command hooks (request/response data as JSON on stdin; non-zero exit code aborts request) and advanced subprocess hooks via hashicorp/go-plugin (gRPC-based RPC, cross-platform, any language)
- **FR-024**: FeaturesConfig MUST be wired into the generated root command so feature flags can be toggled via config; required config keys: `features.retry.enabled` (default: true), `features.retry.max_attempts` (default: 3), `features.retry.initial_interval` (default: "1s"), `features.pagination.enabled` (default: true), `features.pagination.default_page_size` (default: 20), `features.hooks.enabled` (default: false), `features.verbose.redact_headers` (default: ["Authorization", "X-Api-Key"])

#### Cliford Tool

- **FR-025**: The Cliford generator MUST accept a project name flag/argument to replace `OWNER` and `PROJECT` placeholders in generated `Makefile` and `Dockerfile`
- **FR-026**: The Cliford generator MUST generate man pages when invoked with a `--docs` or `generate-docs` subcommand

#### Documentation & Completions

- **FR-030**: Generated apps MUST auto-generate an `llms.txt` file (LLM-optimized command/flag summary) alongside man pages and Markdown docs via the `generate-docs` subcommand
- **FR-031**: Generated apps MUST produce shell completions for bash, zsh, and fish without error; shell completion generation MUST be included as a quality gate in the generated app

#### Bug Fixes

- **FR-032**: Body property flag names MUST NOT collide with path/query parameter flag names; colliding body props MUST use the `body-` prefix
- **FR-033**: The TUI MUST read the server URL from the runtime config rather than a value hardcoded at generation time
- **FR-034**: All generated distribution files (`Makefile`, `Dockerfile`, `goreleaser.yml`) MUST have `OWNER` and `PROJECT` placeholders replaced before writing

### Key Entities

- **SecurityCredential**: Represents a resolved auth credential value (key, token, client secret) loaded from config/env, associated with a security scheme name
- **RetryPolicy**: Encapsulates strategy, intervals, max attempts, and retriable status codes — resolved from registry `RetryConfig` at runtime
- **PaginationState**: Tracks the current cursor/offset/page across multi-page fetches for a single `--all` invocation
- **HookContext**: The data passed to before/after hooks — request metadata, operation ID, elapsed time, response status
- **FeaturesConfig**: Top-level config struct governing which optional features (retry, hooks, pagination, verbose) are enabled per generated app

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can run a generated CLI against a real API requiring Bearer token auth without any manual header flags — 100% of authenticated requests succeed when the env var is set
- **SC-002**: A generated CLI retries correctly on transient errors — a command succeeds after 2 server-side 503s with exponential backoff within the configured max elapsed time
- **SC-003**: A generated CLI command exits within 500ms of the configured timeout when the server is unresponsive, rather than hanging indefinitely
- **SC-004**: Running any generated list command produces a readable table output by default — column headers match the response schema property names
- **SC-005**: Zero `OWNER` or `PROJECT` placeholder strings remain in any generated file after invoking the generator with a project name
- **SC-006**: `--verbose` output on any command shows the full request URL, method, and non-secret headers before the response is printed
- **SC-007**: A paginated list operation with `--all` returns the same total item count as manually fetching each page and concatenating results
- **SC-008**: A DELETE operation without `--yes` always shows a confirmation prompt; no HTTP request is made if the user responds `n`
- **SC-009**: `myapp version` output contains the version string from the spec's `info.version` field
- **SC-010**: All generated commands display the full OpenAPI `description` field in `--help` output, word-wrapped at 80 characters

## Assumptions

- The existing technology stack (Go, Cobra, Bubbletea, Viper, kin-openapi) is fixed — no new frameworks will be introduced
- OAuth 2.0 Device Code flow is out of scope for this feature; only Client Credentials will be implemented
- The generated app's config file format is YAML/TOML via Viper — no new config format will be introduced
- Credentials are stored in the OS keychain by default (macOS Keychain, Linux Secret Service, Windows Credential Manager) via zalando/go-keyring; environments without keychain access fall back to an encrypted file; plain-text config file is the last-resort fallback with a generated security documentation warning — credential resolution order is: flags > env > OS keychain > encrypted file > config file
- Hook implementations support two modes: shell command hooks (subprocess, JSON on stdin, exit code signals abort) for simple cases and advanced subprocess hooks via hashicorp/go-plugin (gRPC-based, cross-platform, language-agnostic) for advanced use; native Go `.so` plugins are not used due to Windows incompatibility and build constraint fragility
- Man page generation uses Cobra's built-in `doc` package — no third-party man page library is needed
- Shell completions use Cobra's built-in completion generation (bash, zsh, fish)
- Cliford's generation pipeline hooks (`before:generate`, `after:sdk`, etc. per Constitution Principle VI) are a separate feature from generated app runtime hooks (`before_request`, `after_response`) and are explicitly out of scope for this feature
- Table output uses a simple ASCII table writer available in the Go standard library or a lightweight dependency already in the module graph
- The `--verbose` flag will not attempt to decompress or pretty-print binary response bodies
- Interactive prompts use `bufio.Scanner` on stdin — no TUI library is required for CLI-mode prompts
- Confirmation prompts default to "No" when Enter is pressed without input, consistent with convention
- The TUI's server URL override will be implemented by reading Viper config in the generated `main.go` init function and passing it to the TUI model

## Clarifications

### Session 2026-04-11

- Q: How should credentials be stored in the generated app's config file — plain-text YAML/TOML, OS keychain, or file-permission-restricted plain text? → A: OS keychain by default (via zalando/go-keyring) with encrypted-file fallback for headless environments; plain-text config file is the last-resort fallback with a security documentation warning
- Q: What mechanism should before_request/after_response hooks use — shell subprocess, Go plugin, or both? → A: Both — shell command hooks (JSON on stdin, non-zero exit code aborts request) for simple cases and advanced subprocess hooks via hashicorp/go-plugin (gRPC-based, cross-platform, any language) for advanced use; native Go `.so` plugins are not used due to Windows incompatibility
- Q: What naming convention should generated credential env vars follow? → A: `<APP>_<SCHEME_NAME>_<CREDENTIAL_TYPE>` (e.g., `PETSTORE_APIKEYAUTH_API_KEY`), derived entirely from spec metadata
- Q: How should table output determine which response properties to display as columns? → A: Properties marked `x-cliford-display: true` in the spec are the default columns; if none are marked, all properties are shown; `--fields` flag overrides the column set
- Q: What are the default retry parameters when no explicit retry config is provided? → A: 3 total attempts; initial interval 1s; max interval 30s; no elapsed time cap
