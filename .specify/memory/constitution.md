<!--
Sync Impact Report
==================
Version change: N/A → 1.0.0 (initial ratification)
Modified principles: N/A (first version)
Added sections:
  - Core Principles (8 principles)
  - Technology Constraints
  - Development Workflow
  - Governance
Removed sections: N/A
Templates requiring updates:
  - .specify/templates/plan-template.md — ✅ compatible (Constitution Check section aligns)
  - .specify/templates/spec-template.md — ✅ compatible (requirements/scenarios structure aligns)
  - .specify/templates/tasks-template.md — ✅ compatible (phase structure supports principles)
Follow-up TODOs: None
-->

# Cliford Constitution

## Core Principles

### I. OpenAPI as Source of Truth

The OpenAPI specification is the single authoritative input for all code
generation. Every CLI command, TUI view, SDK method, flag, and parameter
MUST be derivable from the OpenAPI spec and Cliford configuration. No
generated artifact may introduce API surface area that is not rooted in
the spec. Configuration that extends behavior (hooks, themes, aliases)
MUST be layered on top of the spec, never contradict it.

### II. SDK-First Architecture

All code generation follows an SDK-first pipeline: parse the OpenAPI
spec, generate a typed Go SDK via oapi-codegen, then build CLI and TUI
layers on top of that SDK. The CLI and TUI layers MUST NOT make HTTP
calls directly; they MUST invoke SDK methods. This ensures a single
implementation of auth, retries, pagination, and error handling shared
across all presentation modes. The SDK is the contract boundary.

### III. Three Modes, One Codebase

Cliford MUST generate applications that support pure CLI, pure TUI, and
hybrid CLI+TUI modes from a single generation pass. Mode selection is
resolved via explicit flags (`--tui`, `--no-interactive`, `-y`),
environment variables, user config, and auto-detection (TTY, agent
environment) in that precedence order. No mode may require a separate
generation run or a different project structure. The Adapter Pattern
(CLI adapter, TUI adapter, headless adapter) reading from a shared
Operation Registry is the canonical architecture for this split.

### IV. Configuration Over Convention

Cliford and the apps it generates MUST be configurable at every
meaningful layer. Cliford itself accepts configuration via
`cliford.yaml` (primary), `x-cliford-*` OpenAPI extensions
(per-operation), and optionally a Go DSL (`cliford.go`) for advanced
use. Generated apps MUST use Viper for end-user configuration with a
clear resolution order: CLI flags > environment variables > config file >
defaults. Credential resolution follows: flags > env > OS keychain >
config file. Every behavior that a reasonable developer might want to
change MUST be configurable without modifying generated code.

### V. Custom Code Survives Regeneration

Generated code MUST support custom code regions: clearly marked
sections where developers can add logic that persists across
regeneration cycles. Cliford MUST detect existing custom code regions
before overwriting, warn if any would be lost, and preserve them in the
new output. A `cliford diff` command MUST exist to preview changes
before applying them. Backup of the previous generation MUST be stored
before overwriting.

### VI. Extensibility via Hooks

The generation pipeline MUST expose lifecycle hooks at every stage
(`before:generate`, `before:sdk`, `after:sdk`, `before:cli`,
`after:cli`, `before:tui`, `after:tui`, `before:docs`, `after:docs`,
`after:generate`) and transform hooks (`transform:operation`,
`transform:command`, `transform:model`, `transform:style`). Hooks MUST
be definable as shell commands in `cliford.yaml`, Go functions in
`cliford.go`, or external plugin binaries. Hooks MUST NOT be required
for basic operation; the tool MUST produce fully functional output with
zero hooks configured.

### VII. Idiomatic Go Output

All generated Go code MUST be idiomatic, pass `go vet`, and be
formatted with `gofmt`. Generated code MUST use standard library
patterns where possible and MUST NOT introduce unnecessary abstractions.
Dependencies MUST be minimized and explicitly justified. The generated
project MUST be a valid Go module that compiles with `go build` and
whose tests pass with `go test ./...` immediately after generation with
no manual intervention.

### VIII. Security by Default

Generated apps MUST store credentials in the OS keychain by default
with an encrypted-file fallback for environments without keychain
access. Credentials MUST NOT appear in debug output, dry-run output,
error messages, or log files; all sensitive values MUST be redacted
automatically. The tool MUST warn when sending credentials over
non-HTTPS connections. Auth tokens MUST have lifecycle management
(expiry tracking, proactive refresh for OAuth). No generated code may
introduce OWASP Top 10 vulnerabilities.

## Technology Constraints

The following technology stack is mandatory for Cliford and the
applications it generates:

- **SDK Generation**: oapi-codegen (as a Go library, not CLI binary)
- **CLI Framework**: Cobra (command tree, flags, help, completions)
- **TUI Framework**: Bubbletea (Elm architecture terminal UIs)
- **TUI Components**: Bubbles (pre-built UI primitives)
- **TUI Styling**: Lipgloss (CSS-inspired terminal styling)
- **Configuration**: Viper (both Cliford itself and generated apps)
- **Distribution**: GoReleaser (cross-platform builds, packaging)
- **Auth Storage**: go-keyring or equivalent (cross-platform keychain)
- **Language**: Go 1.22+ (for both Cliford and generated output)
- **Spec Format**: OpenAPI 3.0 and 3.1

Generated apps MUST auto-generate documentation via Cobra's doc package
in both Markdown (human-readable) and llms.txt (LLM-optimized) formats.
Generated apps MUST use SemVer for versioning, injected via ldflags at
build time.

Third-party dependencies in generated apps MUST be limited to the stack
above plus explicitly configured `additionalDependencies`. Every
dependency MUST serve a clear, documented purpose.

## Development Workflow

### Cliford Development (the tool itself)

1. **Test-First**: New features and bug fixes MUST begin with failing
   tests. The Red-Green-Refactor cycle is enforced.
2. **Integration Tests**: The generation pipeline MUST have end-to-end
   tests that generate a complete app from a reference OpenAPI spec,
   compile it, and verify it runs.
3. **Golden File Tests**: Generated output MUST be compared against
   golden files to detect unintended changes.
4. **Dogfooding**: Cliford's own CLI interface MUST be built with Cobra
   and follow the same patterns it generates.

### Generated App Quality Gates

Every generated app MUST pass the following checks immediately after
generation with no manual steps:

1. `go build ./...` succeeds
2. `go vet ./...` reports no issues
3. `go test ./...` passes (generated test scaffolding)
4. The binary starts and responds to `--help`
5. Shell completions generate without error
6. Documentation generates without error

### Code Review Standards

- All changes to generation templates MUST include before/after diffs
  of generated output for a reference spec.
- Custom code region preservation MUST be verified in any PR that
  modifies template structure.
- Hook execution order MUST be documented and tested.

## Governance

This constitution supersedes all ad-hoc practices, verbal agreements,
and undocumented conventions for the Cliford project. All contributions
MUST comply with these principles.

### Amendment Procedure

1. Propose amendment as a pull request modifying this file.
2. Amendment MUST include rationale and impact assessment.
3. Amendment MUST update the version according to SemVer:
   - MAJOR: Principle removal, redefinition, or backward-incompatible
     governance change.
   - MINOR: New principle or section added, material expansion of
     existing guidance.
   - PATCH: Clarifications, typo fixes, non-semantic refinements.
4. Amendment MUST propagate changes to dependent templates and
   documentation.
5. All maintainers MUST review and approve before merge.

### Compliance Review

- Every pull request MUST be checked against applicable principles.
- The plan template's Constitution Check section MUST gate
  implementation work.
- Quarterly review of this constitution for relevance and completeness.

Refer to `CLAUDE.md` for runtime development guidance specific to AI
agent workflows.

**Version**: 1.0.0 | **Ratified**: 2026-04-10 | **Last Amended**: 2026-04-10
