# Contract: Credential Resolution

**Type**: Internal contract  
**Consumers**: All generated app commands that make authenticated HTTP requests  
**Producers**: Generated `internal/auth/resolver.go`

---

## Resolution Order

For each security scheme, credentials are resolved in this priority order (highest first):

1. **CLI flag** — `--<scheme>-token`, `--<scheme>-api-key`, etc.
2. **Environment variable** — `<APP>_<SCHEME_NAME>_<CREDENTIAL_TYPE>`
3. **OS keychain** — stored under service=`<app-name>`, account=`<SCHEME>_<TYPE>`
4. **Encrypted file fallback** — `~/.config/<app-name>/credentials.enc` (AES-256-GCM, passphrase derived from machine ID)
5. **Plain-text config file** — `~/<app-name>.yaml` `auth.<scheme>.credential` (last resort; security warning generated)

The first non-empty value wins. If all sources are empty for a **required** scheme, the app exits with:

```
Error: authentication required
  Scheme: <scheme-name> (<type>)
  Configure via: <APP>_<SCHEME_NAME>_<CREDENTIAL_TYPE> env var
              or: <app-name> auth <scheme-name> login
```

---

## Env Var Naming Convention

| Scheme Type | Example Scheme Name | Env Var |
|-------------|---------------------|---------|
| apiKey | `ApiKeyAuth` | `PETSTORE_APIKEYAUTH_API_KEY` |
| http bearer | `BearerAuth` | `PETSTORE_BEARERAUTH_TOKEN` |
| http basic | `BasicAuth` | `PETSTORE_BASICAUTH_USERNAME`, `PETSTORE_BASICAUTH_PASSWORD` |
| oauth2 | `OAuth2` | `PETSTORE_OAUTH2_CLIENT_ID`, `PETSTORE_OAUTH2_CLIENT_SECRET` |

Rules:
- App prefix: slugified `info.title` uppercased (e.g., "Pet Store API" → `PETSTORE`)
- Scheme segment: scheme name uppercased with non-alphanumeric removed
- Type suffix: `API_KEY`, `TOKEN`, `USERNAME`, `PASSWORD`, `CLIENT_ID`, `CLIENT_SECRET`

---

## Sensitive Value Redaction

The following headers are always redacted in verbose output and logs:

- `Authorization`
- `X-Api-Key`
- `X-Auth-Token`
- Any header whose name contains `secret`, `token`, `key`, or `password` (case-insensitive)

Redacted display: `[REDACTED]`

---

## Keychain Entry Lifecycle

| Event | Action |
|-------|--------|
| `<app> auth <scheme> login` | Store in keychain |
| `<app> auth <scheme> logout` | Delete from keychain |
| `<app> auth <scheme> status` | Read from keychain (display masked) |
| Token expiry detected | Re-fetch and update keychain entry |
| OAuth2 refresh | Update keychain entry with new token + new expiry |
