# Contract: Cliford CLI Interface

**Date**: 2026-04-10

This document defines the CLI interface contract for Cliford itself (the code
generation tool).

## Commands

### `cliford init`

Initialize a new Cliford project from an OpenAPI spec.

```
cliford init [--spec <path>] [--name <name>] [--package <module-path>]
             [--mode <pure-cli|pure-tui|hybrid>] [--interactive]
```

**Behavior**: Creates `cliford.yaml` with defaults derived from the spec.
If `--interactive` (default when TTY), launches a TUI wizard for
configuration. Otherwise uses flags and sensible defaults.

**Output**: `cliford.yaml` in current directory + confirmation message.

**Exit codes**: 0 success, 1 spec parse error, 2 invalid config.

---

### `cliford generate`

Run the full generation pipeline.

```
cliford generate [--spec <path>] [--config <path>] [--output-dir <path>]
                 [--dry-run] [--force] [--verbose]
```

**Flags**:
- `--spec`: Override spec path from config (default: from cliford.yaml)
- `--config`: Config file path (default: `./cliford.yaml`)
- `--output-dir`: Output directory (default: current directory)
- `--dry-run`: Show what would be generated without writing files
- `--force`: Overwrite without backup/diff confirmation
- `--verbose`: Detailed pipeline progress output

**Behavior**: Executes all pipeline stages (parse -> SDK -> CLI -> TUI ->
infra). Respects custom code regions. Creates backup before overwriting.
Runs hooks at each stage.

**Output**: Generated Go project files. Summary of files created/modified.

**Exit codes**: 0 success, 1 spec error, 2 config error, 3 generation error,
4 hook error.

---

### `cliford diff`

Preview changes that regeneration would make.

```
cliford diff [--spec <path>] [--config <path>]
```

**Behavior**: Runs generation to a temporary directory and diffs against
existing output. Highlights custom code regions that are safe.

**Output**: Unified diff to stdout.

---

### `cliford validate`

Validate configuration and spec.

```
cliford validate [--spec <path>] [--config <path>]
```

**Behavior**: Parses spec and config, checks for errors, validates
extension annotations, reports issues.

**Output**: List of errors/warnings. Exit 0 if valid, 1 if errors.

---

### `cliford version bump <type>`

Bump the app version.

```
cliford version bump <auto|patch|minor|major>
```

**Behavior**: For `auto`, diffs current spec against the previously
generated spec (from lockfile) and determines bump type. Updates version
in `cliford.yaml`.

---

### `cliford config show`

Display current configuration.

```
cliford config show [--format <json|yaml|table>]
```

---

### `cliford config init`

Create cliford.yaml interactively.

```
cliford config init [--spec <path>]
```

Alias for the interactive portion of `cliford init`.

---

### `cliford doctor`

Check environment and dependencies.

```
cliford doctor
```

**Behavior**: Verifies Go version, oapi-codegen availability, config
validity, spec parsability. Reports pass/fail for each check.

---

## Global Flags (all commands)

```
--help, -h          Show help
--version, -v       Show version
--quiet, -q         Suppress non-error output
--no-color          Disable colored output
```

## Environment Variables

```
CLIFORD_CONFIG      Path to cliford.yaml (overrides --config)
CLIFORD_SPEC        Path to OpenAPI spec (overrides --spec)
CLIFORD_NO_HOOKS    Disable all hooks (set to "true")
CLIFORD_NO_COLOR    Disable colored output
```
