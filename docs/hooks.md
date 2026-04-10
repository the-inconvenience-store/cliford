# Hooks

Hooks are the extensibility mechanism for the Cliford generation pipeline.
They run at defined points during generation, letting you modify behavior
without editing Cliford's source.

## Hook Points

### Lifecycle Hooks

Run shell commands before or after each pipeline stage.

| Hook | When |
|------|------|
| `before:generate` | Before any generation begins |
| `after:validate` | After spec + config validation |
| `before:sdk` | Before SDK generation |
| `after:sdk` | After SDK generation |
| `before:cli` | Before CLI command generation |
| `after:cli` | After CLI command generation |
| `before:tui` | Before TUI generation |
| `after:tui` | After TUI generation |
| `before:docs` | Before documentation generation |
| `after:docs` | After documentation generation |
| `after:generate` | After all generation completes |

### Transform Hooks

Modify metadata by reading JSON from stdin and writing modified JSON to stdout.

| Hook | Data |
|------|------|
| `transform:operation` | Operation metadata (per operation) |
| `transform:command` | Cobra command metadata |
| `transform:model` | Bubbletea model metadata |
| `transform:style` | Lipgloss style metadata |

## Defining Hooks

### Shell Commands (cliford.yaml)

```yaml
hooks:
  "before:generate":
    - run: "echo 'Starting generation...'"
  "after:generate":
    - run: "gofmt -w ."
    - run: "go vet ./..."
  "after:sdk":
    - run: "echo 'SDK generated at $OUTPUT_DIR/internal/sdk'"
```

Multiple hooks at the same point run sequentially. A non-zero exit code
halts the pipeline.

### Context Variables

Shell commands can reference these variables:

| Variable | Value |
|----------|-------|
| `$SPEC_PATH` | Path to OpenAPI spec |
| `$OUTPUT_DIR` | Generation output directory |
| `$STAGE` | Current stage name |
| `$APP_NAME` | App binary name |

### Transform Example

A transform hook that adds a custom flag to every operation:

```bash
# add-trace-flag.sh
#!/bin/sh
# Reads operation JSON from stdin, adds a trace flag, writes to stdout
jq '. + {customFlags: [{name: "trace", type: "bool", description: "Enable tracing"}]}'
```

```yaml
hooks:
  "transform:operation":
    - run: "./scripts/add-trace-flag.sh"
```

## Execution Behavior

- Lifecycle hooks run in the **output directory** as working directory.
- Transform hooks receive JSON on **stdin** and must write valid JSON to
  **stdout**. If stdout is empty, the input passes through unchanged.
- A **non-zero exit code** from any hook fails the pipeline.
- Hook **stderr** is printed to the terminal for debugging.
- Hooks are **optional** — Cliford produces fully functional output with
  zero hooks configured.

## Use Cases

| Goal | Hook | Example |
|------|------|---------|
| Format generated code | `after:generate` | `gofmt -w .` |
| Run linters | `after:generate` | `golangci-lint run` |
| Add custom middleware | `after:cli` | Script that injects imports |
| Generate additional files | `after:generate` | Script that creates test stubs |
| Validate output | `after:generate` | `go build ./... && go vet ./...` |
| Notify CI | `after:generate` | `curl -X POST https://ci.example.com/webhook` |
