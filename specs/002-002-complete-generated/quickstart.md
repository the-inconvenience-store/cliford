# Quickstart: Complete Generated App Wiring

**Branch**: `002-002-complete-generated` | **Date**: 2026-04-11

---

## What This Feature Delivers

After this feature is implemented, running `cliford generate` on any OpenAPI spec produces a fully wired Go app that:

- Authenticates against the API automatically (keychain → env → config fallback chain)
- Retries on transient failures with exponential backoff
- Enforces request timeouts
- Displays list results as formatted tables
- Prompts for missing required flags interactively
- Confirms destructive operations before executing
- Supports `--verbose` for request/response tracing
- Fetches all pages with `--all` on paginated endpoints
- Allows before/after request hooks via shell or go-plugin

---

## Developer Workflow (Cliford Tool)

```bash
# Generate a fully wired CLI from an OpenAPI spec
cliford generate --spec petstore.yaml --project petstore --output ./out/

# Verify the generated app builds and tests pass immediately
cd out/
go build ./...
go test ./...
./petstore --help
```

---

## End-User Workflow (Generated App)

### Initial Setup

```bash
# Log in — stores credentials in OS keychain
petstore auth bearerauth login --token <your-token>

# Or via environment variable
export PETSTORE_BEARERAUTH_TOKEN=<your-token>

# Or via config file (last resort)
echo "auth:\n  bearerauth:\n    token: <your-token>" >> ~/.petstore.yaml
```

### Listing Resources

```bash
# Default: formatted table
petstore list-pets

# All pages
petstore list-pets --all

# Specific fields
petstore list-pets --fields id,name,status

# Raw JSON
petstore list-pets --output json
```

### Creating Resources (with interactive prompts)

```bash
# Omit required flags → prompts appear
petstore create-pet
# name: Fluffy
# status: available

# Or supply all flags
petstore create-pet --name Fluffy --status available
```

### Destructive Operations

```bash
# Prompts for confirmation by default
petstore delete-pet --id 123
# Delete pet 123? [y/N] y

# Skip prompt with --yes
petstore delete-pet --id 123 --yes
```

### Verbose Debugging

```bash
petstore list-pets --verbose
# → stderr:
# > GET https://api.petstore.com/v3/pets
# > Authorization: [REDACTED]
# < 200 OK
# < Content-Type: application/json
```

### Hooks

```yaml
# ~/.petstore.yaml
features:
  hooks:
    enabled: true
    before_request:
      - type: shell
        command: "scripts/audit-log.sh"
```

---

## Key Files in Generated Output

```
<project>/
├── cmd/<project>/main.go          # Entry point; reads Viper config, wires http.Client
├── internal/auth/
│   └── resolver.go                # 5-tier credential resolution
├── internal/client/
│   └── factory.go                 # Layered http.Client (auth → retry → transport)
├── internal/output/
│   └── table.go                   # text/tabwriter renderer
├── internal/hooks/
│   └── runner.go                  # Shell and go-plugin hook execution
└── docs/                          # Man pages + Markdown docs (auto-generated)
```
