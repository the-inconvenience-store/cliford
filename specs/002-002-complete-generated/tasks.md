# Tasks: Complete Generated App Wiring

**Input**: Design documents from `/specs/002-002-complete-generated/`  
**Branch**: `002-002-complete-generated`  
**Tech Stack**: Go 1.22+, Cobra, Bubbletea, Viper, kin-openapi, oapi-codegen, zalando/go-keyring, golang.org/x/oauth2, hashicorp/go-plugin  
**Structure**: `internal/` (generators) + `templates/` (code generation templates) + `pkg/registry/` (types)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story from spec.md (US1–US18)
- All paths relative to `/Users/samstevens/asot/cliford/`

---

## Phase 1: Setup (Dependencies)

**Purpose**: Add new third-party dependencies required by the implementation plan.

- [x] T001 Add `github.com/zalando/go-keyring` to `go.mod` and run `go mod tidy`
- [x] T002 [P] Add `golang.org/x/oauth2` to `go.mod` and run `go mod tidy`
- [x] T003 [P] Add `github.com/hashicorp/go-plugin` to `go.mod` and run `go mod tidy`

---

## Phase 2: Foundational (SDK-First Architecture Refactor)

**Purpose**: Establish the layered `http.Client` (Auth → Retry → Default) and make generated `RunE` functions call oapi-codegen SDK methods rather than constructing HTTP requests inline. This is the critical architectural prerequisite that all user story work builds on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete — all P1/P2/P3 stories depend on the layered client and SDK-first call pattern.

- [x] T004 Refactor generated `RunE` functions in `internal/cli/generator.go` to call oapi-codegen SDK client methods instead of constructing `http.Request` objects inline
- [x] T005 Create `templates/sdk/factory.go.tmpl` to generate `internal/client/factory.go` in the output app — assembles `http.Client{Transport: AuthTransport{Base: RetryTransport{Base: http.DefaultTransport}}}`
- [x] T006 Update `templates/cli/main.go.tmpl` to construct the layered `http.Client` once at startup and inject it into the SDK client before passing to Cobra commands
- [x] T007 [P] Update `internal/pipeline/` orchestrator to invoke `auth_enhancer`, `retry_enhancer`, and `oauth_enhancer` in the correct order before CLI/TUI generator runs
- [x] T008 Update golden files after SDK-First refactor: `UPDATE_GOLDEN=1 go test ./tests/unit/... -count=1` from `/Users/samstevens/asot/cliford`

**Checkpoint**: `go build ./...` and `go test ./...` must pass. Generated app compiles and responds to `--help`.

---

## Phase 3: User Story 1 — Auth Credentials Applied to Requests (P1) 🎯 MVP

**Goal**: Every generated app reads credentials from the 5-tier resolution chain and injects the correct auth header on every request automatically.

**Independent Test**: Set `PETSTORE_BEARERAUTH_TOKEN=test123`. Run any generated list command with `--verbose`. Confirm `Authorization: Bearer test123` appears in stderr (redacted to `[REDACTED]` in actual output — token presence confirmed via `auth status` command).

- [x] T009 Complete `internal/cli/auth.go` credential resolver to implement 5-tier chain: CLI flags → env var (`<APP>_<SCHEME>_<TYPE>`) → zalando/go-keyring keychain → AES-256-GCM encrypted file (`~/.config/<app>/credentials.enc`) → plain-text config file
- [x] T010 [P] Implement custom AES-256-GCM encrypted-file fallback store in `templates/cli/auth_store.go.tmpl` (activated when `go-keyring` returns an error)
- [x] T011 [P] Generate `<APP>_<SCHEME_NAME>_<CREDENTIAL_TYPE>` env var names from registry `SecurityScheme` metadata in `internal/cli/auth.go`
- [x] T012 [US1] Generate clear auth error messages per FR-003 in `internal/cli/auth.go`: print scheme name, type, and exact env var name when credentials missing
- [x] T013 Wire `CredentialResolver` output into `AuthTransport` in `templates/sdk/factory.go.tmpl` — inject correct header per scheme type (Bearer, Basic Base64, apiKey header/query)
- [x] T014 Update golden files after auth wiring: `UPDATE_GOLDEN=1 go test ./tests/unit/... -count=1`
- [ ] T014a [P] [US1] Add unit test for `CredentialResolver` 5-tier chain in `tests/unit/auth_resolver_test.go`: all tiers exercised, priority order verified, missing credential error message validated — write test first, confirm it fails before T009 implementation

**Checkpoint**: Generated app for a Bearer-auth spec sends correct `Authorization` header when env var is set; prints actionable error when missing.

---

## Phase 4: User Story 2 — Retry on Transient Failures (P1)

**Goal**: Generated app automatically retries on 429/503/network errors with exponential backoff using defaults when no `RetryConfig` is present in registry.

**Independent Test**: Point generated CLI at a mock server returning 503 twice then 200. Run a GET operation. Command succeeds after retries.

- [x] T015 Complete `internal/sdk/retry_enhancer.go` to apply defaults (3 attempts, 1s initial interval, 30s max interval, 2.0 exponent, 25% jitter) when `op.Retries` is nil in `OperationMeta`; treat `MaxElapsedTime == 0` as no time limit
- [x] T016 Confirm `RetryTransport` is wired as inner transport layer in `templates/sdk/factory.go.tmpl` (below `AuthTransport`)
- [x] T017 Verify `internal/sdk/retry_enhancer.go` non-retriable bypass: status codes 400, 401, 403, 404 must not trigger retry
- [ ] T017a [US2] Wire `features.retry.enabled`, `features.retry.max_attempts`, and `features.retry.initial_interval` from FeaturesConfig into `RetryTransport` construction in `templates/sdk/factory.go.tmpl` so retry parameters are overridable at runtime via Viper config
- [ ] T017b [P] [US2] Add unit test for `RetryTransport` in `tests/unit/retry_transport_test.go`: 3-attempt 503 succeeds; 400 no retry; interval doubling; `MaxElapsedTime == 0` treated as no limit — write test first, confirm it fails before T015 implementation

**Checkpoint**: Unit test confirms 3 retry attempts on 503 with exponential intervals; single attempt on 400.

---

## Phase 5: User Story 3 — Per-Request Timeout Enforcement (P1)

**Goal**: Generated app exits promptly with a timeout error if the API server does not respond within the configured duration.

**Independent Test**: Configure 1s timeout, call operation against 5s-delayed mock server. Command exits with timeout error in ~1s.

- [x] T018 Update `templates/cli/main.go.tmpl` to apply per-operation `OperationMeta.Timeout` to `http.Client.Timeout` when constructing the layered client
- [ ] T019 [P] Add `<APP>_REQUEST_TIMEOUT` env var and `request_timeout` Viper config key to generated root command in `templates/cli/root.go.tmpl` — used as global default when no per-operation timeout is set
- [ ] T019a [P] [US3] Add unit test for timeout enforcement in `tests/unit/timeout_test.go`: configure 1s timeout against a mock that delays 5s, verify command exits within 500ms of configured timeout — write test first

**Checkpoint**: `go test ./...` passes; generated app honours 1s timeout flag in integration scenario.

---

## Phase 6: User Story 4 — Debug/Verbose Request Tracing (P2)

**Goal**: `--verbose`/`-v` prints full request and response to stderr, with sensitive headers redacted.

**Independent Test**: Run any generated list command with `--verbose`. Confirm method, URL, and `Authorization: [REDACTED]` on stderr.

- [x] T020 Add `--verbose`/`-v` persistent flag to generated root command in `internal/cli/generator.go`
- [x] T021 Create `templates/sdk/verbose_transport.go.tmpl` generating `VerboseTransport` that wraps any `http.RoundTripper` and prints request/response to stderr when `--verbose` is set
- [x] T022 Implement header redaction in `VerboseTransport` template: redact `Authorization`, `X-Api-Key`, and any header whose name contains `secret`, `token`, `key`, or `password` (case-insensitive) — replace values with `[REDACTED]`
- [x] T023 Wire `VerboseTransport` as outermost layer in `templates/sdk/factory.go.tmpl` (wrapping `AuthTransport` chain)

**Checkpoint**: `--verbose` on a generated command prints request line to stderr; auth header value is `[REDACTED]`.

---

## Phase 7: User Story 5 — Interactive Prompts for Missing Required Args (P2)

**Goal**: Generated CLI prompts for each missing required flag when run interactively; prints usage error when stdin is not a TTY.

**Independent Test**: Run a DELETE command without `--id`. App prompts `id: `. Provide value. Request proceeds.

- [x] T024 Create `internal/cli/prompts.go` to generate per-command `promptMissingArgs()` helper functions into `templates/cli/prompts.go.tmpl`
- [x] T025 Wire `promptMissingArgs()` at start of each generated `RunE` function in `internal/cli/generator.go` — called before request execution
- [x] T026 Add TTY detection in generated prompts template (`templates/cli/prompts.go.tmpl`): call `term.IsTerminal(int(os.Stdin.Fd()))` and skip prompt + print Cobra usage error when stdin is not a TTY

**Checkpoint**: Running command without required `--id` in a TTY shows `id: ` prompt; running with stdin redirected from file prints usage error.

---

## Phase 8: User Story 6 — Cursor/Offset Pagination in CLI (P2)

**Goal**: Paginated list operations accept `--page-token`/`--offset`/`--page`/`--limit` flags and an `--all` flag that fetches every page.

**Independent Test**: Run `list --all` against a paginated mock. Confirm all items across all pages appear in stdout.

- [x] T027 Complete `internal/sdk/pagination_enhancer.go` to generate typed `--page-token`, `--offset`, `--page`, `--limit` flags from `OperationMeta.Pagination` config
- [x] T028 Add `--all` flag to generated commands where `OperationMeta.Pagination != nil` in `internal/cli/generator.go`
- [x] T029 Implement `--all` page-fetch loop in `templates/cli/paginate.go.tmpl`: drive `PageIterator`, accumulate results across pages, output combined slice
- [x] T030 Wire pagination flags into generated `RunE` functions in `internal/cli/generator.go` — pass current page params to SDK call, update state from response
- [ ] T030a [US6] Wire `features.pagination.enabled` and `features.pagination.default_page_size` from FeaturesConfig into pagination flag generation in `templates/cli/paginate.go.tmpl` so page size is overridable at runtime via Viper config

**Checkpoint**: `--all` on a 3-page mock returns full result count; `--page-token <cursor>` fetches correct page.

---

## Phase 9: User Story 7 — OAuth 2.0 Client Credentials Flow (P2)

**Goal**: Generated app exchanges `client_id`/`client_secret` for a token automatically, caches it, and refreshes before expiry.

**Independent Test**: Configure `client_id`/`client_secret`. Run protected operation. Confirm token fetch occurs, is cached (second call skips fetch), and is used in `Authorization` header.

- [x] T031 Create `internal/sdk/oauth_enhancer.go` to generate `OAuthTokenSource` using `clientcredentials.Config` from `golang.org/x/oauth2/clientcredentials`
- [x] T032 [P] Generate `ReuseTokenSource` wrapper with in-memory cache (`sync.Mutex` + `*oauth2.Token`) in `templates/sdk/oauth_source.go.tmpl`
- [x] T033 Wire OAuth2 token source into `AuthTransport` for `oauth2` scheme type in `templates/sdk/factory.go.tmpl` — use `oauth2.Transport` wrapping the retry transport
- [x] T034 Integrate token storage/retrieval with keychain in generated resolver: store fetched token under `account=<SCHEME>_TOKEN`; proactively refresh 60s before `ExpiresAt`

**Checkpoint**: Token fetched on first request; reused on second; refreshed when `ExpiresAt - 60s` passes.

---

## Phase 10: User Story 8 — Before/After Request Hooks (P3)

**Goal**: Shell and go-plugin hooks execute around every request when configured in `features.hooks`.

**Independent Test**: Register a shell hook that appends to a log file. Run any operation. Confirm log entry written.

- [x] T035 Complete `internal/hooks/` shell hook executor: exec configured command, write `HookContext` JSON to stdin, treat non-zero exit as abort
- [x] T036 [P] Implement go-plugin hook executor in `internal/hooks/plugin_runner.go` using `hashicorp/go-plugin` gRPC transport — load plugin binary from configured path
- [x] T037 [P] Add `HookContext` struct and JSON tags to `pkg/registry/types.go` (fields: OperationID, Method, URL, Headers, Body, Timestamp, StatusCode, ResponseHeaders, ResponseBody, ElapsedMs, Error)
- [x] T038 Wire hook execution in generated `templates/sdk/factory.go.tmpl`: call `BeforeRequest` hooks before `RoundTrip`; call `AfterResponse` hooks after response received
- [x] T039 Wire `FeaturesConfig.Hooks.Enabled` Viper toggle in generated `templates/cli/main.go.tmpl` — skip hook runner construction when hooks disabled

**Checkpoint**: Shell hook script receives valid JSON on stdin; non-zero exit aborts request with message from stderr.

---

## Phase 11: User Story 9 — Semantic Version in Generated Binary (P3)

**Goal**: `myapp version` outputs the spec's `info.version`, `info.title`, and a build timestamp.

**Independent Test**: Generate from spec with `info.version: "2.1.0"`. Run `myapp version`. Confirm `2.1.0` in output.

- [ ] T040 Update `templates/infra/Makefile.tmpl` to inject `info.version`, `info.title`, and `BUILD_TIME` via `-ldflags` at `go build`
- [ ] T041 Complete generated `version` command in `templates/cli/version.go.tmpl` to print `AppVersion`, `AppTitle`, and `BuildTime` from ldflags-injected vars

---

## Phase 12: User Story 10 — Distribution Placeholder Removal (P3)

**Goal**: All `OWNER`/`PROJECT` placeholders in generated `Makefile`, `Dockerfile`, and `goreleaser.yml` are replaced with values derived from the spec title or `--project` flag.

**Independent Test**: Generate project with `--project petstore`. Run `grep -r OWNER .`. No matches.

- [ ] T042 Complete `internal/distribution/` placeholder substitution: replace `OWNER` and `PROJECT` tokens in `templates/infra/Makefile.tmpl`, `templates/infra/Dockerfile.tmpl`, and `templates/infra/goreleaser.yml.tmpl`
- [ ] T043 Add `--project` flag to `cliford generate` in `cmd/cliford/main.go`; derive value from slugified `info.title` (lowercase, spaces→hyphens) when flag not provided

---

## Phase 13: User Story 11 — Table Output for List Operations (P3)

**Goal**: GET operations returning arrays display as formatted ASCII tables with headers; `--output json` reverts to raw JSON; `--fields` overrides columns.

**Independent Test**: Run a generated list command. Confirm aligned column headers matching response schema property names appear.

- [ ] T044 Add `Display bool` field to `SchemaMeta` in `pkg/registry/types.go`
- [ ] T045 [P] Parse `x-cliford-display: true` extension on response schema properties in `internal/openapi/extensions.go` and set `SchemaMeta.Display = true`
- [ ] T046 Complete table renderer in `internal/cli/output.go` using `text/tabwriter`: select columns where `Display == true`, fall back to all properties; apply `--fields` comma-list override
- [ ] T047 Add `--output` flag (values: `table`, `json`, `detail`; default `table` for array responses) and `--fields` flag to all generated list commands in `internal/cli/generator.go`
- [ ] T048 Update golden files after output changes: `UPDATE_GOLDEN=1 go test ./tests/unit/... -count=1`
- [ ] T048a [P] [US11] Add unit test for table renderer in `tests/unit/table_renderer_test.go`: x-cliford-display column selection; `--fields` override; empty response shows headers + "no results" — write test first

**Checkpoint**: List command without flags shows table; `--output json` shows raw JSON array.

---

## Phase 14: User Story 12 — Body Flag Name Collision Fix (P3)

**Goal**: Body property flag names never collide with path/query parameter names; colliding body props get `body-` prefix.

**Independent Test**: Generate CLI for endpoint with path param `id` and body property `id`. Confirm `--id` and `--body-id` both registered without panic.

- [ ] T049 Audit and fix `bodyPropFlagName()` in `internal/cli/generator.go` to ensure `existingParamFlags` map is built from all path + query param flag names before body props are processed; verify `body-` prefix applied only on collision

---

## Phase 15: User Story 13 — Global Parameters via FeaturesConfig (P3)

**Goal**: Headers and query params configured under `global_params` are injected into every request.

**Independent Test**: Set `global_params.headers.X-Tenant-ID: acme` in config. Run any operation with `--verbose`. Confirm `X-Tenant-ID: acme` in request headers.

- [ ] T050 Add `GlobalParams` struct to `FeaturesConfig` in `internal/config/` and generated config template in `templates/cli/features_config.go.tmpl`
- [ ] T051 Wire global headers and query params injection into `templates/sdk/factory.go.tmpl`: read from `FeaturesConfig.GlobalParams` at startup; add to every request in a `GlobalParamsTransport` layer (per-operation values take precedence)

---

## Phase 16: User Story 14 — Server URL Override via Config (P3)

**Goal**: `server_url` config key and `<APP>_SERVER_URL` env var override the spec's server URL at runtime.

**Independent Test**: Set `PETSTORE_SERVER_URL=http://localhost:8080`. Run any operation. Confirm request goes to `localhost:8080` in `--verbose` output.

- [ ] T052 Add `server_url` Viper config key and `<APP>_SERVER_URL` env var binding to generated root command in `templates/cli/root.go.tmpl`
- [ ] T053 Pass resolved server URL to SDK client constructor in generated `templates/cli/main.go.tmpl` — env var takes precedence over config file

---

## Phase 17: User Story 15 — Man Pages, llms.txt & Shell Completions (P3)

**Goal**: `myapp generate-docs --format man` produces `.1` man pages; `--format markdown` produces `.md` files; `--format llms-txt` produces `llms.txt`; shell completions for bash, zsh, and fish generate without error.

**Independent Test**: Run `myapp generate-docs --format man`. Confirm `.1` files created. Run `myapp completion bash`. Confirm valid bash completion script on stdout.

- [ ] T054 Complete `internal/docs/` man page generation using `cobra/doc.GenManTree` for `--format man` and `cobra/doc.GenMarkdownTree` for `--format markdown`
- [ ] T054a [P] Add `llms.txt` generation to `internal/docs/`: produce an LLM-optimized flat-text summary of all commands, subcommands, flags, and descriptions — wired into the `generate-docs` subcommand with `--format llms-txt`
- [ ] T054b [P] Generate Cobra shell completion subcommand (`completion bash|zsh|fish`) in `templates/cli/completion_cmd.go.tmpl` using Cobra's built-in `GenBashCompletion`, `GenZshCompletion`, `GenFishCompletion`; include in generated app's quality gate check
- [ ] T055 [P] Add `generate-docs` subcommand with `--format` (man|markdown|llms-txt) and `--output-dir` flags to generated app command tree in `templates/cli/docs_cmd.go.tmpl`

---

## Phase 18: User Story 16 — Operation Descriptions in Help Text (P3)

**Goal**: Full `description` field from OpenAPI spec appears in `--help` output, word-wrapped at 80 characters.

**Independent Test**: Generate CLI from spec with multi-sentence operation `description`. Run `--help`. Confirm full description shown.

- [ ] T056 Update `internal/cli/generator.go` to set `cmd.Long` from `op.Description` (word-wrapped at 80 chars using a simple stdlib-based wrap function — no external dependency)

---

## Phase 19: User Story 17 — Confirm Prompt for Destructive Operations (P3)

**Goal**: DELETE operations and operations with `x-cliford-confirm: true` prompt `"... [y/N]:"` before executing; `--yes` skips prompt.

**Independent Test**: Run DELETE without `--yes`. Confirm prompt. Answer `n`. Confirm no HTTP request made.

- [ ] T057 Add `--yes`/`-y` flag to generated DELETE and `x-cliford-confirm` operations in `internal/cli/generator.go`
- [ ] T058 Generate `confirmAction()` helper in `templates/cli/confirm.go.tmpl`: print configured message + `[y/N]:`, default No on Enter, guard non-TTY (error if no `--yes` and not a TTY)

---

## Phase 20: User Story 18 — TUI Server URL from Config (P4)

**Goal**: Generated TUI reads `server_url` from Viper config at runtime rather than using the URL hardcoded at generation time.

**Independent Test**: Override `server_url` in config. Launch TUI, execute an operation. Confirm request goes to override URL.

- [ ] T059 Update `internal/tui/generator.go` to remove hardcoded `BaseURL` string from `OperationItem` struct literals in generated `explorer.go`; instead read `viper.GetString("server_url")` (with spec URL as default) in generated `main.go` and pass to TUI model constructor

---

## Phase 21: Polish & Cross-Cutting Concerns

**Purpose**: Golden file updates, integration test, and any remaining cross-cutting validations. Unit tests for key subsystems have been moved inline with their respective story phases (T014a, T017b, T019a, T048a) to enforce test-first development per Constitution §Dev Workflow.

- [ ] T060 [P] Final golden file update across all generator changes: `UPDATE_GOLDEN=1 go test ./tests/unit/... -count=1` from `/Users/samstevens/asot/cliford`
- [ ] T061 Add integration test: generate from petstore spec → `go build ./...` → run `list-pets --help` → assert output contains description; run `list-pets` → assert table output; run `completion bash` → assert valid script; run `generate-docs --format llms-txt` → assert `llms.txt` created in `tests/integration/petstore_test.go`
- [ ] T062 [P] Verify all generated quality gates pass: `go build ./...`, `go vet ./...`, `go test ./...`, `--help` responds, shell completions generate, docs generate — run against reference petstore spec output

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately; T001/T002/T003 all parallel
- **Phase 2 (Foundational)**: Depends on Phase 1 completion — **BLOCKS all user story phases**
- **Phases 3–5 (US1–US3, P1)**: All depend on Phase 2; can run in parallel with each other (different files)
- **Phases 6–9 (US4–US7, P2)**: Depend on Phase 2; US6/US7 also need Phase 3 (auth) complete for full integration
- **Phases 10–20 (US8–US18, P3/P4)**: Depend on Phase 2; most are independent of each other
- **Phase 21 (Polish)**: Depends on all desired stories complete

### User Story Dependencies

| Story | Priority | Depends On |
|-------|----------|-----------|
| US1 — Auth | P1 | Phase 2 (SDK-First) |
| US2 — Retry | P1 | Phase 2 (SDK-First) |
| US3 — Timeout | P1 | Phase 2 (SDK-First) |
| US4 — Verbose | P2 | Phase 2 |
| US5 — Prompts | P2 | Phase 2 |
| US6 — Pagination | P2 | Phase 2 |
| US7 — OAuth2 | P2 | US1 (credential chain) |
| US8 — Hooks | P3 | Phase 2 |
| US9 — Version | P3 | Phase 2 |
| US10 — Distribution | P3 | Phase 2 |
| US11 — Table Output | P3 | Phase 2 |
| US12 — Flag Collision | P3 | Phase 2 |
| US13 — Global Params | P3 | Phase 2 |
| US14 — Server URL | P3 | Phase 2 |
| US15 — Man Pages | P3 | Phase 2 |
| US16 — Descriptions | P3 | Phase 2 |
| US17 — Confirm Prompt | P3 | Phase 2 |
| US18 — TUI Server URL | P4 | Phase 2 |

### Parallel Opportunities Within Phases

- **Phase 1**: T001, T002, T003 all parallel
- **Phase 2**: T004→T005→T006 sequential (each builds on prior); T007 parallel with T004
- **Phase 3**: T014a (test) first and parallel with T010, T011; T009 after T010/T011; T012, T013 sequential after T009
- **Phase 4**: T017b (test) first; T015 after test written; T016, T017 parallel; T017a after T015
- **Phase 5**: T019a (test) parallel with T018; T019 parallel with T018
- **Phase 9**: T031 sequential; T032, T033 parallel; T034 after T031/T032
- **Phase 10**: T035, T036, T037 parallel; T038 after all three; T039 after T038
- **Phase 13**: T048a (test) parallel with T044, T045; T046 after T044/T045; T047 after T046
- **Phase 17**: T054, T054a, T054b, T055 all parallel
- **Phase 21**: T060, T061, T062 all parallel

---

## Parallel Execution Example: Phase 3 (US1 — Auth)

```
# Run in parallel (no file overlap):
Task T010: "Implement AES-256-GCM encrypted-file fallback store in templates/cli/auth_store.go.tmpl"
Task T011: "Generate <APP>_<SCHEME>_<TYPE> env var names from registry metadata in internal/cli/auth.go"

# Then sequentially:
Task T009: "Complete CredentialResolver 5-tier chain in internal/cli/auth.go"
Task T012: "Generate auth error messages in internal/cli/auth.go"
Task T013: "Wire CredentialResolver into AuthTransport in templates/sdk/factory.go.tmpl"
Task T014: "Update golden files"
```

---

## Implementation Strategy

### MVP First (P1 Stories Only — US1 + US2 + US3)

1. Complete Phase 1: Setup (add deps)
2. Complete Phase 2: Foundational SDK-First refactor (**critical gate**)
3. Complete Phase 3: US1 — Auth credentials applied to requests
4. Complete Phase 4: US2 — Retry on transient failures
5. Complete Phase 5: US3 — Per-request timeout enforcement
6. **STOP and VALIDATE**: Generated petstore CLI authenticates, retries, and times out correctly
7. Demo: `petstore list-pets` works against real API with env var credentials

### Incremental Delivery

1. **MVP** (Phases 1–5): Auth + Retry + Timeout — generated app connects to real APIs reliably
2. **+UX** (Phases 6–8): Verbose + Prompts + Pagination — generated app is pleasant to use
3. **+OAuth** (Phase 9): Client credentials flow — covers machine-to-machine APIs
4. **+Hooks** (Phase 10): Extensibility — power users can inject custom logic
5. **+Polish** (Phases 11–20): Table output, docs, version, distribution — production-ready artifacts
6. **+Tests** (Phase 21): Comprehensive test coverage

### Notes

- Run `go build ./...` and `go vet ./...` after every phase — generated code must compile immediately
- Update golden files (`UPDATE_GOLDEN=1`) after any generator change, not just at end
- T004 (SDK-First refactor) is the highest-risk task — validate with `go build` before proceeding
- US12 (T049, flag collision fix) is a small standalone bug fix — can be done any time after Phase 2
- The `templates/` directory contains Go text templates; changes there do not affect `go build` of Cliford itself but affect generated app output
- Test-first tasks (T014a, T017b, T019a, T048a) MUST be written and fail BEFORE their corresponding implementation tasks, per Constitution §Dev Workflow
- FR-027→FR-032, FR-028→FR-033, FR-029→FR-034 (renumbered after adding FR-030/FR-031 for llms.txt and shell completions)
