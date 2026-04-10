package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// LLMsGenerator produces llms.txt — a flat, LLM-optimized documentation file.
type LLMsGenerator struct {
	outputDir string
	appName   string
}

// NewLLMsGenerator creates an llms.txt generator.
func NewLLMsGenerator(outputDir, appName string) *LLMsGenerator {
	return &LLMsGenerator{outputDir: outputDir, appName: appName}
}

// Generate produces the llms.txt file.
func (g *LLMsGenerator) Generate(reg *registry.Registry) error {
	docsDir := filepath.Join(g.outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", g.appName))
	sb.WriteString(fmt.Sprintf("> %s\n\n", reg.Description))
	sb.WriteString(fmt.Sprintf("Version: %s\n", reg.Version))
	if len(reg.Servers) > 0 {
		sb.WriteString(fmt.Sprintf("Default server: %s\n", reg.Servers[0].URL))
	}
	sb.WriteString("\n")

	// Authentication
	if len(reg.SecuritySchemes) > 0 {
		sb.WriteString("## Authentication\n\n")
		for name, scheme := range reg.SecuritySchemes {
			sb.WriteString(fmt.Sprintf("- **%s**: %s", name, scheme.Type))
			if scheme.Scheme != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", scheme.Scheme))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("Login: `%s auth login --method <method> --token <value>`\n\n", g.appName))
	}

	// Commands by tag
	for tag, ops := range reg.TagGroups {
		sb.WriteString(fmt.Sprintf("## %s\n\n", tag))

		for _, op := range ops {
			sb.WriteString(fmt.Sprintf("### %s %s %s\n\n", g.appName, strings.ToLower(tag), op.CLICommandName))
			sb.WriteString(fmt.Sprintf("%s\n\n", op.Summary))

			if op.Description != "" && op.Description != op.Summary {
				sb.WriteString(fmt.Sprintf("%s\n\n", op.Description))
			}

			sb.WriteString(fmt.Sprintf("HTTP: %s %s\n\n", op.Method, op.Path))

			// Flags
			if len(op.Parameters) > 0 {
				sb.WriteString("Flags:\n")
				for _, p := range op.Parameters {
					required := ""
					if p.Required {
						required = " (required)"
					}
					defVal := ""
					if p.Default != nil {
						defVal = fmt.Sprintf(" [default: %v]", p.Default)
					}
					sb.WriteString(fmt.Sprintf("  --%s <%s>%s%s — %s\n",
						p.FlagName, p.Schema.Type, required, defVal, p.Description))
				}
				sb.WriteString("\n")
			}

			// Auth requirements
			if len(op.Security) > 0 {
				sb.WriteString("Auth: ")
				var auths []string
				for _, sec := range op.Security {
					a := sec.SchemeName
					if len(sec.Scopes) > 0 {
						a += "(" + strings.Join(sec.Scopes, ",") + ")"
					}
					auths = append(auths, a)
				}
				sb.WriteString(strings.Join(auths, " | "))
				sb.WriteString("\n\n")
			} else {
				sb.WriteString("Auth: none required\n\n")
			}

			// Example
			example := fmt.Sprintf("%s %s %s", g.appName, strings.ToLower(tag), op.CLICommandName)
			for _, p := range op.Parameters {
				if p.Required {
					sb.WriteString(fmt.Sprintf("Example: `%s --%s <value>`\n\n", example, p.FlagName))
					break
				}
			}
		}
	}

	// Global flags
	sb.WriteString("## Global Flags\n\n")
	sb.WriteString("  --output-format, -o <string> — Output format: pretty, json, yaml, table [default: pretty]\n")
	sb.WriteString("  --server <string> — Override API server URL\n")
	sb.WriteString("  --timeout <string> — Request timeout [default: 30s]\n")
	sb.WriteString("  --debug — Log request/response to stderr (secrets redacted)\n")
	sb.WriteString("  --dry-run — Display HTTP request without executing\n")
	sb.WriteString("  -y, --yes — Skip all confirmations\n")
	sb.WriteString("  --agent — Force agent mode (structured JSON, no interactive)\n")
	sb.WriteString("  --no-interactive — Disable interactive prompts\n")
	sb.WriteString("  --tui — Launch full TUI mode\n")
	sb.WriteString("  --no-retries — Disable retries for this request\n")
	sb.WriteString("\n")

	sb.WriteString("## Utility Commands\n\n")
	sb.WriteString(fmt.Sprintf("  %s auth login — Authenticate with the API\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s auth logout — Clear stored credentials\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s auth status — Show current auth state\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s auth switch <profile> — Switch active profile\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s config show — Display current configuration\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s config set <key> <value> — Set a config value\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s config get <key> — Get a config value\n", g.appName))
	sb.WriteString(fmt.Sprintf("  %s completion <shell> — Generate shell completions\n", g.appName))

	return os.WriteFile(filepath.Join(docsDir, "llms.txt"), []byte(sb.String()), 0o644)
}
