# OpenAPI Overlays

Cliford supports the [OAI Overlay Specification](https://github.com/OAI/Overlay-Specification)
(v1.0 and v1.1). An overlay is a separate YAML file that patches an OpenAPI
spec without modifying the original file on disk.

Overlays are the standard solution when you do not own or control the spec —
for example, a third-party vendor spec, a machine-generated spec that is
re-synced from a schema registry, or a spec shared across multiple tools that
cannot each carry their own extensions.

## When to use overlays

| Situation | Use overlay |
|-----------|------------|
| Add `x-cliford-*` extensions to an unmodifiable spec | Yes |
| Remove internal or beta endpoints from the generated CLI | Yes |
| Patch server URLs or descriptions for a specific environment | Yes |
| The spec is yours and you can edit it directly | Prefer editing the spec |

## Quick start

Create `cliford.overlay.yaml` alongside `cliford.yaml`:

```yaml
overlay: "1.0.0"
info:
  title: "Cliford extensions for Stripe API"
  version: "0.1.0"
actions:
  # Add CLI config to the list charges endpoint
  - target: "$.paths['/v1/charges'].get"
    update:
      x-cliford-cli:
        defaultJQ: ".data"
        aliases: ["ls", "list"]
        agentFormat: toon

  # Require confirmation on all DELETE operations
  - target: "$.paths.*.delete"
    update:
      x-cliford-cli:
        confirm: true

  # Remove an internal endpoint from the generated CLI
  - target: "$.paths['/internal/health']"
    remove: true
```

Reference it in `cliford.yaml`:

```yaml
overlays:
  - cliford.overlay.yaml
```

Run generation as usual:

```bash
cliford generate
```

The overlay is applied before all other pipeline stages. Both SDK and CLI
generation see the patched spec. No files in your project directory are
modified.

## Overlay file format

Cliford implements the OAI Overlay Specification as defined at
[github.com/OAI/Overlay-Specification](https://github.com/OAI/Overlay-Specification).

```yaml
overlay: "1.0.0"          # Required. "1.0.0" or "1.1.0"
info:
  title: "My overlay"     # Required
  version: "0.1.0"        # Required
actions:
  - target: "<JSONPath>"  # Required per action
    description: "..."    # Optional
    update: <yaml>        # Merge this value into matched nodes
    remove: true          # Remove matched nodes (mutually exclusive with update)
```

### JSONPath targets

`target` is a JSONPath expression that selects nodes in the OpenAPI document.
Common patterns:

| Target | Selects |
|--------|---------|
| `$.paths['/pets'].get` | The GET /pets operation |
| `$.paths.*.delete` | All DELETE operations |
| `$.paths['/pets'].get.parameters[?(@.name=='limit')]` | A specific parameter |
| `$.components.schemas.Pet` | A schema component |
| `$.info` | The info object |

### Merge semantics

| Node type | Behaviour |
|-----------|-----------|
| Primitive (string, number, bool) | Replacement |
| Object / mapping | Recursive deep merge; existing keys not in `update` are preserved |
| Array / sequence | Concatenation: `target + update` |

To replace an array entirely, use `remove: true` on the target first, then add
a second action with `update` containing the replacement.

### `remove: true`

When `remove` is set, the matched node is deleted from its parent. Useful for
hiding endpoints, parameters, or schema fields:

```yaml
actions:
  # Remove a path entirely
  - target: "$.paths['/internal/metrics']"
    remove: true

  # Remove a specific parameter
  - target: "$.paths['/pets'].get.parameters[?(@.name=='x-debug')]"
    remove: true
```

## Configuring overlays

### In cliford.yaml

```yaml
overlays:
  - cliford.overlay.yaml
  - overlays/local.yaml
```

Overlays are applied in listed order. Each action sees the cumulative result
of all previous actions.

### Via --overlay flag

The `--overlay` flag accepts a path and can be repeated:

```bash
cliford generate --overlay cliford.overlay.yaml --overlay overlays/ci.yaml
```

When `--overlay` is provided on the command line, it takes full priority over
the `overlays` list in `cliford.yaml`.

### With cliford validate

Overlays are also applied when running validation, so you can verify the
merged spec before committing:

```bash
cliford validate --spec openapi.yaml --overlay cliford.overlay.yaml
```

## Recommended file layout

| File | Purpose |
|------|---------|
| `cliford.overlay.yaml` | Primary overlay, committed alongside `cliford.yaml` |
| `cliford.overlay.local.yaml` | Developer-only overlay, added to `.gitignore` |
| `overlays/<env>.yaml` | Environment-specific overlays (staging, prod) for CI/CD |

## Precedence

Overlays are applied before all configuration merging. The full resolution
order is:

```
cliford.yaml operation-level overrides  (highest)
  > x-cliford-* extensions (from overlay or original spec)
  > cliford.yaml global settings
  > built-in defaults                   (lowest)
```

Because overlays run first, any `x-cliford-*` extension they add can still be
overridden by a `cliford.yaml` operation-level override.

## Example: adding pagination to a third-party spec

```yaml
overlay: "1.0.0"
info:
  title: "Pagination extensions for Acme API"
  version: "0.1.0"
actions:
  - target: "$.paths['/v1/items'].get"
    update:
      x-cliford-pagination:
        type: cursor
        input:
          cursor: {name: page_token, in: query}
          limit:  {name: page_size, in: query, default: 20}
        output:
          results:    "$.items"
          nextCursor: "$.next_page_token"
          totalCount: "$.total_count"

  - target: "$.paths['/v1/orders'].get"
    update:
      x-cliford-pagination:
        type: offset
        input:
          limit:  {name: limit, in: query, default: 50}
          offset: {name: offset, in: query}
        output:
          results:    "$.orders"
          totalCount: "$.total"
```
