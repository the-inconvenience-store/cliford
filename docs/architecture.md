# Architecture

This document explains the design decisions behind Cliford's generated
applications. It covers the transport chain, credential resolution, and how
the different layers work together.

## The generation pipeline

When you run `cliford generate`, the pipeline executes five stages in order:

1. **Validate**: Parse the OpenAPI spec, build the Operation Registry, check
   for conflicts.
2. **SDK**: Generate typed Go client code via oapi-codegen, plus retry,
   pagination, and error handling helpers.
3. **CLI**: Generate Cobra commands for each operation, grouped by OpenAPI
   tag. Generate auth, config, and generate-docs commands.
4. **TUI**: Generate Bubbletea views (if enabled), plus the hybrid mode
   adapter for inline prompts.
5. **Infra**: Generate `main.go`, `go.mod`, config package, and optionally
   GoReleaser, install scripts, and Homebrew formula.

Each stage can be extended with pipeline hooks (see [Hooks](hooks.md)).

## The shared HTTP client

All generated commands share a single `http.Client` instance created at
startup in `main.go`. This follows the SDK-first principle: authentication,
retries, verbose logging, hooks, and global parameters are implemented once
as transport layers, not duplicated in each command.

The `http.Client` is assembled by `internal/client/factory.go` and injected
into the CLI package via `cli.SetAPIClient()`.

## The transport chain

The shared HTTP client wraps `http.DefaultTransport` with a chain of
`http.RoundTripper` layers. Each layer handles one concern:

```
Request flow (outermost to innermost):

  VerboseTransport     Logs request/response to stderr when --verbose is set.
                       Redacts sensitive headers.
       |
  HooksTransport       Calls before_request hooks (shell or go-plugin).
                       Calls after_response hooks after the response.
       |
  GlobalParamsTransport Injects headers and query params from config
                       (global_params.headers, global_params.query).
       |
  AuthTransport        Resolves credentials via the 5-tier chain and
                       injects the correct Authorization header.
       |
  RetryTransport       Retries on transient failures (429, 503, etc.)
                       with exponential backoff and jitter.
       |
  http.DefaultTransport  Standard Go HTTP transport.
```

Each layer wraps the one below it. When a request is made:

1. VerboseTransport logs the request, then passes it down.
2. HooksTransport calls before_request hooks, then passes it down.
3. GlobalParamsTransport adds configured headers and query params.
4. AuthTransport resolves and attaches credentials.
5. RetryTransport sends the request. If it fails with a retryable status,
   it waits and retries through the same chain (steps 4 and 5 only).
6. The response bubbles back up through each layer.

This layering means verbose logging sees the final request (after auth
headers are added), and retry happens below the auth layer (so refreshed
credentials are used on each attempt).

## Credential resolution

The auth system resolves credentials through a 5-tier chain, stopping at the
first tier that provides a value:

```
1. CLI flags          --token, --api-key
2. Environment vars   <APP>_<SCHEME>_<TYPE>
3. OS keychain        macOS Keychain, Linux Secret Service, Windows Credential Manager
4. Encrypted file     AES-256-GCM encrypted, in ~/.config/<app>/auth/
5. Config file        YAML via Viper
```

This ordering is intentional:

- **Flags** allow per-invocation override. A CI script can pass `--token`
  without affecting stored credentials.
- **Environment variables** are the standard mechanism for secrets in CI/CD
  and container environments. They do not persist to disk.
- **OS keychain** is the most secure persistent store. It uses the
  platform's native secret storage, which is encrypted at rest and access-
  controlled by the operating system.
- **Encrypted file** is the fallback for environments without a keychain
  daemon (containers, CI runners, headless servers). The file is encrypted
  with AES-256-GCM using a key derived from the app name, profile, hostname,
  and OS.
- **Config file** is the last resort. Credentials in the config file are in
  plain text. The generator produces a warning in the documentation advising
  users to protect the file and avoid committing it to version control.

### Environment variable naming

Environment variables follow the pattern `<APP>_<SCHEME>_<TYPE>`:

- `APP` is the `envVarPrefix` from `cliford.yaml`, uppercased.
- `SCHEME` is the security scheme name from the OpenAPI spec, uppercased.
- `TYPE` depends on the scheme: `TOKEN` for bearer and OAuth2, `API_KEY`
  for apiKey, `USERNAME` and `PASSWORD` for basic auth.

This convention is deterministic and derived entirely from spec metadata. No
additional configuration is needed.

## Why a shared client instead of per-command HTTP calls

Earlier versions of Cliford generated inline HTTP request construction in
each command's `RunE` function. This approach had several problems:

- Auth logic was duplicated in every command.
- Retry behavior was inconsistent.
- Adding a new cross-cutting concern (like verbose logging) required
  modifying every generated command.
- Testing individual layers was difficult.

The shared client approach solves these by centralizing all HTTP concerns in
the transport chain. Each command only needs to construct the URL, parameters,
and body, then call `GetAPIClient().Do(req)`. The transport chain handles
everything else.

## Loading spinner

Each generated command wraps the HTTP call in a `withSpinner` function. While
the request is in flight, a small animation cycles on stderr to indicate
loading. The spinner is generated from the `features.spinner` section in
`cliford.yaml`:

- `enabled`: Controls whether the spinner code is included in the generated
  app. When `false`, `withSpinner` becomes a no-op pass-through.
- `frames`: An array of strings that cycle as the animation. The default is
  braille dots (`⠋ ⠙ ⠹ ...`), but any Unicode or ASCII characters work.
- `intervalMs`: Time between frame changes in milliseconds. Lower values
  produce faster animation.

The spinner writes to stderr and clears itself when the response arrives, so
it never appears in piped or redirected stdout. It is also suppressed in
`--no-interactive` and `--agent` modes.

## Table output

For GET operations that return arrays, the generated app defaults to table
output. Column selection follows this logic:

1. If any response schema property has `x-cliford-display: true`, only those
   properties appear as columns.
2. If no properties have the extension, all properties are shown.
3. The `--fields` flag overrides both behaviors with a comma-separated list.

Tables are rendered with `text/tabwriter` from the Go standard library. Column
headers are sorted alphabetically for consistency.

## Confirmation prompts

DELETE operations and operations with `x-cliford-confirm: true` display a
`[y/N]` confirmation prompt before sending the request. The prompt defaults
to "No" when the user presses Enter without input.

The prompt is skipped when:
- `--yes` or `-y` is passed
- stdin is not a terminal (piped input, CI environment)
- `--agent` or `--no-interactive` mode is active

## Interactive prompts for missing arguments

When a command is run in a terminal without all required flags, the generated
app prompts for each missing value:

```
pet-id: _
```

This only applies to string parameters. The prompt is skipped in
non-interactive contexts (piped input, `--no-interactive`, `--agent`).
