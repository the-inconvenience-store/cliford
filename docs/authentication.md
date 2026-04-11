# How to Set Up Authentication

This guide covers configuring authentication for generated CLI applications,
including credential storage, environment variables, OAuth2, and multi-profile
management.

## Supported methods

| Method | OpenAPI Type | Header Format |
|--------|-------------|---------------|
| API Key (header or query) | `apiKey` | Custom header or query parameter |
| HTTP Basic | `http` (scheme: basic) | `Authorization: Basic <base64>` |
| Bearer Token | `http` (scheme: bearer) | `Authorization: Bearer <token>` |
| OAuth 2.0 Client Credentials | `oauth2` (clientCredentials) | `Authorization: Bearer <token>` |

Methods are detected automatically from your OpenAPI spec's `securitySchemes`.

## Credential resolution order

When a request is made, credentials are resolved through a 5-tier chain.
The first tier that provides a value wins:

1. **CLI flags** (`--token`, `--api-key`)
2. **Environment variables** (`<APP>_<SCHEME>_<TYPE>`)
3. **OS keychain** (macOS Keychain, Linux Secret Service, Windows Credential Manager)
4. **Encrypted file** (AES-256-GCM, stored in `~/.config/<app>/auth/`)
5. **Config file** (YAML via Viper, `auth.<scheme>.token`)

If no credentials are found for a required scheme, the app prints an error
with the scheme name, type, and the exact environment variable to set:

```
Error: authentication required
  Scheme: BearerAuth (bearer)
  Configure via: PETSTORE_BEARERAUTH_TOKEN env var
              or: petstore auth login
```

## Environment variable naming

Environment variable names follow the pattern `<APP>_<SCHEME>_<TYPE>`:

| Scheme Type | Scheme Name | Environment Variable |
|-------------|-------------|---------------------|
| Bearer token | `BearerAuth` | `PETSTORE_BEARERAUTH_TOKEN` |
| API key | `ApiKeyAuth` | `PETSTORE_APIKEYAUTH_API_KEY` |
| Basic auth | `BasicAuth` | `PETSTORE_BASICAUTH_USERNAME`, `PETSTORE_BASICAUTH_PASSWORD` |
| OAuth2 | `OAuth2` | `PETSTORE_OAUTH2_TOKEN` |

The app prefix is derived from the `envVarPrefix` in `cliford.yaml` (or the
app name, uppercased). The scheme name comes from the `securitySchemes` key
in your OpenAPI spec, uppercased with special characters replaced by
underscores.

## How to authenticate with environment variables

```bash
# Bearer token
export PETSTORE_BEARERAUTH_TOKEN="sk-your-token-here"
./petstore pets list

# API key
export PETSTORE_APIKEYAUTH_API_KEY="key-abc123"
./petstore pets list

# Basic auth
export PETSTORE_BASICAUTH_USERNAME="admin"
export PETSTORE_BASICAUTH_PASSWORD="secret"
./petstore pets list
```

## How to authenticate interactively

```bash
# Interactive login (prompts for values)
./petstore auth login

# Login with a specific method
./petstore auth login --method bearer --token "sk-..."
./petstore auth login --method apiKey --api-key "key-..."
./petstore auth login --method basic --username admin --password secret

# Non-interactive login (for CI/scripts)
./petstore auth login --method bearer --token "$API_TOKEN" -y

# Check current auth state (secrets are truncated)
./petstore auth status

# Clear stored credentials
./petstore auth logout
```

Credentials stored via `auth login` are saved to the OS keychain when
available, falling back to an encrypted file. A warning is printed when the
encrypted file fallback is used.

## How to set up OAuth 2.0 client credentials

For APIs using OAuth 2.0 client credentials, set three environment variables:

```bash
export PETSTORE_OAUTH2_CLIENT_ID="your-client-id"
export PETSTORE_OAUTH2_CLIENT_SECRET="your-client-secret"
export PETSTORE_OAUTH2_TOKEN_URL="https://auth.example.com/oauth/token"
```

The app automatically exchanges the client ID and secret for an access token,
caches it in memory, and refreshes it 60 seconds before expiry. No manual
token management is needed.

If you already have an access token, set it directly:

```bash
export PETSTORE_OAUTH2_TOKEN="your-access-token"
```

## How to manage multiple profiles

Use profiles to switch between environments (production, staging, local):

```bash
# Login to the staging profile
./petstore auth login --profile staging --token "staging-token"

# Switch active profile
./petstore auth switch staging

# Configure server URL per profile
./petstore config set profiles.staging.server.url https://staging.api.example.com

# Check which profile is active
./petstore auth status
```

Each profile has its own server URL, auth method, and stored credentials.

## Security behavior

- All debug and verbose output redacts `Authorization` headers, API keys,
  and any header whose name contains `secret`, `token`, `key`, or `password`.
  Redacted values appear as `[REDACTED]`.
- Token display in `auth status` is truncated: `sk-t...cdef`.
- The `--verbose` flag shows which headers are sent without revealing secrets.

## Per-operation security

OpenAPI specs can define different security per operation:

```yaml
paths:
  /public/health:
    get:
      security: []           # No auth required
  /admin/users:
    get:
      security:
        - bearerAuth: []     # Requires bearer token
```

Operations with `security: []` do not prompt for or attach credentials.
