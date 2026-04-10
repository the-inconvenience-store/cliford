# Authentication Systems Research

> How generated apps should handle authentication - methods, storage, flows, and per-operation overrides.

## Prior Art: Speakeasy's Approach

Speakeasy supports these auth methods out of the box via OpenAPI security schemes:

| Method | Config Source | Notes |
|--------|-------------|-------|
| HTTP Basic | OpenAPI `securitySchemes` | Works automatically |
| API Key (header/query) | OpenAPI `securitySchemes` | Works automatically |
| Bearer Token | OpenAPI `securitySchemes` | Works automatically |
| OAuth 2.0 | OpenAPI + `x-speakeasy-*` extensions | Requires additional config for token refresh |
| mTLS | SDK hooks | Manual implementation via hook system |
| Custom | `x-speakeasy-custom-security` | Extension-driven |

Credential resolution hierarchy:
1. Command flags (e.g., `--api-key`, `--bearer-token`)
2. Environment variables (e.g., `MYAPP_API_KEY`)
3. OS keychain
4. Config file

## Auth Methods Cliford Should Support

### Tier 1: Out of the Box (from OpenAPI spec)

1. **API Key** (header, query, cookie)
   - Simplest pattern. Store key in keychain/config.
   - Flag: `--api-key` or `--<custom-name>`
   - Env: `<PREFIX>_API_KEY`

2. **HTTP Basic Auth**
   - Username + password
   - Flags: `--username`, `--password`
   - Store in keychain as a credential pair

3. **Bearer Token**
   - Static or long-lived tokens
   - Flag: `--bearer-token` or `--token`
   - Env: `<PREFIX>_BEARER_TOKEN`

4. **OAuth 2.0**
   - Support flows: Authorization Code, Client Credentials, Device Code
   - Token refresh handled automatically
   - Store access token + refresh token in keychain
   - Interactive browser-based login flow

5. **OpenID Connect**
   - Build on OAuth 2.0 with OIDC discovery
   - Auto-discover endpoints from `.well-known/openid-configuration`

### Tier 2: Extended (via configuration/hooks)

6. **mTLS (Mutual TLS)**
   - Client certificate + key paths in config
   - Hook into HTTP client transport

7. **AWS Signature V4**
   - Common for AWS-compatible APIs
   - Read from `~/.aws/credentials` or env vars

8. **Custom Header Auth**
   - Arbitrary header name/value pairs
   - Configurable via hooks

9. **Cookie-based Auth**
   - Session cookies from login endpoint
   - Cookie jar management

## Credential Storage Architecture

### The Auth Store

```
~/.config/<app>/
  config.yaml       # General app config
  auth/
    credentials.yaml # Encrypted credential metadata
    keychain.yaml    # Fallback when OS keychain unavailable
```

### Storage Backends (Priority Order)

1. **OS Keychain** (preferred)
   - macOS: Keychain Access (`security` framework)
   - Windows: Credential Manager (Windows Credential API)
   - Linux: Secret Service API (GNOME Keyring / KDE Wallet)
   - Library: `github.com/zalando/go-keyring` or similar

2. **Encrypted File** (fallback)
   - When OS keychain is unavailable (containers, CI, headless servers)
   - AES-256-GCM encryption
   - Key derived from machine-specific entropy or user passphrase
   - Stored in `~/.config/<app>/auth/keychain.yaml`

3. **Environment Variables** (always available)
   - `<PREFIX>_API_KEY`, `<PREFIX>_TOKEN`, etc.
   - No storage - read at runtime

4. **Config File** (least preferred)
   - Plain text in config file
   - Warning emitted when secrets detected in plain config
   - Useful for non-sensitive identifiers

### Multi-Profile Support

Apps should support multiple auth profiles (e.g., dev/staging/prod):

```yaml
# ~/.config/myapp/config.yaml
profiles:
  default:
    server: https://api.example.com
    auth_method: bearer
  staging:
    server: https://staging.api.example.com
    auth_method: bearer
  dev:
    server: http://localhost:8080
    auth_method: basic
    
active_profile: default
```

Switch profiles: `myapp config use-profile staging`

Each profile has its own credential set in the keychain.

## Auth Commands (Generated)

When `interactiveAuth` is enabled in Cliford config:

```
myapp auth login          # Interactive login flow
myapp auth logout         # Clear stored credentials
myapp auth status         # Show current auth state (redacted)
myapp auth switch         # Switch between profiles
myapp auth refresh        # Force token refresh (OAuth)
myapp config set-token    # Manually set a token
```

### Interactive Login Flow (TUI)

```
$ myapp auth login

  Authentication Method
  > API Key
    Bearer Token
    OAuth 2.0 (Browser)
    Username & Password

  [enter] select  [q] cancel
```

After selection, appropriate TUI form or browser redirect.

### Headless/CI Auth

```bash
# Environment variables (recommended for CI)
export MYAPP_API_KEY="sk-..."
myapp users list

# Flag-based (one-off)
myapp users list --api-key "sk-..."

# Config-based (scripted setup)
myapp config set api-key "sk-..."
```

## Per-Operation Auth Overrides

OpenAPI specs allow different security schemes per operation:

```yaml
paths:
  /public/health:
    get:
      security: []  # No auth required
  /admin/users:
    get:
      security:
        - bearerAuth: []
        - apiKeyAuth: []  # Alternative auth
  /webhooks/register:
    post:
      security:
        - oauth2:
            - webhooks:write
```

Cliford should:
1. Parse per-operation security requirements from OpenAPI
2. Generate appropriate flags per command
3. Skip auth for operations with `security: []`
4. Support alternative auth (try first, fall back to second)
5. Include required OAuth scopes in help text

## Token Lifecycle Management

### OAuth Token Refresh

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Access   │────>│  Check   │────>│  API     │
│  Token    │     │  Expiry  │     │  Call    │
└──────────┘     └────┬─────┘     └──────────┘
                      │ expired
                 ┌────▼─────┐
                 │  Refresh  │
                 │  Token    │
                 └────┬─────┘
                      │ success
                 ┌────▼─────┐
                 │  Update   │
                 │  Keychain │
                 └──────────┘
```

- Auto-refresh before expiry (proactive, not reactive)
- If refresh fails, prompt re-login
- Store token expiry time alongside token

### Session Management

- Track when credentials were last used
- Warn on long-unused credentials
- `auth status` shows credential age and validity

## Security Considerations

1. **Never log credentials** - Redact in debug output, `--dry-run`, and error messages
2. **Warn on insecure storage** - Alert if using plain config file for secrets
3. **Warn on HTTP** - Alert when sending credentials over non-HTTPS
4. **Credential rotation** - Support updating credentials without re-login
5. **Scope minimization** - Only request OAuth scopes needed for the operation
6. **Secure deletion** - Zero memory after use where possible

## Implementation Dependencies

- `github.com/zalando/go-keyring` - Cross-platform keychain access
- `golang.org/x/oauth2` - OAuth 2.0 flows
- `golang.org/x/crypto` - Encryption for file-based fallback
- Standard `crypto/aes`, `crypto/cipher` - AES-256-GCM for encrypted storage
