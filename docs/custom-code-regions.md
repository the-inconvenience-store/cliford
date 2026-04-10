# Custom Code Regions

Custom code regions let you add hand-written logic to generated files that
survives regeneration. This is essential for adding telemetry, custom
validation, response transforms, or any logic not derivable from the OpenAPI
spec.

## Enabling

```bash
cliford generate --custom-regions
```

Or in `cliford.yaml`:

```yaml
features:
  customCodeRegions: true
```

## How It Works

When enabled, Cliford inserts marked regions in generated files:

```go
func petsListCmd() *cobra.Command {
    // ...

    RunE: func(cmd *cobra.Command, args []string) error {
        // --- CUSTOM CODE START: listPets:pre ---
        // --- CUSTOM CODE END: listPets:pre ---

        // ... generated request logic ...

        // --- CUSTOM CODE START: listPets:post ---
        // --- CUSTOM CODE END: listPets:post ---

        return FormatOutput(data, outputFormat)
    },
}
```

Add your code between the markers:

```go
        // --- CUSTOM CODE START: listPets:pre ---
        ctx = tracing.InjectSpan(ctx)
        log.Printf("listing pets with limit=%d", flagLimit)
        // --- CUSTOM CODE END: listPets:pre ---
```

When you regenerate, Cliford extracts your custom code, generates fresh
output, and reinjects your code at the same markers.

## Region Locations

| Location | Region Name | Purpose |
|----------|-------------|---------|
| Root command setup | `root:init` | Global initialization, middleware |
| Before each operation | `<operationId>:pre` | Pre-request logic (logging, tracing, transforms) |
| After each operation | `<operationId>:post` | Post-response logic (metrics, caching) |

## Safety Mechanisms

### Preview Before Regenerating

```bash
cliford diff --spec openapi.yaml --output-dir .
```

Shows what would change, confirming custom regions are safe.

### Automatic Backup

Before every regeneration, Cliford copies the current output to
`.cliford/backup/<timestamp>/`. The last 5 backups are kept.

### Orphaned Region Warnings

If you regenerate after removing an operation from the spec, Cliford warns
about custom code regions that no longer have a home:

```
Warning: orphaned custom code regions:
  ! internal/cli/pets.go:deletePet:pre
  ! internal/cli/pets.go:deletePet:post
```

The orphaned code is preserved in the backup.

### Lockfile

After each generation, `cliford.lock` is written with:
- Spec file SHA256 hash
- Config hash
- Timestamp
- SHA256 checksums of all generated files

This enables `cliford diff` to detect changes and `cliford version auto` to
determine the appropriate SemVer bump.

## Tips

- Keep custom code **small and focused** — if your custom region grows large,
  consider extracting it into a separate package and calling it from the region.
- Use `<operationId>:pre` for request mutation (headers, params, tracing).
- Use `<operationId>:post` for response processing (logging, metrics, caching).
- Use `root:init` for one-time global setup (middleware registration, config
  loading).
