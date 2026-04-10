# Tasks: OpenAPI CLI & TUI Code Generation

**Input**: Design documents from `/specs/001-openapi-cli-codegen/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are not explicitly requested in the spec. Test tasks are omitted. The constitution mandates test-first development for Cliford itself, so integration and golden file tests are included in the Polish phase as cross-cutting concerns.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Single Go module at repository root
- Source: `cmd/`, `internal/`, `pkg/`
- Templates: `templates/`
- Test fixtures: `testdata/`

---

## Phase 1: Setup

**Purpose**: Project initialization and Go module structure

- [x] T001 Initialize Go module with `go mod init github.com/cliford/cliford` and create base directory structure per plan.md (`cmd/cliford/`, `internal/`, `pkg/`, `templates/`, `testdata/`, `tests/`)
- [x] T002 [P] Create reference Petstore OpenAPI spec for testing at `testdata/specs/petstore.yaml` with at least 5 operations across 2 tags, path/query/header/body params, and bearer token security
- [x] T003 [P] Create reference multi-auth OpenAPI spec at `testdata/specs/complex-auth.yaml` with API key, basic, bearer, and OAuth2 security schemes
- [x] T004 [P] Create reference paginated OpenAPI spec at `testdata/specs/paginated.yaml` with offset, cursor, and link-header paginated endpoints

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

- [x] T005 Define public Operation Registry types (OperationMeta, ParamMeta, RequestBodyMeta, ResponseMeta, SecurityRequirement) in `pkg/registry/types.go`
- [x] T006 Define public Theme config types (ThemeConfig, ColorConfig) in `pkg/theme/types.go`
- [x] T007 Implement OpenAPI spec parser supporting 3.0 and 3.1 in `internal/openapi/parser.go` using `github.com/getkin/kin-openapi` — parse operations, parameters, schemas, servers, security schemes
- [x] T008 Implement Operation Registry builder in `internal/openapi/registry.go` — map each parsed operation to OperationMeta with tag-based grouping and stutter removal logic
- [x] T009 [P] Implement Go template engine in `internal/codegen/engine.go` — load templates from `templates/` directory, execute with OperationMeta data, write output files with `gofmt` formatting
- [x] T010 [P] Implement pipeline validation in `internal/pipeline/validate.go` — validate OpenAPI spec is parseable, validate config schema, check for operation ID collisions after stutter removal
- [x] T011 Implement pipeline orchestrator in `internal/pipeline/pipeline.go` — stage execution (parse -> SDK -> CLI -> TUI -> infra), stage status tracking, error propagation, hook integration points (stubs for now)
- [x] T012 Implement Cliford CLI entrypoint in `cmd/cliford/main.go` with Cobra root command, `--version`, `--help`, `--quiet`, `--no-color` global flags

**Checkpoint**: Foundation ready — user story implementation can now begin

---

## Phase 3: User Story 1 — Generate a Working CLI from an OpenAPI Spec (Priority: P1)

**Goal**: Developer provides an OpenAPI spec, runs `cliford generate`, gets a compilable Go CLI binary with one command per operation, proper flags, help, and output formatting.

**Independent Test**: Run `cliford generate` against `testdata/specs/petstore.yaml`. Verify `go build ./...` succeeds, `./petstore --help` works, every operation is present as a command with correct flags.

### Implementation for User Story 1

- [x] T013 [US1] Implement oapi-codegen SDK generation in `internal/sdk/generator.go` — invoke oapi-codegen v2 as Go library to produce types + client code, write to `internal/sdk/sdk.gen.go` in generated app output
- [x] T014 [US1] Create SDK output templates: client wrapper in `templates/sdk/client.go.tmpl` and enhanced error types in `templates/sdk/errors.go.tmpl` (APIError with StatusCode, Body, RequestID, Headers, Operation fields)
- [x] T015 [P] [US1] Create main.go infra template in `templates/infra/main.go.tmpl` — app entrypoint with Cobra root command, version injection via ldflags, Viper config init
- [x] T016 [P] [US1] Create go.mod infra template in `templates/infra/go.mod.tmpl` — Go module with Cobra, Viper, and SDK dependencies
- [x] T017 [US1] Implement Cobra command tree generation in `internal/cli/generator.go` — create one command per OperationMeta, group by tags, wire to SDK method calls, apply stutter removal from registry
- [x] T018 [US1] Implement flag generation in `internal/cli/flags.go` — map each operation parameter to a typed Cobra flag (string, int, bool, string-slice), set required/optional, add help text from spec description, handle path/query/header/cookie placement
- [x] T019 [US1] Implement input precedence logic in `internal/cli/flags.go` — merge individual flags (highest) > `--body` JSON payload > stdin JSON (lowest), handle bytes fields with `file:`, `b64:`, and raw string prefixes
- [x] T020 [US1] Implement output formatter generation in `internal/cli/output.go` — generate `--output-format` flag with pretty (default for TTY), json, yaml, table modes; generate `--jq` expression support; generate `--output-file` for binary responses
- [x] T021 [US1] Create Cobra command templates: root command in `templates/cli/root.go.tmpl`, tag group in `templates/cli/group.go.tmpl`, operation command in `templates/cli/operation.go.tmpl`
- [x] T022 [US1] Create output formatter template in `templates/cli/output.go.tmpl` — pretty printer, JSON marshaler, YAML marshaler, table renderer with column detection from response schema
- [x] T023 [US1] Implement `cliford generate` command in `cmd/cliford/main.go` — parse `--spec`, `--config`, `--output-dir`, `--dry-run`, `--force`, `--verbose` flags; invoke pipeline orchestrator; report files created/modified
- [x] T024 [US1] Implement shell completion generation in `internal/cli/completions.go` — generate `completion` subcommand template for bash, zsh, fish, PowerShell via Cobra's built-in completion support
- [x] T079 [US1] Implement generated app global flags on root command in `templates/cli/root.go.tmpl` — generate all persistent flags: `--output-format`, `--server`, `--timeout`, `--debug`, `--dry-run`, `--no-interactive`, `--tui`, `-y`/`--yes`, `--no-retries`, `--jq`, `--output-file`, `--output-b64`, `--agent`; `--dry-run` captures HTTP request (method, URL, headers, redacted body) and prints to stdout without executing; `--debug` logs full request/response to stderr with secret redaction; `-y`/`--yes` skips all confirmation prompts, errors on missing required params instead of prompting, uses defaults for optional interactive inputs; `--agent` forces structured JSON output and suppresses interactive surfaces

**Checkpoint**: `cliford generate` produces a compilable Go CLI app from any valid OpenAPI spec. All operations are callable with correct flags and output formatting.

---

## Phase 4: User Story 2 — Configure and Customize Generation (Priority: P2)

**Goal**: Developer creates a `cliford.yaml` to customize binary name, env prefix, command aliases, hidden operations, per-operation overrides, and output defaults. OpenAPI `x-cliford-*` extensions work as secondary config.

**Independent Test**: Create `cliford.yaml` with custom name, aliases on 2 operations, one hidden operation, and default output format. Regenerate. Verify all customizations reflected in generated binary.

### Implementation for User Story 2

- [x] T025 [US2] Implement cliford.yaml config parsing in `internal/config/cliford_config.go` — Viper-based loading with all ClifordConfig fields from data-model.md, environment variable binding with `CLIFORD_` prefix, defaults for all fields
- [x] T026 [US2] Implement `x-cliford-*` OpenAPI extension extraction in `internal/openapi/extensions.go` — parse `x-cliford-cli` (aliases, hidden, group, confirm), `x-cliford-tui` (displayAs, refreshable), `x-cliford-pagination`, `x-cliford-retries` from each operation
- [x] T027 [US2] Implement config merge logic in `internal/config/merge.go` — resolution order: cliford.yaml operation-level > x-cliford-* extensions > cliford.yaml globals > built-in defaults; merge into OperationMeta fields in registry
- [x] T028 [US2] Update `internal/cli/generator.go` to apply config overrides — command aliases from `CLIAliases`, hidden commands from `CLIHidden`, custom group names from `CLIGroup`, custom binary name and env prefix
- [x] T029 [US2] Implement `cliford init` command in `cmd/cliford/main.go` — accept `--spec`, `--name`, `--package`, `--mode` flags; derive defaults from spec (title -> name, etc.); write `cliford.yaml`
- [x] T030 [US2] Implement `cliford validate` command in `cmd/cliford/main.go` — parse spec + config, run validation, report errors/warnings
- [x] T031 [P] [US2] Implement JSON Schema generation for cliford.yaml in `internal/config/schema.go` — generate schema from ClifordConfig struct for IDE autocompletion and validation
- [x] T032 [P] [US2] Create test config fixture at `testdata/fixtures/cliford-customized.yaml` — with custom name, aliases, hidden ops, theme, per-operation overrides for testing merge logic

**Checkpoint**: Developers can fully customize generation via `cliford.yaml` and `x-cliford-*` extensions. Config merge precedence works correctly.

---

## Phase 5: User Story 3 — Authenticate with the Target API (Priority: P3)

**Goal**: Generated app supports auth login/logout/status commands, stores credentials in OS keychain with encrypted fallback, attaches auth to requests automatically, supports multiple profiles with server URL switching.

**Independent Test**: Generate from `testdata/specs/complex-auth.yaml`. Run `auth login` with bearer token. Verify stored in keychain. Run authenticated command. Verify token sent. Switch profile. Run `auth status`.

### Implementation for User Story 3

- [x] T033 [US3] Implement auth middleware generation in `internal/sdk/auth_enhancer.go` — generate HTTP transport middleware that reads credentials from storage and attaches to requests based on security scheme (API key header/query, Basic auth header, Bearer token header, OAuth2 access token)
- [x] T034 [US3] Create keychain integration template in `templates/cli/keychain.go.tmpl` — cross-platform credential storage using go-keyring, encrypted file fallback (AES-256-GCM) when keychain unavailable, warning when using fallback
- [x] T035 [US3] Create profile management template in `templates/cli/profiles.go.tmpl` — load/save profiles from `~/.config/<app>/config.yaml`, switch active profile, each profile has server URL + auth method + credential reference
- [x] T036 [US3] Implement auth command generation in `internal/cli/auth.go` — generate `auth login` (interactive credential collection), `auth logout` (clear stored credentials), `auth status` (display redacted auth state), `auth switch` (switch profiles), `auth refresh` (force OAuth token refresh)
- [x] T037 [US3] Implement config command generation in `internal/cli/config_cmd.go` — generate `config show`, `config set`, `config get`, `config reset`, `config edit` (open in $EDITOR), `config use-profile`, `config path`, `config validate`
- [x] T038 [US3] Create Viper config template for generated apps in `templates/cli/config.go.tmpl` — Viper init with XDG config path, env var binding with configurable prefix, flag binding, config file format (YAML)
- [x] T039 [US3] Implement credential redaction in `templates/sdk/client.go.tmpl` — redact Authorization headers, API keys, and any field matching configurable patterns in `--debug` output and error messages
- [x] T040 [US3] Implement server selection in `internal/cli/flags.go` — generate `--server` global flag from spec's `servers` list, support selection by URL or description alias, generate server variable flags for templated URLs
- [x] T080 [US3] Implement OAuth 2.0 flow generation in `internal/cli/auth.go` and `templates/cli/oauth.go.tmpl` — generate Authorization Code flow (browser redirect + local callback server), Device Code flow (polling), Client Credentials flow; generate token storage with expiry tracking, proactive refresh before expiry; use `golang.org/x/oauth2` in generated app dependencies

**Checkpoint**: Generated apps authenticate securely, store credentials in OS keychain, support multi-profile, redact secrets in all output, and handle OAuth token lifecycle.

---

## Phase 6: User Story 4 — Use the App in TUI Mode (Priority: P4)

**Goal**: Generated app offers full-screen TUI with operation explorer, parameter forms, styled response views, and configurable themes. Hybrid mode provides inline prompts. App auto-detects TTY/agent environments.

**Independent Test**: Generate with TUI enabled. Run `--tui`. Navigate explorer, select operation, fill form, execute, view response. Verify theme colors applied. Run with piped output and verify CLI fallback.

### Implementation for User Story 4

- [x] T041 [US4] Implement Bubbletea app generation in `internal/tui/generator.go` — generate main TUI program with navigation between explorer, operation, and response views; wire to SDK methods
- [x] T042 [US4] Implement explorer view generation in `internal/tui/explorer.go` — generate List bubble showing all operations grouped by tag, filterable search, enter to select operation
- [x] T043 [US4] Implement operation form generation in `internal/tui/operation.go` — generate TextInput for string params, selection for enums, TextArea for body input, FilePicker for file params; form submission triggers SDK call
- [x] T044 [US4] Implement response viewport generation in `internal/tui/response.go` — generate Table view for array responses, Viewport with scrolling for single-object/large responses, Spinner during API call
- [x] T045 [US4] Implement Lipgloss theme engine generation in `internal/tui/theme.go` — generate Theme struct from ThemeConfig, map semantic colors to Lipgloss styles, border style selection, component-specific styles
- [x] T046 [US4] Implement shared TUI component generation in `internal/tui/components.go` — confirmation dialog, status bar (active profile + server URL), help bar (keybindings), error notification bar
- [x] T047 [US4] Create TUI templates: app in `templates/tui/app.go.tmpl`, explorer in `templates/tui/explorer.go.tmpl`, operation form in `templates/tui/operation.go.tmpl`, response in `templates/tui/response.go.tmpl`, theme in `templates/tui/theme.go.tmpl`
- [x] T048 [US4] Implement mode detection in `internal/hybrid/mode.go` — TTY check, agent environment detection (Claude Code, Cursor, Codex, Aider, Windsurf, etc.), flag precedence (`--tui` > `--no-interactive` > `-y` > env > config > auto)
- [x] T049 [US4] Implement inline Bubbletea prompts for hybrid mode in `internal/hybrid/prompt.go` — small focused Bubbletea programs for missing required params (text input, selection), return values to calling Cobra command
- [x] T050 [US4] Implement adapter wiring in `internal/hybrid/adapter.go` — connect mode detection to command execution; route to TUI adapter (full Bubbletea), CLI adapter (Cobra), or headless adapter (no prompts, JSON output) based on resolved mode

**Checkpoint**: Generated apps support full TUI, hybrid inline prompts, and headless modes with auto-detection and configurable themes.

---

## Phase 7: User Story 5 — Extend Generated Code Without Losing Changes (Priority: P5)

**Goal**: Generated code includes custom code regions. Developer code in these regions survives regeneration. `cliford diff` previews changes. Backups created before overwriting.

**Independent Test**: Generate app. Add code in extension regions. Regenerate. Verify custom code preserved. Run `cliford diff` before regenerating and verify diff is accurate.

### Implementation for User Story 5

- [x] T051 [US5] Implement custom code region preservation in `internal/codegen/regions.go` — extract regions from existing files before regeneration, reinject into new output, detect region name mismatches (operation removed), emit warnings for orphaned regions
- [x] T052 [US5] Implement pre-generation backup in `internal/codegen/backup.go` — copy current generated output to `.cliford/backup/<timestamp>/` before overwriting, limit to last 5 backups, skip if `--force` flag
- [x] T053 [US5] Implement generation diff preview in `internal/codegen/diff.go` — generate to temp directory, unified diff against current output, highlight custom code regions as safe, color-coded output
- [x] T054 [US5] Implement `cliford diff` command in `cmd/cliford/main.go` — invoke diff logic from T053, output to stdout, exit code 0 if no changes, 1 if changes exist
- [x] T055 [US5] Implement lockfile management in `internal/pipeline/lockfile.go` — write `cliford.lock` after generation with: spec hash, config hash, cliford version, timestamp, list of generated files with checksums
- [x] T056 [US5] Update all templates (SDK, CLI, TUI, infra) to include `--- CUSTOM CODE START: <name> ---` / `--- CUSTOM CODE END: <name> ---` markers at strategic extension points (imports, pre/post operation, error handling, root init, config init) when `features.customCodeRegions` is enabled in config

**Checkpoint**: Custom code survives regeneration. Diff preview works. Backups protect against data loss.

---

## Phase 8: User Story 6 — Handle Pagination, Retries, and Errors Gracefully (Priority: P6)

**Goal**: Generated SDK includes pagination helpers and retry logic. CLI surfaces `--all`, `--max-pages`, `--no-retries` flags. Errors display clearly with status codes, messages, and request IDs.

**Independent Test**: Generate from `testdata/specs/paginated.yaml`. Call paginated endpoint with `--all`. Verify all pages fetched. Simulate 503 and verify retry. Trigger 422 and verify structured error output.

### Implementation for User Story 6

- [x] T057 [US6] Implement pagination helper generation in `internal/sdk/pagination_enhancer.go` — generate `ListAll` (fetch all pages into slice) and `ListIter` (memory-efficient iterator) methods for paginated operations; support offset, page, cursor, link-header, and URL-based patterns based on PaginationConfig
- [x] T058 [US6] Create pagination templates in `templates/sdk/pagination.go.tmpl` — iterator type with `Next()` method, termination detection (empty array, null cursor, no Link header), configurable page size default
- [x] T059 [US6] Implement retry wrapper generation in `internal/sdk/retry_enhancer.go` — generate HTTP transport middleware with exponential backoff + jitter, configurable via RetryConfig, respect Retry-After and X-RateLimit-Reset headers, preserve idempotency keys across retries
- [x] T060 [US6] Create retry templates in `templates/sdk/retry.go.tmpl` — retry middleware with configurable initial interval, max interval, max elapsed time, exponent, jitter, status code matching (including `5XX` wildcard), connection error retry
- [x] T061 [US6] Implement enhanced error type generation in `internal/sdk/errors.go` — generate typed error hierarchy: APIError (base), StructuredError (JSON body), ValidationError (field-level), RateLimitError (with retry info), NetworkError (connection/DNS)
- [x] T062 [US6] Update `internal/cli/flags.go` to generate pagination flags (`--all`, `--max-pages`, `--page`, `--limit`, `--cursor`) on paginated commands and retry flags (`--no-retries`, `--retry-max-attempts`, `--retry-max-elapsed`) on all commands
- [x] T063 [US6] Update `internal/cli/output.go` to generate error formatting — pretty mode with status code + field errors + request ID, JSON mode with structured error object, debug mode with full redacted request/response trace

**Checkpoint**: Generated apps paginate, retry, and display errors gracefully with zero user-side implementation.

---

## Phase 9: User Story 7 — Distribute the Generated App (Priority: P7)

**Goal**: Generated app includes GoReleaser config, install scripts, Homebrew formula, version injection via ldflags, and documentation generation.

**Independent Test**: Generate with release config enabled. Run `goreleaser check`. Verify install scripts are valid. Verify `--version` output format. Generate docs.

### Implementation for User Story 7

- [x] T064 [US7] Implement GoReleaser config generation in `internal/distribution/goreleaser.go` — generate `.goreleaser.yaml` with cross-platform builds (darwin/linux/windows, amd64/arm64), ldflags for version/commit/date injection, checksum file
- [x] T065 [P] [US7] Implement install script generation in `internal/distribution/install.go` — generate `install.sh` (Unix: detect OS/arch, download binary, verify checksum, install to PATH) and `install.ps1` (Windows equivalent)
- [x] T066 [P] [US7] Implement Homebrew formula generation in `internal/distribution/homebrew.go` — generate formula template for tap publishing, configurable via `distribution.homebrew.tap` config field
- [x] T067 [US7] Create distribution templates: GoReleaser in `templates/infra/goreleaser.yaml.tmpl`, GitHub Actions release workflow in `templates/infra/release.yaml.tmpl`, install scripts in `templates/infra/install.sh.tmpl` and `templates/infra/install.ps1.tmpl`
- [x] T068 [US7] Implement Markdown doc generation in `internal/docs/markdown.go` — generate one Markdown file per command using Cobra's `doc.GenMarkdownTree`, `DisableAutoGenTag = true`, inject front matter for static sites
- [x] T069 [US7] Implement llms.txt generation in `internal/docs/llms.go` — generate flat LLM-optimized documentation with all commands, flags, examples, auth requirements; structured with H2/H3 headings for vector index chunking
- [x] T070 [US7] Implement `cliford version bump` command in `cmd/cliford/main.go` — accept `auto|patch|minor|major` argument; for `auto`, diff current spec against lockfile's spec hash to determine bump type (ops added -> minor, removed -> major, metadata -> patch); update version in `cliford.yaml`

**Checkpoint**: Generated apps ship with complete distribution tooling, version management, and auto-generated documentation.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Hook system, developer tooling, testing infrastructure, and final integration

- [x] T071 Implement hook registration in `internal/hooks/registry.go` — parse hook definitions from cliford.yaml (`hooks` section), register shell command hooks with execution context
- [x] T072 Implement lifecycle hook execution in `internal/hooks/lifecycle.go` — run `before:*` and `after:*` hooks at each pipeline stage, pass stage context (spec path, output dir, stage name), capture stdout/stderr, fail pipeline on non-zero exit
- [x] T073 Implement transform hook execution in `internal/hooks/transform.go` — run `transform:operation`, `transform:command`, `transform:model`, `transform:style` hooks, pass current metadata as JSON on stdin, read modified metadata from stdout
- [x] T074 Implement `cliford doctor` command in `cmd/cliford/main.go` — check Go version >= 1.22, check oapi-codegen importability, validate config file if present, validate spec if present, report pass/fail per check
- [x] T075 [P] Create end-to-end integration test in `tests/integration/generate_test.go` — generate from `testdata/specs/petstore.yaml`, run `go build ./...`, run `go vet ./...`, verify binary responds to `--help`, verify all operations present
- [x] T076 [P] Create custom code region integration test in `tests/integration/regions_test.go` — generate, inject custom code in regions, regenerate with modified spec (add operation), verify custom code preserved and new operation added
- [x] T077 [P] Create golden file test infrastructure in `tests/unit/golden_test.go` — compare generated output against `testdata/golden/` files, update flag for refreshing golden files, fail on unexpected diffs
- [x] T078 Update pipeline orchestrator `internal/pipeline/pipeline.go` to wire all stages together — connect SDK generation (T013), CLI generation (T017), TUI generation (T041), infra generation (T064), hook execution (T072), custom code regions (T051), and lockfile (T055) into the full pipeline

**Checkpoint**: All user stories integrated, hooks operational, tests green, tool ready for dogfooding.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3–9)**: All depend on Foundational phase completion
  - US1 (Phase 3) has no dependencies on other stories
  - US2 (Phase 4) depends on US1 (needs CLI generation to apply config to)
  - US3 (Phase 5) depends on US1 (needs SDK + CLI to add auth to)
  - US4 (Phase 6) depends on US1 (needs Operation Registry + SDK)
  - US5 (Phase 7) depends on US1 (needs generated files to preserve regions in)
  - US6 (Phase 8) depends on US1 (needs SDK to add pagination/retry to)
  - US7 (Phase 9) depends on US1 (needs generated app to distribute)
- **Polish (Phase 10)**: Depends on all user stories being complete

### Within Each User Story

- SDK/model tasks before CLI/TUI tasks
- Templates before generators that use them
- Core logic before commands that invoke it
- Story complete before moving to next priority

### Parallel Opportunities

All user stories US2–US7 can begin in parallel after US1 completes, as they modify different packages:
- US2: `internal/config/`, `internal/openapi/extensions.go`
- US3: `internal/cli/auth.go`, `templates/cli/keychain.go.tmpl`
- US4: `internal/tui/`, `internal/hybrid/`, `templates/tui/`
- US5: `internal/codegen/`, `internal/pipeline/lockfile.go`
- US6: `internal/sdk/pagination_enhancer.go`, `internal/sdk/retry_enhancer.go`, `templates/sdk/`
- US7: `internal/distribution/`, `internal/docs/`, `templates/infra/`

---

## Parallel Example: User Story 1

```bash
# These can run in parallel (different files):
Task T015: "Create main.go infra template in templates/infra/main.go.tmpl"
Task T016: "Create go.mod infra template in templates/infra/go.mod.tmpl"

# These can run in parallel after T013:
Task T017: "Implement Cobra command tree generation"
Task T018: "Implement flag generation"
Task T020: "Implement output formatter generation"
Task T024: "Implement shell completion generation"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Generate from petstore.yaml, `go build`, `--help`, call operations
5. Ship as v0.1.0 — basic CLI generation works

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 -> Test independently -> **v0.1.0** (basic CLI generation)
3. Add US2 -> Test independently -> **v0.2.0** (configurable generation)
4. Add US3 -> Test independently -> **v0.3.0** (authentication)
5. Add US4 -> Test independently -> **v0.4.0** (TUI mode)
6. Add US5 -> Test independently -> **v0.5.0** (custom code regions)
7. Add US6 -> Test independently -> **v0.6.0** (pagination/retries/errors)
8. Add US7 -> Test independently -> **v0.7.0** (distribution)
9. Polish -> **v1.0.0** (hooks, tests, doctor, full integration)

### Parallel Team Strategy

With multiple developers after US1 is complete:

- Developer A: US2 (Configuration) + US5 (Custom Code Regions)
- Developer B: US3 (Auth) + US6 (Pagination/Retries)
- Developer C: US4 (TUI) — largest story, needs dedicated focus
- Developer D: US7 (Distribution) + Polish

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable after US1
- US1 is the critical path — all other stories depend on it
- Templates (`.tmpl` files) and generators (`.go` files) are separate tasks because templates define the output shape while generators orchestrate the pipeline
- The `internal/sdk/` enhancer files are split by concern: `auth_enhancer.go` (US3), `pagination_enhancer.go` (US6), `retry_enhancer.go` (US6) to enable parallel development
