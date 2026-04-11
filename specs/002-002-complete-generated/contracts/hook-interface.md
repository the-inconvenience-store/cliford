# Contract: Hook Interface

**Type**: Extension point contract  
**Consumers**: Developers writing hooks for generated Cliford apps  
**Producers**: Cliford-generated `internal/hooks/` package

---

## Shell Hook Contract

### Invocation

The generated app execs the configured command as a subprocess.

```
<command> [args...]
```

- `stdin`: UTF-8 JSON object (see schemas below)
- `stdout`: Ignored
- `stderr`: Displayed to user on non-zero exit
- `exit code 0`: Continue with request/response
- `exit code != 0`: Abort — request is not sent (before hooks) or response is suppressed (after hooks)

### Before-Request JSON Schema

```json
{
  "operation_id": "string",
  "method": "string",
  "url": "string",
  "headers": { "Header-Name": "value or [REDACTED]" },
  "body": "base64-encoded string or null",
  "timestamp": "ISO 8601"
}
```

### After-Response JSON Schema

```json
{
  "operation_id": "string",
  "method": "string",
  "url": "string",
  "headers": { "Header-Name": "value" },
  "body": "base64-encoded string or null",
  "timestamp": "ISO 8601",
  "status_code": 200,
  "response_headers": { "Header-Name": "value" },
  "response_body": "base64-encoded string or null",
  "elapsed_ms": 142,
  "error": "string or empty"
}
```

---

## go-plugin Hook Contract

Advanced hooks use `hashicorp/go-plugin` with gRPC transport.

### Proto Interface

```proto
syntax = "proto3";

message HookRequest {
  string operation_id = 1;
  string method = 2;
  string url = 3;
  map<string, string> headers = 4;
  bytes body = 5;
  int64 timestamp_unix = 6;
  int32 status_code = 7;          // 0 for before-request
  map<string, string> response_headers = 8;
  bytes response_body = 9;
  int64 elapsed_ms = 10;
  string error = 11;
}

message HookResponse {
  bool abort = 1;
  string abort_reason = 2;
  map<string, string> modified_headers = 3; // Optional: replace headers
}

service HookService {
  rpc BeforeRequest(HookRequest) returns (HookResponse);
  rpc AfterResponse(HookRequest) returns (HookResponse);
}
```

### Plugin Discovery

Plugin binaries are configured in `cliford.yaml` (generator config) or the generated app's config file:

```yaml
features:
  hooks:
    enabled: true
    before_request:
      - type: shell
        command: "scripts/log-request.sh"
      - type: go-plugin
        plugin_path: "plugins/custom-auth"
    after_response:
      - type: shell
        command: "scripts/log-response.sh"
```
