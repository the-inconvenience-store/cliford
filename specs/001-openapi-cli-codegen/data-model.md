# Data Model: OpenAPI CLI & TUI Code Generation

**Date**: 2026-04-10

## Core Entities

### OpenAPISpec

The parsed representation of the input OpenAPI specification.

**Fields**:
- `Title` (string): API title from `info.title`
- `Version` (string): API version from `info.version`
- `Description` (string): API description from `info.description`
- `Servers` ([]ServerConfig): Available API servers
- `Operations` ([]Operation): All parsed operations
- `SecuritySchemes` (map): Authentication methods from `components.securitySchemes`
- `GlobalParameters` ([]Parameter): Parameters defined at spec root
- `Extensions` (map): Extracted `x-cliford-*` annotations

**Relationships**: Contains many Operations, SecuritySchemes, and Servers.

---

### Operation

A single API endpoint (method + path) that maps to one CLI command and one
TUI form.

**Fields**:
- `OperationID` (string): Unique identifier from spec
- `Method` (string): HTTP method (GET, POST, PUT, DELETE, PATCH)
- `Path` (string): URL path with parameters (e.g., `/users/{id}`)
- `Summary` (string): Short description for help text
- `Description` (string): Long description for documentation
- `Tags` ([]string): Grouping tags (map to command groups)
- `Parameters` ([]Parameter): Path, query, header, cookie params
- `RequestBody` (RequestBody, optional): Body schema for POST/PUT/PATCH
- `Responses` (map): Response schemas keyed by status code
- `Security` ([]SecurityRequirement): Auth requirements for this operation
- `Deprecated` (bool): Whether the operation is deprecated

**Relationships**: Belongs to tag group(s). Has many Parameters. May have
a RequestBody. References SecuritySchemes.

---

### Parameter

An input to an operation (path, query, header, or cookie parameter).

**Fields**:
- `Name` (string): Parameter name as it appears in the spec
- `In` (string): Location - path, query, header, cookie
- `Description` (string): Help text for flag
- `Required` (bool): Whether the parameter is required
- `Schema` (Schema): Type information (string, integer, boolean, array, etc.)
- `Default` (any, optional): Default value if not provided
- `Enum` ([]any, optional): Allowed values
- `FlagName` (string, derived): CLI flag name (kebab-case of Name)

**Validation rules**:
- Path parameters are always required
- Flag names must not collide within a command (disambiguate if needed)
- Enum values generate flag completion hints

---

### RequestBody

The body payload for POST/PUT/PATCH operations.

**Fields**:
- `ContentType` (string): Media type (application/json, multipart/form-data, etc.)
- `Schema` (Schema): Body structure definition
- `Required` (bool): Whether body is required
- `Description` (string): Help text

**Relationships**: Belongs to an Operation. Contains a Schema.

---

### SecurityScheme

An authentication method declared in the OpenAPI spec.

**Fields**:
- `Name` (string): Scheme identifier
- `Type` (string): apiKey, http, oauth2, openIdConnect
- `In` (string, for apiKey): header, query, cookie
- `Scheme` (string, for http): basic, bearer
- `BearerFormat` (string, optional): JWT, opaque, etc.
- `Flows` (OAuthFlows, for oauth2): Authorization code, client credentials, etc.
- `OpenIDConnectURL` (string, for OIDC): Discovery endpoint

**Relationships**: Referenced by Operations via SecurityRequirements.

---

### OperationMeta

The enriched metadata for an operation, combining spec data with Cliford
configuration. This is the central type in the Operation Registry.

**Fields**:
- All fields from Operation (embedded)
- `CLIAliases` ([]string): Command aliases from config
- `CLIGroup` (string): Override tag-based grouping
- `CLIHidden` (bool): Hide from help output
- `CLIConfirm` (bool): Require confirmation before execution
- `CLIConfirmMessage` (string): Custom confirmation text
- `TUIDisplay` (DisplayMode): table, detail, form, custom
- `TUIRefreshable` (bool): Auto-refresh capability in TUI
- `PaginationConfig` (PaginationConfig, optional): How to paginate
- `RetryConfig` (RetryConfig, optional): Per-operation retry overrides
- `Timeout` (duration, optional): Per-operation timeout
- `SDKMethodName` (string): Name of the generated SDK method

**Relationships**: Built from Operation + ClifordConfig + OpenAPI extensions.

---

### ClifordConfig

The developer's configuration for the code generation tool.

**Fields**:
- `Version` (string): Config schema version
- `SpecPath` (string): Path to OpenAPI spec file
- `App.Name` (string): Generated binary name
- `App.Package` (string): Go module path
- `App.EnvVarPrefix` (string): Environment variable prefix
- `App.Version` (string): App semantic version
- `App.Description` (string): App description
- `Generation.Mode` (enum): pure-cli, pure-tui, hybrid
- `Generation.SDK.Generator` (string): oapi-codegen (default)
- `Generation.SDK.OutputDir` (string): SDK output directory
- `Generation.CLI.OutputDir` (string): CLI output directory
- `Generation.TUI.Enabled` (bool): Whether to generate TUI code
- `Generation.TUI.OutputDir` (string): TUI output directory
- `Auth.Interactive` (bool): Generate interactive auth commands
- `Auth.Keychain` (bool): Enable OS keychain storage
- `Auth.Methods` ([]string): Enabled auth methods
- `Theme` (ThemeConfig): TUI theme configuration
- `Features.Pagination` (bool): Enable pagination support
- `Features.Retries` (RetryConfig): Retry configuration
- `Features.CustomCodeRegions` (bool): Enable custom code markers
- `Features.Documentation.Markdown` (bool): Generate Markdown docs
- `Features.Documentation.LLMsTxt` (bool): Generate llms.txt
- `Features.Distribution.GoReleaser` (bool): Generate GoReleaser config
- `Operations` (map): Per-operation overrides keyed by operationID
- `Hooks` (map): Hook definitions keyed by hook point
- `GlobalParams` ([]GlobalParam): Global parameters for all requests

**Validation rules**:
- `App.Name` must be a valid Go identifier and shell-safe
- `App.Package` must be a valid Go module path
- `App.EnvVarPrefix` must be uppercase, underscored
- `Generation.Mode` must be one of the enum values

---

### Profile

A named environment configuration for the generated app's end user.

**Fields**:
- `Name` (string): Profile identifier (e.g., "default", "staging", "prod")
- `ServerURL` (string): API server base URL
- `AuthMethod` (string): Which security scheme to use
- `IsActive` (bool): Whether this is the currently selected profile

**Relationships**: Each profile has its own credential set stored in the
credential backend. One profile is active at a time.

---

### ThemeConfig

Visual customization for the generated app's TUI.

**Fields**:
- `Colors.Primary` (color): Primary brand color
- `Colors.Secondary` (color): Secondary color
- `Colors.Accent` (color): Accent/highlight color
- `Colors.Background` (color): Background color
- `Colors.Text` (color): Default text color
- `Colors.Dimmed` (color): Muted/secondary text
- `Colors.Error` (color): Error state color
- `Colors.Success` (color): Success state color
- `Colors.Warning` (color): Warning state color
- `Borders` (enum): normal, rounded, thick, double
- `Spinner` (string): Spinner animation style
- `Table.HeaderBold` (bool): Bold table headers
- `Table.StripeRows` (bool): Alternating row colors
- `Compact` (bool): Reduce padding and margins

---

### PaginationConfig

Pagination behavior for a specific operation.

**Fields**:
- `Type` (enum): offset, page, cursor, link, url
- `Input.CursorParam` (string): Name of cursor/page/offset parameter
- `Input.CursorIn` (string): Parameter location (query, body)
- `Input.LimitParam` (string): Name of limit/per_page parameter
- `Input.LimitDefault` (int): Default page size
- `Output.ResultsPath` (string): JSONPath to results array in response
- `Output.NextCursorPath` (string): JSONPath to next cursor/page/URL
- `Output.TotalCountPath` (string, optional): JSONPath to total count

---

### RetryConfig

Retry behavior configuration.

**Fields**:
- `Enabled` (bool): Whether retries are active
- `Strategy` (string): "exponential-backoff" (only option currently)
- `InitialInterval` (duration): First retry delay (default: 500ms)
- `MaxInterval` (duration): Maximum retry delay (default: 60s)
- `MaxElapsedTime` (duration): Total retry window (default: 5m)
- `Exponent` (float): Backoff multiplier (default: 1.5)
- `Jitter` (bool): Add randomness to delays (default: true)
- `StatusCodes` ([]int): HTTP codes that trigger retry
- `RetryConnectionErrors` (bool): Retry DNS/connection failures

---

## State Transitions

### Generation Pipeline States

```
INIT -> PARSING -> VALIDATED -> SDK_GENERATING -> SDK_COMPLETE
  -> CLI_GENERATING -> CLI_COMPLETE -> TUI_GENERATING -> TUI_COMPLETE
  -> INFRA_GENERATING -> COMPLETE
```

Hooks fire at each state transition (before/after).
Failure at any stage halts the pipeline with a diagnostic error.

### Credential Lifecycle

```
NONE -> STORED (via login) -> ACTIVE (auto-attached to requests)
  -> EXPIRED (token expiry detected) -> REFRESHED (auto-refresh)
  -> REVOKED (via logout) -> NONE
```

OAuth tokens: proactive refresh before expiry.
All other types: stored until explicitly removed.
