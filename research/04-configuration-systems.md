# Configuration Systems Research

> How Cliford is configured, and how the apps it generates are configured. Exploring OpenAPI extensions, Viper, and alternative approaches.

## Two Levels of Configuration

Cliford has a unique dual-config challenge:

1. **Cliford Config** - How developers configure the code generation tool itself
2. **App Config** - How end users of the generated apps configure their experience

Both should use Viper, but with different schemas and purposes.

---

## Level 1: Cliford Configuration (Developer-Facing)

### Approach A: OpenAPI Extensions (Speakeasy-Style)

Embed config directly in the OpenAPI spec using `x-cliford-*` extensions:

```yaml
# In your OpenAPI spec
info:
  title: My API
  x-cliford:
    cliName: myapp
    packageName: github.com/myorg/myapp-cli
    envVarPrefix: MYAPP
    mode: hybrid # pure-cli | pure-tui | hybrid

paths:
  /users:
    get:
      operationId: listUsers
      x-cliford-cli:
        aliases: [ls, list]
        hidden: false
        group: users
      x-cliford-tui:
        displayAs: table
        refreshable: true
      x-cliford-pagination:
        style: cursor
      x-cliford-retries:
        enabled: true
        maxAttempts: 3
```

**Pros:**

- Single source of truth
- Config lives with the API definition
- Natural for teams already using OpenAPI
- Version controlled with the spec

**Cons:**

- Pollutes the OpenAPI spec with CLI concerns
- Harder to share specs with non-CLI consumers
- Limited expressiveness (YAML only)
- Harder to do complex logic (e.g., conditional hooks)

### Approach B: Standalone Config File (cliford.yaml)

Separate config file alongside the OpenAPI spec:

```yaml
# cliford.yaml
version: "1"

spec: ./openapi.yaml

app:
  name: myapp
  package: github.com/myorg/myapp-cli
  envVarPrefix: MYAPP
  version: 0.1.0
  description: "CLI for My API"

generation:
  mode: hybrid # pure-cli | pure-tui | hybrid
  sdk:
    generator: oapi-codegen
    outputDir: internal/sdk
    config:
      package: sdk
      generate:
        - types
        - client
  cli:
    framework: cobra
    outputDir: internal/cli
  tui:
    enabled: true
    framework: bubbletea
    outputDir: internal/tui

auth:
  interactive: true
  keychain: true
  methods:
    - bearer
    - apiKey

theme:
  colors:
    primary: "#7D56F4"
    accent: "#4ECDC4"
    error: "#FF4444"
  borders: rounded

features:
  pagination: true
  retries:
    enabled: true
    maxAttempts: 3
    backoff: exponential
  documentation:
    markdown: true
    llmsTxt: true
  distribution:
    goreleaser: true
    homebrew: false

operations:
  listUsers:
    cli:
      aliases: [ls]
      group: users
    tui:
      displayAs: table
  createUser:
    cli:
      confirmBeforeExecute: true
    tui:
      displayAs: form
```

**Pros:**

- Clean separation from OpenAPI spec
- Richer configuration (nested objects, arrays)
- Can reference external files (theme files, hook scripts)
- Easier to manage in large projects
- Familiar YAML config pattern

**Cons:**

- Separate file to maintain
- Config can drift from spec
- Need to reference operations by ID (coupling)

### Approach C: Hybrid (Recommended for Cliford)

Use **both** approaches. The standalone `cliford.yaml` is the primary config, but also support `x-cliford-*` extensions in the OpenAPI spec for per-operation overrides. The standalone config takes precedence.

Resolution order:

1. `cliford.yaml` operation-level config (highest)
2. `x-cliford-*` OpenAPI extensions
3. `cliford.yaml` global defaults
4. Cliford built-in defaults (lowest)

This lets teams:

- Keep the bulk of config in `cliford.yaml`
- Add per-operation hints inline in the spec where it makes sense
- Share the OpenAPI spec without CLI noise (extensions are ignored by other tools)

### Approach D: Go DSL (Advanced Users)

For power users, allow a `cliford.go` file that programmatically configures generation:

```go
// cliford.go
package main

import "github.com/the-inconvenience-store/cliford/config"

func Configure() *config.Config {
    return config.New().
        Name("myapp").
        Spec("./openapi.yaml").
        Mode(config.Hybrid).
        OnBeforeGenerate(func(ctx *config.Context) {
            // Custom pre-generation logic
        }).
        Operation("listUsers", func(op *config.Operation) {
            op.Aliases("ls", "list")
            op.TUIDisplay(config.Table)
            op.AddFlag("--active-only", "Filter to active users", false)
        })
}
```

**Pros:**

- Full programming language power
- Type-safe configuration
- IDE autocompletion
- Complex conditional logic
- Custom hooks as real functions

**Cons:**

- Higher barrier to entry
- Not as declarative
- Harder to validate statically
- Requires Go knowledge

**Recommendation**: Offer the Go DSL as an optional advanced config alongside `cliford.yaml`. The YAML config covers 90% of use cases; the Go DSL covers the remaining 10%.

---

## Level 2: App Configuration (End-User-Facing)

Generated apps use Viper for end-user configuration.

### Config File Location

Follow XDG Base Directory Specification:

```
~/.config/<app>/config.yaml    # Primary config
~/.config/<app>/auth/           # Auth credentials
~/.config/<app>/themes/         # Custom themes (optional)
```

Also support:

- `./<app>.yaml` in current directory (project-level override)
- Environment variables with `<PREFIX>_` prefix
- Command-line flags (highest priority)

### Viper Integration in Generated Apps

```go
// Generated config initialization
func initConfig() {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(filepath.Join(xdgConfigHome(), appName))
    viper.AddConfigPath(".")

    viper.SetEnvPrefix(envVarPrefix)
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

    viper.ReadInConfig() // Not fatal if missing
}
```

### Default Generated Config Schema

```yaml
# ~/.config/myapp/config.yaml

# Server configuration
server:
  url: https://api.example.com
  timeout: 30s

# Authentication
auth:
  method: bearer # bearer | api-key | basic | oauth
  # Credentials stored in keychain, not here

# Display preferences
output:
  format: pretty # pretty | json | yaml | table
  color: auto # auto | always | never
  pager: true # Use pager for long output

# Mode preferences
mode: hybrid # cli | tui | headless
interactive: true # Prompt for missing params
confirm_destructive: true # Confirm before DELETE/PUT

# TUI preferences (when in TUI/hybrid mode)
tui:
  theme: default # default | dark | light | custom
  animations: true
  mouse: true

# Profiles
profiles:
  default:
    server:
      url: https://api.example.com
  staging:
    server:
      url: https://staging.api.example.com

active_profile: default
```

### Config Commands (Generated)

```
myapp config show              # Display current config
myapp config set <key> <value> # Set a config value
myapp config get <key>         # Get a config value
myapp config reset             # Reset to defaults
myapp config edit              # Open config in $EDITOR
myapp config use-profile <name> # Switch profiles
myapp config path              # Show config file location
```

---

## Viper in Cliford Itself

Cliford also uses Viper for its own operation:

```yaml
# ~/.config/cliford/config.yaml

defaults:
  mode: hybrid
  sdk_generator: oapi-codegen
  theme:
    primary: "#7D56F4"
  features:
    pagination: true
    retries: true
    documentation: true

templates:
  custom_dir: ~/.config/cliford/templates/

hooks:
  before_generate: []
  after_generate: []
  before_sdk: []
  after_sdk: []
```

This provides user-level defaults that apply to all Cliford projects unless overridden by the project's `cliford.yaml`.

---

## Hook System Design

Hooks are the primary extensibility mechanism. They run at defined points in the generation pipeline.

### Hook Types

1. **Lifecycle Hooks** - Run at generation pipeline stages
   - `before:generate` / `after:generate`
   - `before:sdk` / `after:sdk`
   - `before:cli` / `after:cli`
   - `before:tui` / `after:tui`
   - `before:docs` / `after:docs`

2. **Transform Hooks** - Modify generated code
   - `transform:operation` - Modify operation config before codegen
   - `transform:command` - Modify Cobra command after generation
   - `transform:model` - Modify Bubbletea model after generation
   - `transform:style` - Modify Lipgloss styles

3. **Custom Hooks** - User-defined extension points
   - `custom:<name>` - Arbitrary named hooks

### Hook Implementation

Hooks can be defined as:

**Shell commands** (in `cliford.yaml`):

```yaml
hooks:
  after:generate:
    - run: "gofmt -w ./internal/"
    - run: "go vet ./..."
```

**Go functions** (in `cliford.go`):

```go
config.Hook("after:generate", func(ctx *hook.Context) error {
    // Custom post-generation logic
    return nil
})
```

**External plugins** (as Go plugins or separate binaries):

```yaml
plugins:
  - name: my-custom-plugin
    path: ./plugins/myplugin
    hooks:
      - transform:operation
      - after:generate
```

---

## Configuration Validation

Both Cliford config and generated app config should have JSON Schema definitions for:

- IDE autocompletion
- Config file validation
- Documentation generation

Cliford generates a JSON Schema for each app's config based on the OpenAPI spec and generation options.

```
$ cliford validate              # Validate cliford.yaml
$ myapp config validate         # Validate app config (generated command)
```
