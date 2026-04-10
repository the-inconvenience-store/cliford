# Distribution

Cliford generates everything needed to ship your CLI app to users on macOS,
Linux, and Windows.

## Enabling Release Artifacts

```bash
cliford generate --release
```

Or in `cliford.yaml`:

```yaml
features:
  distribution:
    goreleaser: true
    homebrew: false    # Set true to generate a Homebrew formula
```

## Generated Files

| File | Purpose |
|------|---------|
| `.goreleaser.yaml` | Cross-platform build config |
| `.github/workflows/release.yaml` | GitHub Actions workflow (triggers on `v*` tags) |
| `install.sh` | Unix install script (macOS/Linux) |
| `install.ps1` | Windows install script |
| `homebrew/<app>.rb` | Homebrew formula template |

## GoReleaser

The generated `.goreleaser.yaml` produces binaries for:

- **OS**: Linux, macOS (Darwin), Windows
- **Arch**: amd64, arm64
- **Format**: `.tar.gz` (Unix), `.zip` (Windows)
- **Checksums**: SHA256 in `checksums.txt`

Version, commit hash, and build date are injected via ldflags:

```yaml
ldflags:
  - -X main.version={{.Version}}
  - -X main.commit={{.Commit}}
  - -X main.date={{.Date}}
```

### Creating a Release

```bash
# Tag a version
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions runs automatically, or run locally:
goreleaser release --clean
```

### Testing Locally

```bash
goreleaser check          # Validate config
goreleaser build --snapshot --clean  # Build without publishing
```

## Install Scripts

### Unix (install.sh)

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/myapp/main/install.sh | sh
```

The script:
1. Detects OS (linux/darwin) and architecture (amd64/arm64)
2. Fetches the latest release from GitHub
3. Downloads the binary and checksums
4. Verifies SHA256 checksum
5. Installs to `/usr/local/bin` (or uses `sudo` if needed)

### Windows (install.ps1)

```powershell
irm https://raw.githubusercontent.com/OWNER/myapp/main/install.ps1 | iex
```

The script installs to `%LOCALAPPDATA%\<app>\` and adds it to the user PATH.

## Homebrew

The generated formula in `homebrew/<app>.rb` is a template. To publish:

1. Create a tap repository: `OWNER/homebrew-tap`
2. Copy the formula file to `Formula/<app>.rb`
3. Update SHA256 hashes and version after each release

Then users install with:

```bash
brew tap OWNER/tap
brew install <app>
```

## Version Management

### SemVer

Generated apps use semantic versioning. The version is set in `cliford.yaml`
and injected at build time via ldflags.

```bash
./myapp --version
# myapp version 1.2.3 (commit: abc1234, built: 2026-04-10T12:00:00Z)
```

### Bumping

```bash
cliford version patch    # 1.2.3 -> 1.2.4
cliford version minor    # 1.2.3 -> 1.3.0
cliford version major    # 1.2.3 -> 2.0.0
cliford version auto     # Analyze spec changes, pick automatically
```

Auto-bump logic:
- **Operations added** -> minor
- **Operations removed or signatures changed** -> major
- **Descriptions/metadata only** -> patch

## Documentation Generation

Every generation run produces:

| Output | Location | Description |
|--------|----------|-------------|
| Markdown CLI reference | `docs/cli/index.md` | Command index with all operations |
| Per-tag docs | `docs/cli/<tag>.md` | Operations with flags, auth, HTTP details |
| LLM-optimized docs | `docs/llms.txt` | Flat text optimized for AI agent context windows |
| Docgen utility | `cmd/docgen/main.go` | Run to regenerate docs from the command tree |

The `docs/llms.txt` file is designed to be pasted into AI assistants for
context about your CLI tool.
