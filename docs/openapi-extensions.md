# OpenAPI Extensions Reference

Cliford supports `x-cliford-*` extensions in your OpenAPI spec for
per-operation configuration. These are secondary to `cliford.yaml`. If both
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
        group: pets
        hidden: false
        confirm: false
        confirmMessage: ""
        defaultJQ: ".pets"
        agentFormat: toon
        defaultOutputFormat: table
        requestId: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `aliases` | `[]string` | `[]` | Alternative command names |
| `group` | `string` | (from tag) | Override command group |
| `hidden` | `bool` | `false` | Hide from help output |
| `confirm` | `bool` | `false` | Prompt for confirmation before executing |
| `confirmMessage` | `string` | auto-generated | Custom confirmation text |
| `defaultJQ` | `string` | `""` | Default jq expression applied to output; overridable with `--jq` |
| `agentFormat` | `string` | `""` | Output format override when `--agent` is active (e.g. `toon`, `json`); overrides global `features.agentOutputFormat` |
| `defaultOutputFormat` | `string` | `""` | Default `--output-format` for this operation (e.g. `table`); user can override with `--output-format` at runtime |
| `requestId` | `bool` | `false` | Enable request ID injection for this operation; generates a UUID, attaches it as a header, and embeds it in error messages |

When `confirm` is `true` or the operation is a DELETE, the generated command
displays a `[y/N]` prompt before sending the request. The `--yes` flag skips
the prompt.

When `defaultJQ` is set, the generated command always applies the jq
expression to its response — the user does not need to pass `--jq`. The user
can still override it by passing `--jq` with a different expression. Cliford
validates the expression at generation time and fails immediately if it cannot
be parsed.

When `defaultOutputFormat` is set, the generated command uses that format
when the user has not explicitly passed `--output-format`. The precedence is:

1. `cliford.yaml` operation-level `defaultOutputFormat` (highest)
2. `x-cliford-cli.defaultOutputFormat`
3. `--output-format` global default (from `generation.cli.flags.outputFormat.default` or `"pretty"`)

`--agent` mode still takes priority: when `--agent` is active and
`--output-format` has not been explicitly changed, the agent format wins over
`defaultOutputFormat`.

## x-cliford-tui

TUI-specific per-operation configuration.

```yaml
paths:
  /pets:
    get:
      x-cliford-tui:
        displayAs: table
        refreshable: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `displayAs` | `string` | (auto) | `table`, `detail`, `form`, `custom` |
| `refreshable` | `bool` | `false` | Enable TUI auto-refresh |

Display mode auto-detection:
- GET returning array: `table`
- GET returning object: `detail`
- POST/PUT/PATCH: `form`
- DELETE: `detail`

## x-cliford-display

Per-property extension on response schema fields that controls default table
columns.

```yaml
components:
  schemas:
    Pet:
      type: object
      properties:
        id:
          type: integer
          x-cliford-display: true
        name:
          type: string
          x-cliford-display: true
        internal_notes:
          type: string
          # No x-cliford-display, hidden from table by default
```

When any property in a response schema has `x-cliford-display: true`, only
those properties appear as table columns by default. If no properties have
this extension, all properties are shown.

The `--fields` CLI flag overrides this behavior:

```bash
./myapp pets list --fields id,name,status
```

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
| `input.<param>.name` | `string` | Query or body parameter name |
| `input.<param>.in` | `string` | `query` or `body` |
| `input.<param>.default` | `int` | Default value |
| `output.results` | `string` | JSONPath to results array |
| `output.nextCursor` | `string` | JSONPath to next cursor, page, or URL |
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
  > built-in defaults           (lowest)
```

Extensions are ignored by tools that do not understand them, so your OpenAPI
spec remains compatible with other consumers.
