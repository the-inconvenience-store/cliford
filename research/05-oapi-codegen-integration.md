# oapi-codegen Integration Strategy

> How Cliford uses oapi-codegen to generate the SDK layer, and what customizations we need to apply.

## What oapi-codegen Provides

`oapi-codegen` converts OpenAPI 3.0 specifications into Go code. It's the foundation of our SDK generation layer.

### Generation Modes

| Mode | Flag | Output |
|------|------|--------|
| **Types only** | `generate: types` | Go structs from OpenAPI schemas |
| **Client** | `generate: client` | HTTP client functions for each operation |
| **Server (Chi)** | `generate: chi-server` | Chi router interfaces + middleware |
| **Server (Echo)** | `generate: echo-server` | Echo router interfaces + middleware |
| **Server (net/http)** | `generate: std-http-server` | Standard library handlers |
| **Strict Server** | `generate: strict-server` | StrictServerInterface with typed request/response |
| **Embedded Spec** | `generate: spec` | Embedded OpenAPI spec for runtime use |

**For Cliford, we need**: `types` + `client` (and optionally `spec` for runtime validation).

### Configuration (oapi-codegen YAML)

```yaml
# oapi-codegen.yaml
package: sdk
generate:
  models: true
  client: true
  embedded-spec: true
output: internal/sdk/sdk.gen.go
output-options:
  skip-prune: false       # Remove unused types
  nullable-type: pointer  # How to handle nullable fields
```

### Key Features

- **Idiomatic Go** - Generates clean, readable Go code
- **Interface-based** - Server code uses interfaces for testability
- **Import mapping** - Split large specs across packages
- **Type pruning** - Unused types removed by default
- **Strict mode** - Typed request/response objects reduce boilerplate
- **Parameter extraction** - Auto-extracts path, query, header, cookie params
- **Backward compatibility** - `CompatibilityOptions` for upgrade paths

### Limitations

- OpenAPI 3.0 primary (3.1 experimental in separate repo)
- No OpenAPI 2.0 (Swagger) support
- Implicit `additionalProperties` ignored by default
- Single-file output per spec (can be worked around with import mapping)

## Cliford's SDK Generation Pipeline

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  OpenAPI     │────>│  Pre-process │────>│ oapi-codegen │────>│ Post-process │
│  Spec        │     │  (hooks)     │     │  (types +    │     │  (hooks)     │
└──────────────┘     └──────────────┘     │   client)    │     └──────┬───────┘
                                          └──────────────┘            │
                                                                ┌─────▼──────┐
                                                                │ Enhanced   │
                                                                │ SDK Layer  │
                                                                └────────────┘
```

### Step 1: Pre-processing

Before invoking oapi-codegen, Cliford pre-processes the OpenAPI spec:

1. **Extract Cliford extensions** - Read `x-cliford-*` annotations, store in Operation Registry
2. **Validate spec** - Ensure spec is valid OpenAPI 3.0+
3. **Resolve references** - Flatten `$ref` chains for codegen
4. **Apply transforms** - Run `transform:operation` hooks
5. **Strip extensions** - Remove `x-cliford-*` from spec copy (clean input for oapi-codegen)

### Step 2: oapi-codegen Invocation

Cliford invokes oapi-codegen programmatically (as a Go library, not CLI):

```go
import "github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"

func generateSDK(spec *openapi3.T, config *SDKConfig) error {
    opts := codegen.Configuration{
        PackageName: config.Package,
        Generate: codegen.GenerateOptions{
            Models:       true,
            Client:       true,
            EmbeddedSpec: true,
        },
        OutputOptions: codegen.OutputOptions{
            SkipPrune: false,
        },
    }
    
    code, err := codegen.Generate(spec, opts)
    if err != nil {
        return err
    }
    
    return os.WriteFile(config.OutputPath, []byte(code), 0644)
}
```

Using oapi-codegen as a library gives us:
- Programmatic control over generation
- Ability to intercept and modify output
- No external binary dependency
- Access to internal types for our post-processing

### Step 3: Post-processing (SDK Enhancement)

After oapi-codegen generates the base SDK, Cliford enhances it:

1. **Retry wrapper** - Wrap client methods with configurable retry logic
2. **Pagination helpers** - Add `ListAll()` variants for paginated endpoints
3. **Auth injection** - Add auth middleware to the HTTP client
4. **Error enhancement** - Rich error types with status codes, response bodies, request IDs
5. **Context propagation** - Ensure all methods accept `context.Context`
6. **Timeout support** - Per-operation timeout configuration
7. **Telemetry hooks** - Optional request/response logging points

### Generated SDK Structure

```
internal/sdk/
  sdk.gen.go           # oapi-codegen output (types + client)
  client_enhanced.go   # Cliford enhancements (retry, auth, etc.)
  pagination.go        # Pagination helpers
  errors.go            # Enhanced error types
  middleware.go        # HTTP middleware chain
  config.go           # SDK configuration
```

The `*.gen.go` files are fully regenerated on each run. The enhancement files use the generated types and are also regenerated, but respect custom code regions.

## Custom Code Regions in SDK Layer

```go
// client_enhanced.go

func (c *Client) ListUsers(ctx context.Context, params *ListUsersParams) (*ListUsersResponse, error) {
    // GENERATED CODE - DO NOT EDIT ABOVE THIS LINE
    
    // --- CUSTOM CODE START: ListUsers:before ---
    // Developer can add custom pre-request logic here
    // --- CUSTOM CODE END: ListUsers:before ---
    
    resp, err := c.sdk.ListUsers(ctx, params)
    
    // --- CUSTOM CODE START: ListUsers:after ---
    // Developer can add custom post-request logic here
    // --- CUSTOM CODE END: ListUsers:after ---
    
    return resp, err
}
```

On regeneration, Cliford preserves everything between `CUSTOM CODE START` and `CUSTOM CODE END` markers.

## SDK-to-CLI/TUI Bridge

The SDK is the foundation for both CLI and TUI layers. The Operation Registry bridges them:

```go
type OperationMeta struct {
    ID          string
    Method      string
    Path        string
    Summary     string
    Description string
    Tags        []string
    Parameters  []ParamMeta
    RequestBody *BodyMeta
    Response    *ResponseMeta
    Security    []SecurityRequirement
    
    // CLI-specific
    CLIAliases  []string
    CLIGroup    string
    CLIHidden   bool
    CLIConfirm  bool  // Confirm before execute
    
    // TUI-specific
    TUIDisplay  DisplayMode  // table, detail, form, custom
    TUIRefresh  bool         // Auto-refresh capability
    
    // Runtime
    Pagination  *PaginationConfig
    Retries     *RetryConfig
    Timeout     *time.Duration
    
    // SDK function reference
    SDKFunc     interface{}  // Typed function pointer
}
```

The CLI adapter reads `OperationMeta` to generate Cobra commands. The TUI adapter reads it to generate Bubbletea models. Both call the same SDK functions.

## oapi-codegen Version Strategy

- Pin to a specific release tag in `go.mod`
- Track oapi-codegen releases for compatibility
- Provide `CompatibilityOptions` passthrough for users who need backward compat
- Test against a matrix of oapi-codegen versions in CI

## Alternative SDK Generators (Future)

While oapi-codegen is the initial choice, the architecture should allow swapping generators:

- **ogen** (`github.com/ogen-go/ogen`) - Alternative Go codegen, different tradeoffs
- **openapi-generator** - Multi-language, Java-based, heavier
- **Custom** - Users could provide their own SDK via the hook system

The `sdk.generator` config field allows for future expansion:

```yaml
generation:
  sdk:
    generator: oapi-codegen  # oapi-codegen | ogen | custom
```
