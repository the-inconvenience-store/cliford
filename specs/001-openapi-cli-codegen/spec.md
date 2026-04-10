# Feature Specification: OpenAPI CLI & TUI Code Generation

**Feature Branch**: `001-openapi-cli-codegen`
**Created**: 2026-04-10
**Status**: Draft
**Input**: User description: "Cliford - an OSS CLI tool and library that generates Go-powered CLI and TUI applications from OpenAPI specifications"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Generate a Working CLI from an OpenAPI Spec (Priority: P1)

A developer has an existing OpenAPI specification for their API. They want
to quickly produce a fully functional command-line application that wraps
every API operation as a CLI command, with proper flags, help text, and
output formatting, without writing any code by hand.

**Why this priority**: This is the core value proposition of Cliford. Without
this capability nothing else matters. A developer MUST be able to go from
spec to working binary in a single command.

**Independent Test**: Provide a sample OpenAPI spec with at least 5
operations across 2 tags. Run `cliford generate`. Verify the output compiles,
the binary responds to `--help`, and every operation is callable with the
correct flags.

**Acceptance Scenarios**:

1. **Given** a valid OpenAPI 3.0 spec with tagged operations, **When** the
   developer runs the generation command, **Then** a complete project is
   produced that compiles without errors and exposes one CLI command per
   API operation grouped by tag.
2. **Given** an operation with path, query, header, and body parameters,
   **When** the developer inspects the generated command, **Then** each
   parameter appears as a correctly typed CLI flag with help text drawn
   from the spec description.
3. **Given** a generated binary, **When** the end user executes a command
   with valid flags, **Then** the command makes the correct HTTP request
   and renders the response in the chosen output format (pretty, JSON,
   YAML, or table).
4. **Given** a generated binary, **When** the end user runs any command
   with `--help`, **Then** usage information, flag descriptions, and
   examples are displayed.

---

### User Story 2 - Configure and Customize Generation (Priority: P2)

A developer wants to control how the generated application looks, behaves,
and is named. They need to set the binary name, environment variable prefix,
command aliases, output defaults, and per-operation overrides without editing
generated source code.

**Why this priority**: Every real-world API has naming conventions, branding,
and ergonomic preferences that differ from auto-generated defaults.
Customisation is required for any production use.

**Independent Test**: Create a configuration file that sets the app name,
adds aliases to two operations, hides one operation, and changes the default
output format. Regenerate and verify all customizations are reflected.

**Acceptance Scenarios**:

1. **Given** a configuration file setting `cliName: petstore` and
   `envVarPrefix: PET`, **When** the developer regenerates, **Then** the
   binary is named `petstore` and all environment variables are prefixed
   with `PET_`.
2. **Given** per-operation config adding aliases `ls` and `list` to a
   `listPets` operation, **When** the end user runs `petstore pets ls`,
   **Then** the command executes the same operation as `petstore pets
   listPets`.
3. **Given** a configuration marking an operation as hidden, **When** the
   end user runs `--help`, **Then** the hidden operation does not appear
   in help output but is still callable directly.
4. **Given** inline OpenAPI extension annotations on an operation, **When**
   the developer also has a standalone config for the same operation,
   **Then** the standalone config takes precedence over the inline
   annotation.

---

### User Story 3 - Authenticate with the Target API (Priority: P3)

An end user of the generated app needs to authenticate against the API
using their credentials. They expect a secure, ergonomic flow that
remembers their credentials across sessions and supports multiple
environments (production, staging, local).

**Why this priority**: Nearly all real APIs require authentication. Without
built-in auth, the generated app is unusable for protected endpoints.

**Independent Test**: Generate an app from a spec with bearer token security.
Run the login command, provide a token, verify it is stored securely. Run an
authenticated operation and verify the token is sent. Run logout and verify
the token is removed.

**Acceptance Scenarios**:

1. **Given** an OpenAPI spec declaring bearer token authentication, **When**
   the developer generates and the end user runs the login command, **Then**
   the user is prompted for their token and it is stored in the operating
   system's secure credential store.
2. **Given** stored credentials, **When** the end user runs an authenticated
   command, **Then** the credentials are automatically attached to the
   request without manual flag passing.
3. **Given** multiple configured profiles (production, staging), **When** the
   end user switches profiles, **Then** subsequent commands use the
   credentials and server URL for the selected profile.
4. **Given** an operation marked with `security: []` in the spec, **When**
   the end user calls it without credentials, **Then** the command succeeds
   without prompting for authentication.
5. **Given** credentials stored in the secure store, **When** the end user
   runs the status command, **Then** a summary of the current auth state is
   displayed with all secret values redacted.

---

### User Story 4 - Use the App in TUI Mode (Priority: P4)

A developer wants to generate an application that offers an interactive
terminal user interface alongside or instead of the traditional CLI. End
users should be able to browse available operations, fill in parameters via
forms, and view responses in styled views. Developers should be able to
theme the TUI to match their brand.

**Why this priority**: Interactive TUI significantly improves discoverability
and onboarding for complex APIs. It also differentiates Cliford from every
existing code generator.

**Independent Test**: Generate an app with TUI enabled. Launch in TUI mode.
Navigate the operation explorer, select an operation, fill the form, execute,
and view the response without ever typing a raw command.

**Acceptance Scenarios**:

1. **Given** a generated app with TUI enabled, **When** the end user
   launches the app with the TUI flag, **Then** a full-screen interactive
   interface appears showing an operation explorer.
2. **Given** the TUI explorer, **When** the end user selects an operation,
   **Then** a parameter form is displayed with appropriate input controls
   for each parameter type (text fields, selections, file pickers).
3. **Given** a submitted operation form, **When** the response returns,
   **Then** it is displayed in a styled, scrollable view appropriate to the
   data shape (table for lists, detail view for single objects).
4. **Given** a developer theme configuration specifying brand colors and
   border style, **When** the app is regenerated, **Then** the TUI reflects
   the configured theme.
5. **Given** an app with TUI enabled, **When** the end user runs a command
   in a non-interactive environment (piped output, CI runner), **Then** the
   app falls back to plain CLI output automatically.

---

### User Story 5 - Extend Generated Code Without Losing Changes (Priority: P5)

A developer has generated an app and needs to add custom logic (telemetry,
custom validation, transformed responses) that does not exist in the
OpenAPI spec. When the spec changes and they regenerate, their custom code
MUST be preserved.

**Why this priority**: Real-world apps always need hand-written extensions.
If regeneration destroys custom work, developers will stop regenerating and
the tool loses its value.

**Independent Test**: Generate an app. Add custom code in the designated
custom code regions. Modify the OpenAPI spec (add a new operation). Regenerate.
Verify the new operation appears AND all custom code is preserved.

**Acceptance Scenarios**:

1. **Given** a generated file with marked custom code regions, **When** the
   developer adds code inside an custom code region and regenerates, **Then**
   the custom code is preserved in the new output.
2. **Given** custom code in custom code regions, **When** the developer runs
   a preview command before regenerating, **Then** a diff is shown
   indicating what will change and confirming custom regions are safe.
3. **Given** a regeneration that would structurally conflict with an
   custom code region (e.g., the operation was removed from the spec), **When**
   the developer regenerates, **Then** a warning is emitted listing affected
   regions and a backup is created before any files are overwritten.

---

### User Story 6 - Handle Pagination, Retries, and Errors Gracefully (Priority: P6)

End users interacting with paginated endpoints need seamless auto-pagination.
Transient failures should be retried automatically. Errors should be
displayed clearly with enough context to diagnose issues.

**Why this priority**: These runtime behaviors are essential for a
production-grade CLI. Without them, users must implement retry loops and
manual pagination themselves.

**Independent Test**: Call a paginated endpoint with the auto-paginate flag
and verify all pages are fetched. Simulate a transient 503 and verify the
request is retried and eventually succeeds. Trigger a validation error and
verify a clear, structured error message is displayed.

**Acceptance Scenarios**:

1. **Given** a paginated endpoint, **When** the end user passes the
   auto-paginate flag, **Then** all pages are fetched and the combined
   results are returned as a single response.
2. **Given** a server returning HTTP 503, **When** the client retries,
   **Then** it uses increasing delays between attempts and respects the
   server's Retry-After header if present.
3. **Given** a server returning a validation error (HTTP 422) with a
   structured error body, **When** the error is displayed, **Then** the
   output includes the status code, field-level error messages, and a
   request identifier if available.
4. **Given** retry configuration set to a maximum of 3 attempts, **When**
   all attempts fail, **Then** the final error is displayed with a note
   indicating how many retries were attempted.

---

### User Story 7 - Distribute the Generated App (Priority: P7)

A developer wants to ship the generated CLI/TUI app to end users on
multiple platforms with a standard installation experience (Homebrew on
macOS, native packages on Linux, WinGet on Windows).

**Why this priority**: A CLI tool that cannot be easily installed will not
be adopted. Distribution support is table stakes for any production CLI.

**Independent Test**: Run the release pipeline on a generated app. Verify
that cross-platform binaries are produced, a Homebrew formula is generated,
and install scripts work on macOS and Linux.

**Acceptance Scenarios**:

1. **Given** a generated app with release configuration enabled, **When**
   the developer tags a version and triggers the release pipeline, **Then**
   cross-platform binaries are produced for macOS, Linux, and Windows.
2. **Given** Homebrew distribution enabled, **When** the release completes,
   **Then** a Homebrew formula is published that allows end users to install
   the app via `brew install`.
3. **Given** a generated app, **When** the version is displayed via
   `--version`, **Then** it shows the semantic version, build commit, and
   build date.

---

### Edge Cases

- What happens when the OpenAPI spec contains no operations? The tool
  MUST produce an error explaining that at least one operation is required.
- What happens when two operations would generate the same CLI command
  name after stutter removal? The tool MUST detect the collision and
  either disambiguate automatically or produce a clear error.
- What happens when the user provides both a `--body` JSON payload and
  individual flags for the same fields? Individual flags MUST take
  precedence over the body payload, with the merged result sent.
- What happens when the TUI is launched in a terminal that does not
  support required capabilities (e.g., no color, insufficient width)?
  The app MUST degrade gracefully, disabling unsupported features with
  a warning rather than crashing.
- What happens when the operating system has no secure credential store
  available? The app MUST fall back to encrypted file-based storage and
  warn the user that an alternative storage method is in use.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The tool MUST accept OpenAPI 3.0 and 3.1 specifications as
  input and produce a complete, compilable application project as output.
- **FR-002**: Each API operation in the spec MUST map to a distinct
  command in the generated app, grouped by OpenAPI tag.
- **FR-003**: Command names MUST remove redundant prefixes/suffixes
  (stutter removal) by default, with an option to disable this behavior.
- **FR-004**: Each operation parameter MUST be exposed as a typed CLI
  flag with a help description sourced from the spec.
- **FR-005**: The generated app MUST support input via individual flags,
  a JSON body payload, and piped stdin, with flags taking highest
  precedence.
- **FR-006**: The generated app MUST support output in at least four
  formats: human-readable (pretty), JSON, YAML, and table.
- **FR-007**: The tool MUST support a standalone configuration file as
  the primary customization mechanism, with per-operation inline spec
  annotations as a secondary mechanism.
- **FR-008**: The generated app MUST support authentication methods
  declared in the OpenAPI spec's `securitySchemes`, including at minimum
  API key, HTTP basic, and bearer token.
- **FR-009**: Credentials MUST be stored in the operating system's secure
  credential store by default, with encrypted file-based fallback.
- **FR-010**: The generated app MUST support multiple named profiles,
  each with its own server URL, auth method, and credentials.
- **FR-011**: The generated app MUST be runnable in at least three modes:
  non-interactive CLI, interactive TUI, and hybrid (CLI with inline
  interactive prompts for missing required parameters).
- **FR-012**: Mode selection MUST follow a defined precedence: explicit
  flags, then environment variable, then user config file, then
  auto-detection based on terminal capabilities.
- **FR-013**: The generated app MUST provide a `-y` / `--yes` flag that
  skips all confirmation prompts and uses defaults for optional inputs.
- **FR-014**: The tool MUST support developer-defined custom code regions
  in generated code that are preserved across regeneration.
- **FR-015**: The tool MUST provide a diff/preview command that shows
  what regeneration would change before applying it.
- **FR-016**: The generated app MUST auto-paginate when the endpoint is
  paginated and the user requests all results.
- **FR-017**: The generated app MUST retry failed requests with
  configurable backoff for transient errors and rate limits.
- **FR-018**: The generated app MUST produce structured, actionable error
  messages including status code, error body, and request identifier.
- **FR-019**: The tool MUST generate shell completion scripts for at
  least bash, zsh, fish, and PowerShell.
- **FR-020**: The tool MUST generate human-readable documentation and
  machine-readable documentation optimized for LLM consumption from the
  command tree.
- **FR-021**: The generated app MUST use semantic versioning, with the
  version injected at build time.
- **FR-022**: The tool MUST generate release and distribution
  configuration for cross-platform builds.
- **FR-023**: The generated app MUST support developer-configurable
  theming for TUI colors, borders, and component styles.
- **FR-024**: The generation pipeline MUST expose lifecycle and transform
  hooks that developers can use to customize behavior at each stage.
- **FR-025**: Sensitive values MUST be automatically redacted in all
  debug output, dry-run output, error messages, and logs.
- **FR-026**: The generated app MUST support server selection from the
  servers listed in the OpenAPI spec, including server variable
  substitution.
- **FR-027**: The tool MUST support a dry-run mode that displays the
  HTTP request that would be sent without executing it.
- **FR-028**: The generated app MUST detect AI agent environments and
  automatically switch to a structured, non-interactive output mode.

### Key Entities

- **OpenAPI Specification**: The input document describing the API's
  operations, parameters, schemas, security, and servers.
- **Operation**: A single API endpoint (method + path) that maps to one
  CLI command and one TUI form.
- **Profile**: A named set of server URL, authentication method, and
  stored credentials for a specific environment.
- **Custom Code Region**: A marked section in generated source code where
  developers may add custom logic that survives regeneration.
- **Theme**: A set of color, border, and component style definitions
  controlling the visual appearance of the TUI.
- **Hook**: A developer-defined action (shell command, function, or
  plugin) that runs at a specific point in the generation pipeline.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can go from a valid OpenAPI spec to a working,
  installable application in under 5 minutes with no manual code edits.
- **SC-002**: Every operation in a 50+ operation OpenAPI spec is
  correctly represented as a CLI command with accurate flags and help text
  after a single generation run.
- **SC-003**: End users can authenticate, switch profiles, and make
  authenticated API calls in under 2 minutes on first use via the
  interactive login flow.
- **SC-004**: Custom code in custom code regions survives at least 10
  consecutive regeneration cycles with zero data loss.
- **SC-005**: The generated app responds to `--help` on any command in
  under 200 milliseconds.
- **SC-006**: Auto-pagination retrieves all pages from an endpoint with
  1000+ items without user intervention.
- **SC-007**: Transient errors (503, 429) are retried and resolved
  without user awareness when the server recovers within the retry
  window.
- **SC-008**: The TUI mode is usable for browsing and executing all
  operations without requiring the user to memorize any command names.
- **SC-009**: 90% of developers using the tool for the first time can
  produce a working app without consulting documentation beyond the
  `--help` output.
- **SC-010**: The generated app can be installed by end users on macOS,
  Linux, and Windows using their platform's standard package manager.

## Assumptions

- Developers have a valid, well-formed OpenAPI 3.0 or 3.1 specification
  for their API before using Cliford. The tool does not create or edit
  specs.
- The target deployment environment for generated apps supports modern
  terminal emulators with at minimum ANSI color support. TUI features
  degrade gracefully on limited terminals.
- Generated apps target server-side APIs over HTTPS. Local/development
  use over HTTP is supported but will trigger security warnings.
- The operating system provides either a native keychain service or
  sufficient filesystem permissions for encrypted credential storage.
- End users have network access to the target API server from the
  machine where the generated app runs.
- Developers using the custom code region feature are comfortable reading
  and writing in the generated project's programming language.
- Distribution via platform package managers requires the developer to
  have appropriate accounts and permissions (e.g., a GitHub repository
  for Homebrew tap publishing).
