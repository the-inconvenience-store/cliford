# Authentication

Generated apps support all standard OpenAPI security schemes with secure
credential storage and multi-profile management.

## Supported Methods

| Method | OpenAPI Type | Automatic |
|--------|-------------|-----------|
| API Key (header/query) | `apiKey` | Yes |
| HTTP Basic | `http` (basic) | Yes |
| Bearer Token | `http` (bearer) | Yes |
| OAuth 2.0 | `oauth2` | Yes (AuthCode, ClientCreds, DeviceCode) |
| OpenID Connect | `openIdConnect` | Via hooks |
| mTLS | Custom | Via hooks |

Methods are auto-detected from your OpenAPI spec's `securitySchemes`.

## Auth Commands

```bash
# Login interactively
myapp auth login

# Login with specific method
myapp auth login --method bearer --token "sk-..."
myapp auth login --method apiKey --api-key "key-..."
myapp auth login --method basic --username admin --password secret
myapp auth login --method oauth2 --token "access-token-..."

# Non-interactive login (for CI/scripts)
myapp auth login --method bearer --token "$API_TOKEN" -y

# Check auth state (secrets are redacted)
myapp auth status
# Profile: default
# Method:  bearer
# Token:   sk-t...cdef

# Logout
myapp auth logout

# Force refresh OAuth token
myapp auth refresh
```

## Credential Storage

Credentials are stored using a priority chain:

1. **OS Keychain** (preferred) — macOS Keychain, Windows Credential Manager,
   Linux Secret Service
2. **Encrypted file** (fallback) — AES-256-GCM encrypted, stored in
   `~/.config/<app>/auth/`. Used in containers and CI where no keychain exists.
   A warning is printed when this fallback is used.
3. **Environment variables** (runtime only) — `<PREFIX>_API_KEY`,
   `<PREFIX>_BEARER_TOKEN`, etc.
4. **CLI flags** (per-command) — `--token`, `--api-key`, etc.

Credential resolution at request time:
**Flags > Environment > Keychain/File > Config**

## Profiles

Manage multiple environments (production, staging, local) with profiles:

```bash
# Login to the staging profile
myapp auth login --profile staging --token "staging-token"

# Switch active profile
myapp auth switch staging

# Check which profile is active
myapp auth status

# Configure server URL per profile
myapp config set profiles.staging.server.url https://staging.api.example.com
```

Each profile has its own:
- Server URL
- Auth method
- Stored credentials

## Per-Operation Security

OpenAPI specs can define different security per operation:

```yaml
paths:
  /public/health:
    get:
      security: []         # No auth required
  /admin/users:
    get:
      security:
        - bearerAuth: []   # Requires bearer token
```

Cliford respects these: operations with `security: []` won't prompt for or
attach credentials.

## Security Practices

- Credentials are **never logged** — all debug output, dry-run output, and
  error messages automatically redact Authorization headers, API keys, and
  tokens.
- **HTTPS warnings** — sending credentials over HTTP triggers a warning
  (except for `localhost`).
- Token display is always truncated: `sk-t...cdef`
- The `auth status` command shows auth state without revealing full secrets.

## Environment Variables

For CI/CD and scripting, set credentials via environment variables:

```bash
export MYAPP_BEARER_TOKEN="sk-..."
export MYAPP_API_KEY="key-..."
export MYAPP_SERVER_URL="https://api.example.com"

myapp pets list  # Credentials attached automatically
```
