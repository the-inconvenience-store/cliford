# Hooks

Cliford provides two distinct hook systems: **pipeline hooks** that run during
code generation, and **runtime hooks** that run in the generated application
when API requests are made.

## Pipeline hooks (generation time)

Pipeline hooks run shell commands at defined points during the `cliford
generate` pipeline. They let you modify the generation output without editing
Cliford's source.

### Hook points

#### Lifecycle hooks

| Hook | When |
|------|------|
| `before:generate` | Before any generation begins |
| `after:validate` | After spec and config validation |
| `before:sdk` | Before SDK generation |
| `after:sdk` | After SDK generation |
| `before:cli` | Before CLI command generation |
| `after:cli` | After CLI command generation |
| `before:tui` | Before TUI generation |
| `after:tui` | After TUI generation |
| `before:docs` | Before documentation generation |
| `after:docs` | After documentation generation |
| `after:generate` | After all generation completes |

#### Transform hooks

Transform hooks modify metadata by reading JSON from stdin and writing
modified JSON to stdout.

| Hook | Data |
|------|------|
| `transform:operation` | Operation metadata (per operation) |
| `transform:command` | Cobra command metadata |
| `transform:model` | Bubbletea model metadata |
| `transform:style` | Lipgloss style metadata |

### How to define pipeline hooks

Add shell commands to `cliford.yaml`:

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

### Context variables

Shell commands can reference these variables:

| Variable | Value |
|----------|-------|
| `$SPEC_PATH` | Path to OpenAPI spec |
| `$OUTPUT_DIR` | Generation output directory |
| `$STAGE` | Current stage name |
| `$APP_NAME` | App binary name |

### Transform hook example

A transform hook that adds a custom flag to every operation:

```bash
#!/bin/sh
# add-trace-flag.sh
jq '. + {customFlags: [{name: "trace", type: "bool", description: "Enable tracing"}]}'
```

```yaml
hooks:
  "transform:operation":
    - run: "./scripts/add-trace-flag.sh"
```

### Execution behavior

- Lifecycle hooks run in the **output directory** as working directory.
- Transform hooks receive JSON on **stdin** and must write valid JSON to
  **stdout**. If stdout is empty, the input passes through unchanged.
- A **non-zero exit code** from any hook fails the pipeline.
- Hook **stderr** is printed to the terminal for debugging.
- Hooks are **optional**. Cliford produces fully functional output with zero
  hooks configured.

## Runtime hooks (generated app)

Runtime hooks run in the generated application at request time. They allow
end users to inject custom logic around every HTTP request without modifying
the generated code.

### Hook types

| Type | Mechanism | Use case |
|------|-----------|----------|
| `shell` | Exec subprocess, JSON on stdin | Logging, auditing, simple header injection |
| `go-plugin` | hashicorp/go-plugin via net/rpc | Header injection from external logic, credential enrichment |

### How to configure runtime hooks

They are configured in `cliford.yaml` and baked into the generated app at generation
time. Every user who runs the generated binary gets the same hooks.

```yaml
# cliford.yaml
features:
  hooks:
    beforeRequest:
      - type: shell
        command: "scripts/audit-log.sh"
      - type: shell
        command: "scripts/inject-trace-header.sh"
      - type: go-plugin
        pluginPath: "./bin/trace-plugin"
    afterResponse:
      - type: shell
        command: "scripts/log-response.sh"
```

Run `cliford generate` to regenerate after changing hooks.

### Shell hook protocol

Shell hooks receive a JSON object on stdin with this structure:

**before_request:**

```json
{
  "operation_id": "listPets",
  "method": "GET",
  "url": "https://api.example.com/pets?limit=10",
  "headers": {
    "Authorization": "[REDACTED]",
    "Content-Type": "application/json"
  },
  "timestamp": "2026-04-11T10:30:00Z"
}
```

**after_response** (same fields plus response data):

```json
{
  "operation_id": "listPets",
  "method": "GET",
  "url": "https://api.example.com/pets?limit=10",
  "headers": {"Authorization": "[REDACTED]"},
  "timestamp": "2026-04-11T10:30:00Z",
  "status_code": 200,
  "response_headers": {"Content-Type": "application/json"},
  "elapsed_ms": 142
}
```

### Exit code behavior

- **before_request hooks**: A non-zero exit code **aborts the request**. The
  error message from stderr is shown to the user.
- **after_response hooks**: A non-zero exit code is **logged but does not
  fail** the command. The response is still displayed.

### Sensitive header redaction

Headers containing `authorization`, `secret`, `token`, `key`, or `password`
(case-insensitive) are replaced with `[REDACTED]` in the hook context JSON.
This prevents credential leakage to hook scripts.

### go-plugin hooks

go-plugin hooks run as separate subprocess binaries managed by the generated
app via [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) with
net/rpc transport. Unlike shell hooks, they can modify outgoing request headers
and receive typed `HookResponse` values.

#### The HookService interface

Your plugin binary must implement this interface:

```go
type HookService interface {
    BeforeRequest(ctx HookContext) (*HookResponse, error)
    AfterResponse(ctx HookContext) (*HookResponse, error)
}
```

`HookContext` carries the same fields as the shell hook JSON (operation ID,
method, URL, headers, timestamp, status code, etc.).

`HookResponse` has three fields:

| Field | Type | Description |
|-------|------|-------------|
| `Abort` | `bool` | If true, the request is cancelled and the error is shown to the user |
| `AbortReason` | `string` | Message displayed when `Abort` is true |
| `ModifiedHeaders` | `map[string]string` | Headers to set on the outgoing request (before_request only) |

#### Minimal plugin skeleton

```go
package main

import (
    "github.com/hashicorp/go-plugin"
    hooks "myapp/internal/hooks"
)

// Handshake constants — must match the generated app exactly.
var handshakeConfig = plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "CLIFORD_HOOK_PLUGIN",
    MagicCookieValue: "hook_v1",
}

type MyHookPlugin struct{}

func (p *MyHookPlugin) BeforeRequest(ctx hooks.HookContext) (*hooks.HookResponse, error) {
    return &hooks.HookResponse{
        ModifiedHeaders: map[string]string{
            "X-Request-ID": generateID(),
        },
    }, nil
}

func (p *MyHookPlugin) AfterResponse(ctx hooks.HookContext) (*hooks.HookResponse, error) {
    return nil, nil // no-op
}

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: handshakeConfig,
        Plugins: plugin.PluginSet{
            "hook": &hooks.HookPlugin{Impl: &MyHookPlugin{}},
        },
    })
}
```

Build and wire the plugin:

```bash
go build -o ./plugins/my-hook ./cmd/my-hook

# ~/.config/myapp/config.yaml
features:
  hooks:
    enabled: true
hooks:
  before_request:
    - type: go-plugin
      plugin_path: "./plugins/my-hook"
```

The generated app spawns the plugin binary once and reuses the subprocess for
all requests. The `CLIFORD_HOOK_PLUGIN=hook_v1` magic cookie prevents random
binaries from being accidentally loaded as plugins.

## Pipeline hooks vs runtime hooks

| Aspect | Pipeline hooks | Runtime hooks |
|--------|---------------|---------------|
| When | During `cliford generate` | During generated app execution |
| Configured in | `cliford.yaml` | Generated app's config file |
| Purpose | Modify generation output | Modify HTTP requests/responses |
| Failure behavior | Halts generation pipeline | Aborts request (before) or logs warning (after) |
