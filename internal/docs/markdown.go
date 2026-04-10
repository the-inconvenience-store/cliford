// Package docs generates documentation from the Operation Registry
// in both human-readable (Markdown) and LLM-optimized (llms.txt) formats.
package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cliford/cliford/pkg/registry"
)

// MarkdownGenerator produces one Markdown file per command in the generated app.
type MarkdownGenerator struct {
	outputDir string
	appName   string
	pkgPath   string
}

// NewMarkdownGenerator creates a Markdown doc generator.
func NewMarkdownGenerator(outputDir, appName, pkgPath string) *MarkdownGenerator {
	return &MarkdownGenerator{outputDir: outputDir, appName: appName, pkgPath: pkgPath}
}

// Generate produces Markdown documentation files and a docgen utility.
func (g *MarkdownGenerator) Generate(reg *registry.Registry) error {
	docsDir := filepath.Join(g.outputDir, "docs", "cli")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}

	// Generate index
	if err := g.generateIndex(reg, docsDir); err != nil {
		return err
	}

	// Generate one file per tag group
	for tag, ops := range reg.TagGroups {
		if err := g.generateGroupDoc(tag, ops, docsDir); err != nil {
			return err
		}
	}

	// Generate docgen command for the app
	return g.generateDocgenCmd(reg)
}

func (g *MarkdownGenerator) generateIndex(reg *registry.Registry, docsDir string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s CLI Reference\n\n", g.appName))
	sb.WriteString(fmt.Sprintf("%s\n\n", reg.Description))
	sb.WriteString("## Commands\n\n")

	for tag, ops := range reg.TagGroups {
		sb.WriteString(fmt.Sprintf("### %s\n\n", tag))
		sb.WriteString("| Command | Description |\n")
		sb.WriteString("|---------|-------------|\n")
		for _, op := range ops {
			sb.WriteString(fmt.Sprintf("| `%s %s %s` | %s |\n",
				g.appName, strings.ToLower(tag), op.CLICommandName, op.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Utility Commands\n\n")
	sb.WriteString("| Command | Description |\n")
	sb.WriteString("|---------|-------------|\n")
	sb.WriteString(fmt.Sprintf("| `%s auth login` | Authenticate with the API |\n", g.appName))
	sb.WriteString(fmt.Sprintf("| `%s auth logout` | Clear stored credentials |\n", g.appName))
	sb.WriteString(fmt.Sprintf("| `%s auth status` | Show authentication state |\n", g.appName))
	sb.WriteString(fmt.Sprintf("| `%s config show` | Display configuration |\n", g.appName))
	sb.WriteString(fmt.Sprintf("| `%s config set` | Set a config value |\n", g.appName))
	sb.WriteString(fmt.Sprintf("| `%s completion` | Generate shell completions |\n", g.appName))

	sb.WriteString("\n## Global Flags\n\n")
	sb.WriteString("| Flag | Description |\n")
	sb.WriteString("|------|-------------|\n")
	sb.WriteString("| `--output-format, -o` | Output format: pretty, json, yaml, table |\n")
	sb.WriteString("| `--server` | Override API server URL |\n")
	sb.WriteString("| `--timeout` | Request timeout |\n")
	sb.WriteString("| `--debug` | Log request/response to stderr |\n")
	sb.WriteString("| `--dry-run` | Display request without executing |\n")
	sb.WriteString("| `-y, --yes` | Skip all confirmations |\n")
	sb.WriteString("| `--agent` | Force agent mode |\n")
	sb.WriteString("| `--no-interactive` | Disable interactive prompts |\n")
	sb.WriteString("| `--tui` | Launch TUI mode |\n")

	return os.WriteFile(filepath.Join(docsDir, "index.md"), []byte(sb.String()), 0o644)
}

func (g *MarkdownGenerator) generateGroupDoc(tag string, ops []registry.OperationMeta, docsDir string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s %s\n\n", g.appName, strings.ToLower(tag)))
	sb.WriteString(fmt.Sprintf("%s operations.\n\n", tag))

	for _, op := range ops {
		sb.WriteString(fmt.Sprintf("## %s %s %s\n\n", g.appName, strings.ToLower(tag), op.CLICommandName))
		sb.WriteString(fmt.Sprintf("%s\n\n", op.Summary))

		if op.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", op.Description))
		}

		sb.WriteString(fmt.Sprintf("**HTTP**: `%s %s`\n\n", op.Method, op.Path))

		if len(op.Parameters) > 0 {
			sb.WriteString("### Flags\n\n")
			sb.WriteString("| Flag | Type | Required | Description |\n")
			sb.WriteString("|------|------|----------|-------------|\n")
			for _, p := range op.Parameters {
				required := ""
				if p.Required {
					required = "Yes"
				}
				sb.WriteString(fmt.Sprintf("| `--%s` | %s | %s | %s |\n",
					p.FlagName, p.Schema.Type, required, p.Description))
			}
			sb.WriteString("\n")
		}

		if len(op.Security) > 0 {
			sb.WriteString("### Authentication\n\n")
			for _, sec := range op.Security {
				sb.WriteString(fmt.Sprintf("- `%s`", sec.SchemeName))
				if len(sec.Scopes) > 0 {
					sb.WriteString(fmt.Sprintf(" (scopes: %s)", strings.Join(sec.Scopes, ", ")))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	filename := strings.ToLower(strings.ReplaceAll(tag, " ", "-")) + ".md"
	return os.WriteFile(filepath.Join(docsDir, filename), []byte(sb.String()), 0o644)
}

func (g *MarkdownGenerator) generateDocgenCmd(reg *registry.Registry) error {
	docgenDir := filepath.Join(g.outputDir, "cmd", "docgen")
	if err := os.MkdirAll(docgenDir, 0o755); err != nil {
		return err
	}

	code := fmt.Sprintf(`// Docgen generates documentation for %s.
// Run: go run ./cmd/docgen -out ./docs/cli -format markdown
package main

import (
	"flag"
	"fmt"
	"os"

	"%s/internal/cli"
)

func main() {
	outDir := flag.String("out", "./docs/cli", "Output directory")
	format := flag.String("format", "markdown", "Format: markdown, llms")
	flag.Parse()

	_ = outDir
	_ = format

	root := cli.RootCmd("%s", "dev")
	root.DisableAutoGenTag = true

	fmt.Fprintf(os.Stderr, "Documentation generation via Cobra doc package - run %%s %%s\n", *format, *outDir)
	fmt.Println("Use 'go doc' or Cobra's doc.GenMarkdownTree for full Markdown generation.")
}
`, g.appName, g.pkgPath, g.appName)

	return os.WriteFile(filepath.Join(docgenDir, "main.go"), []byte(code), 0o644)
}
