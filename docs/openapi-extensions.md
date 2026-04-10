# OpenAPI Extensions Reference

Cliford supports `x-cliford-*` extensions in your OpenAPI spec for
per-operation configuration. These are secondary to `cliford.yaml` — if both
define a setting, `cliford.yaml` takes precedence.

## x-cliford-cli

CLI-specific per-operation configuration.

```yaml
paths:
  /pets:
    get:
      operationId: listPets
      x-cliford-cli:
        aliases:
          - ls
          - list
        group: pets          # Override tag-based grouping
        hidden: false        # Hide from --help (still callable)
        confirm: false       # Require confirmation before executing
        confirmMessage: ""   # Custom confirmation prompt
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `aliases` | `[]string` | `[]` | Alternative command names |
| `group` | `string` | (from tag) | Override command group |
| `hidden` | `bool` | `false` | Hide from help output |
| `confirm` | `bool` | `false` | Prompt for confirmation |
| `confirmMessage` | `string` | `""` | Custom confirmation text |

## x-cliford-tui

TUI-specific per-operation configuration.

```yaml
paths:
  /pets:
    get:
      x-cliford-tui:
        displayAs: table     # How to render the response
        refreshable: true    # Enable auto-refresh in TUI
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `displayAs` | `string` | (auto) | `table`, `detail`, `form`, `custom` |
| `refreshable` | `bool` | `false` | Enable TUI auto-refresh |

Display mode auto-detection:
- GET returning array -> `table`
- GET returning object -> `detail`
- POST/PUT/PATCH -> `form`
- DELETE -> `detail`

## x-cliford-pagination

Pagination configuration for endpoints that return paged results.

```yaml
paths:
  /items:
    get:
      x-cliford-pagination:
        type: cursor
        input:
          cursor:
            name: cursor
            in: query
          limit:
            name: limit
            in: query
            default: 20
        output:
          results: "$.data"
          nextCursor: "$.meta.nextCursor"
          totalCount: "$.meta.total"
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | `string` | `offset`, `page`, `cursor`, `link`, `url` |
| `input.<param>.name` | `string` | Query/body parameter name |
| `input.<param>.in` | `string` | `query` or `body` |
| `input.<param>.default` | `int` | Default value |
| `output.results` | `string` | JSONPath to results array |
| `output.nextCursor` | `string` | JSONPath to next cursor/page/URL |
| `output.totalCount` | `string` | JSONPath to total count (optional) |

## x-cliford-retries

Per-operation retry configuration.

```yaml
paths:
  /webhooks:
    post:
      x-cliford-retries:
        enabled: true
        maxAttempts: 5
        statusCodes:
          - 429
          - 503
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `true` | Enable retries |
| `maxAttempts` | `int` | `3` | Max retry attempts |
| `statusCodes` | `[]int` | `[408,429,500,502,503,504]` | HTTP codes that trigger retry |

## Precedence

When both `cliford.yaml` and OpenAPI extensions define the same setting:

```
cliford.yaml operation-level  (highest)
  > x-cliford-* extension
  > cliford.yaml global defaults
  > built-in defaults         (lowest)
```

Extensions are ignored by tools that don't understand them, so your OpenAPI
spec remains compatible with other consumers.
