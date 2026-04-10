# Implementation Plan: OpenAPI CLI & TUI Code Generation

**Branch**: `001-openapi-cli-codegen` | **Date**: 2026-04-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-openapi-cli-codegen/spec.md`

## Summary

Cliford is an OSS CLI tool and Go library that reads OpenAPI 3.0/3.1
specifications and generates production-ready Go applications supporting
pure CLI, pure TUI, and hybrid CLI+TUI modes. The generation pipeline
follows an SDK-first architecture: oapi-codegen produces typed Go client
code, Cliford enhances it with retry/pagination/auth middleware, then
builds Cobra commands and Bubbletea TUI views on top via the Adapter
Pattern reading from a shared Operation Registry. Configuration uses a
layered system (cliford.yaml > x-cliford-* OpenAPI extensions > defaults)
powered by Viper at both tool and app level.

## Technical Context

**Language/Version**: Go 1.22+ (for Cliford itself and all generated output)
**Primary Dependencies**:
  - oapi-codegen v2 (SDK generation, used as Go library)
  - kin-openapi (OpenAPI 3.0/3.1 spec parsing and validation)
  - Cobra (CLI command tree, flags, completions, doc generation)
  - Bubbletea (Elm-architecture TUI framework)
  - Bubbles (TUI components: text input, table, list, spinner, viewport, etc.)
  - Lipgloss (CSS-inspired terminal styling)
  - Viper (configuration management)
  - go-keyring (cross-platform OS keychain access)
  - GoReleaser (distribution and release automation)
**Storage**: OS keychain (credentials), filesystem (config YAML, encrypted fallback)
**Testing**: `go test` with golden file comparisons and end-to-end generation tests
**Target Platform**: macOS, Linux, Windows (cross-compiled via GoReleaser)
**Project Type**: CLI tool + Go library (dual: Cliford is both a CLI and a library)
**Performance Goals**: Generation of 50+ operation spec in <10s; generated `--help` in <200ms
**Constraints**: Generated apps must compile with zero manual intervention; all dependencies
  justified per Constitution Principle VII
**Scale/Scope**: Support OpenAPI specs with 200+ operations, generated apps used by thousands

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Status | Evidence |
|---|-----------|--------|----------|
| I | OpenAPI as Source of Truth | PASS | All commands/flags derived from spec via Operation Registry (research/02, research/05) |
| II | SDK-First Architecture | PASS | oapi-codegen generates SDK; CLI/TUI layers invoke SDK only (research/05 pipeline) |
| III | Three Modes, One Codebase | PASS | Adapter Pattern with CLI/TUI/headless adapters from shared registry (research/02) |
| IV | Configuration Over Convention | PASS | Layered config: cliford.yaml > x-cliford-* > defaults; Viper in both tools (research/04) |
| V | Custom Code Survives Regeneration | PASS | Marked extension regions with diff preview and backup (research/06) |
| VI | Extensibility via Hooks | PASS | Lifecycle + transform hooks at all pipeline stages (research/04) |
| VII | Idiomatic Go Output | PASS | go vet, gofmt enforced; minimal deps; generated quality gates in constitution |
| VIII | Security by Default | PASS | Keychain-first storage, auto-redaction, HTTPS warnings (research/03) |

All 8 principles satisfied. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/001-openapi-cli-codegen/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── cliford-cli.md
│   └── generated-app-cli.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
├── cliford/
│   └── main.go                      # Cliford binary entrypoint

internal/
├── openapi/
│   ├── parser.go                    # OpenAPI spec parsing + validation
│   ├── extensions.go                # x-cliford-* extension extraction
│   └── registry.go                  # Operation Registry builder
├── config/
│   ├── cliford_config.go            # cliford.yaml parsing (Viper)
│   ├── merge.go                     # Config resolution (yaml > extensions > defaults)
│   └── schema.go                    # JSON Schema generation for config validation
├── sdk/
│   ├── generator.go                 # oapi-codegen invocation (as library)
│   ├── auth_enhancer.go             # Auth middleware generation
│   ├── pagination_enhancer.go       # Pagination helper generation
│   ├── retry_enhancer.go            # Retry wrapper generation
│   ├── errors.go                    # Enhanced error type generation
│   └── regions.go                   # Custom code region management
├── cli/
│   ├── generator.go                 # Cobra command tree generation
│   ├── flags.go                     # Flag generation from operation params
│   ├── output.go                    # Output formatter generation (json/yaml/table/pretty)
│   ├── auth.go                      # Auth command generation (login/logout/status)
│   ├── config_cmd.go                # Config command generation (show/set/get)
│   └── completions.go              # Shell completion generation
├── tui/
│   ├── generator.go                 # Bubbletea app generation
│   ├── explorer.go                  # Explorer view generation (List bubble)
│   ├── operation.go                 # Operation form view generation
│   ├── response.go                  # Response viewport generation
│   ├── theme.go                     # Lipgloss theme engine generation
│   └── components.go               # Shared TUI component generation
├── hybrid/
│   ├── mode.go                      # Mode detection (TTY, agent, flags)
│   ├── prompt.go                    # Inline Bubbletea prompt generation
│   └── adapter.go                   # Adapter pattern wiring
├── hooks/
│   ├── lifecycle.go                 # before:/after: hook execution
│   ├── transform.go                 # transform: hook execution
│   └── registry.go                  # Hook registration from config
├── codegen/
│   ├── engine.go                    # Template engine (Go templates)
│   ├── regions.go                   # Custom code region preservation
│   ├── backup.go                    # Pre-generation backup
│   └── diff.go                      # Generation diff preview
├── distribution/
│   ├── goreleaser.go                # GoReleaser config generation
│   ├── install.go                   # Install script generation
│   └── homebrew.go                  # Homebrew formula generation
├── docs/
│   ├── markdown.go                  # Cobra doc generation (Markdown)
│   ├── llms.go                      # llms.txt generation
│   └── schema.go                    # --usage schema generation
└── pipeline/
    ├── pipeline.go                  # Orchestrates all generation stages
    ├── validate.go                  # Pre-generation validation
    └── lockfile.go                  # Generation lockfile management

pkg/
├── registry/
│   └── types.go                     # OperationMeta, ParamMeta, etc. (public types)
└── theme/
    └── types.go                     # Theme config types (public)

templates/
├── sdk/                             # Go templates for SDK enhancement files
├── cli/                             # Go templates for Cobra commands
├── tui/                             # Go templates for Bubbletea models
├── infra/                           # Templates for main.go, goreleaser, etc.
└── docs/                            # Templates for doc generation

testdata/
├── specs/
│   ├── petstore.yaml                # Reference spec for testing
│   ├── complex-auth.yaml            # Multi-auth spec
│   └── paginated.yaml               # Pagination patterns spec
├── golden/                          # Golden file outputs for comparison
└── fixtures/                        # Test config files

tests/
├── integration/
│   ├── generate_test.go             # End-to-end: spec -> compile -> run
│   ├── regions_test.go              # Custom code region preservation
│   └── modes_test.go                # CLI/TUI/headless mode tests
└── unit/
    ├── parser_test.go
    ├── registry_test.go
    ├── config_test.go
    └── ...
```

**Structure Decision**: Single Go module at repository root. The `internal/`
package contains all non-exported implementation organized by pipeline stage.
The `pkg/` package exports types needed by plugins/extensions. The `templates/`
directory holds Go text templates for code generation. The `testdata/` directory
holds reference specs and golden files for testing. This aligns with standard
Go project layout and Constitution Principle VII (idiomatic Go).

## Complexity Tracking

No violations. All 8 constitution principles satisfied without exception.
