// Package cli generates Cobra command trees from the Operation Registry.
package cli

import (
	"fmt"
	goformat "go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/the-inconvenience-store/cliford/internal/codegen"
	"github.com/the-inconvenience-store/cliford/internal/config"
	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// SpinnerConfig controls the loading spinner in generated apps.
type SpinnerConfig struct {
	Enabled    bool
	Frames     []string
	IntervalMs int
}

// DefaultSpinnerConfig returns the default spinner settings.
func DefaultSpinnerConfig() SpinnerConfig {
	return SpinnerConfig{
		Enabled:    true,
		Frames:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		IntervalMs: 80,
	}
}

// Generator produces Cobra CLI code from the Operation Registry.
type Generator struct {
	engine              *codegen.Engine
	outputDir           string
	appName             string
	pkgPath             string
	envPrefix           string
	customCodeRegions   bool
	generateTUI         bool
	spinner             SpinnerConfig
	agentOutputFormat   string              // global default format when --agent is active
	flagsCfg            config.CLIFlagsConfig // controls which flags are generated
	requestIDEnabled    bool                // globally enable request ID injection
	requestIDHeader     string              // HTTP header name for request ID (default: X-Request-ID)
	watchEnabled        bool                // features.watch.enabled
	watchInterval       string              // features.watch.defaultInterval (e.g. "5s")
	watchMaxCount       int                 // features.watch.maxCount (0 = infinite)
	waitEnabled         bool                // features.wait.enabled
	waitInterval        string              // features.wait.defaultInterval (e.g. "15s")
	waitTimeout         string              // features.wait.defaultTimeout ("" = no timeout)
}

// NewGenerator creates a CLI generator.
func NewGenerator(engine *codegen.Engine, outputDir, appName, envPrefix string) *Generator {
	return &Generator{
		engine:    engine,
		outputDir: outputDir,
		appName:   appName,
		envPrefix: envPrefix,
		spinner:   DefaultSpinnerConfig(),
		flagsCfg:  config.DefaultFlagsConfig(),
	}
}

// SetSpinnerConfig sets the loading spinner configuration.
func (g *Generator) SetSpinnerConfig(cfg SpinnerConfig) {
	g.spinner = cfg
}

// SetCustomCodeRegions enables custom code region markers in generated output.
func (g *Generator) SetCustomCodeRegions(enabled bool) {
	g.customCodeRegions = enabled
}

// SetGenerateTUI enables TUI launch wiring in the root command.
func (g *Generator) SetGenerateTUI(enabled bool) {
	g.generateTUI = enabled
}

// SetAgentOutputFormat sets the global default output format used when --agent
// is active and --output-format was not explicitly set at runtime.
func (g *Generator) SetAgentOutputFormat(format string) {
	g.agentOutputFormat = format
}

// SetRequestID enables per-request UUID injection and sets the HTTP header name.
// When enabled, every generated RunE creates a requestID variable, sets it as a
// request header, and embeds it in all error messages for server-side correlation.
func (g *Generator) SetRequestID(enabled bool, header string) {
	g.requestIDEnabled = enabled
	g.requestIDHeader = header
	if g.requestIDHeader == "" {
		g.requestIDHeader = "X-Request-ID"
	}
}

// SetFlagsConfig sets which global flags are generated and their defaults.
func (g *Generator) SetFlagsConfig(cfg config.CLIFlagsConfig) {
	g.flagsCfg = cfg
}

// SetWatchConfig enables watch/poll mode and sets the global defaults baked into
// generated flag defaults. interval is the default --poll-interval value (e.g. "5s");
// maxCount is the default --watch-count value (0 = infinite).
func (g *Generator) SetWatchConfig(enabled bool, interval string, maxCount int) {
	g.watchEnabled = enabled
	g.watchInterval = interval
	if g.watchInterval == "" {
		g.watchInterval = "5s"
	}
	g.watchMaxCount = maxCount
}

// SetPackagePath sets the Go module path for import statements.
func (g *Generator) SetPackagePath(pkgPath string) {
	g.pkgPath = pkgPath
}

// SetWaitConfig enables wait mode and sets the global defaults baked into
// generated flag defaults. interval is the default polling interval (e.g. "15s");
// timeout is the default --wait-timeout value ("" = no timeout).
func (g *Generator) SetWaitConfig(enabled bool, interval, timeout string) {
	g.waitEnabled = enabled
	g.waitInterval = interval
	if g.waitInterval == "" {
		g.waitInterval = "15s"
	}
	g.waitTimeout = timeout
}

// isWatchEnabledForOp reports whether watch/poll mode should be generated for op.
// Returns false for non-GET operations (side effects make polling semantically wrong).
// Per-op CLIWatchEnabled overrides the global flag; nil inherits.
func isWatchEnabledForOp(op registry.OperationMeta, globalEnabled bool) bool {
	if op.Method != "GET" {
		return false
	}
	if op.CLIWatchEnabled != nil {
		return *op.CLIWatchEnabled
	}
	return globalEnabled
}

// isWaitEnabledForOp reports whether wait mode should be generated for op.
// Not restricted to GET — any operation with a configured condition or explicit enable qualifies.
// Per-op CLIWaitEnabled overrides; nil auto-detects from CLIWaitCondition presence.
func isWaitEnabledForOp(op registry.OperationMeta, globalEnabled bool) bool {
	if op.CLIWaitEnabled != nil {
		return *op.CLIWaitEnabled
	}
	// Auto-enable when a condition is configured (presence implies intent to use wait)
	if op.CLIWaitCondition != "" {
		return globalEnabled
	}
	return false
}

// Generate produces all CLI source files from the registry.
// It generates the files directly using Go string building rather than
// Go text/template, because the generated output IS Go source code and
// template-in-template is error-prone.
func (g *Generator) Generate(reg *registry.Registry) error {
	cliDir := filepath.Join(g.outputDir, "internal", "cli")

	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		return fmt.Errorf("create CLI dir: %w", err)
	}

	// Generate root command with all global flags
	if err := g.generateRoot(reg, cliDir); err != nil {
		return fmt.Errorf("generate root: %w", err)
	}

	// Generate one file per tag group with operation commands
	for tag, ops := range reg.TagGroups {
		if err := g.generateGroup(tag, ops, reg, cliDir); err != nil {
			return fmt.Errorf("generate group %s: %w", tag, err)
		}
	}

	return nil
}

func (g *Generator) generateRoot(reg *registry.Registry, cliDir string) error {
	// Determine whether any operation needs the generateRequestID() helper.
	needsRequestID := g.requestIDEnabled
	if !needsRequestID {
		for _, ops := range reg.TagGroups {
			for _, op := range ops {
				if op.CLIRequestID {
					needsRequestID = true
					break
				}
			}
			if needsRequestID {
				break
			}
		}
	}

	// Determine whether any GET operation has watch mode enabled.
	// When true, watchSleep_ and isTTYStdout_ helpers are emitted and "context" is imported.
	needsWatch := false
	if g.watchEnabled && g.flagsCfg.Watch.Enabled {
		for _, ops := range reg.TagGroups {
			for _, op := range ops {
				if isWatchEnabledForOp(op, g.watchEnabled) {
					needsWatch = true
					break
				}
			}
			if needsWatch {
				break
			}
		}
	}

	// Determine whether any operation has wait mode enabled.
	// When true, errWaitNotYet_ sentinel + evalWaitCondition_ helper are emitted,
	// and "errors" is imported.
	needsWait := false
	if g.waitEnabled && g.flagsCfg.Wait.Enabled {
		for _, ops := range reg.TagGroups {
			for _, op := range ops {
				if isWaitEnabledForOp(op, g.waitEnabled) {
					needsWait = true
					break
				}
			}
			if needsWait {
				break
			}
		}
	}

	var sb StringBuilder
	sb.Line("// Code generated by Cliford. DO NOT EDIT outside custom code regions.")
	sb.Line("package cli")
	sb.Line("")
	sb.Line("import (")
	if needsWatch || needsWait {
		sb.Line(`	"context"`)
	}
	if needsRequestID {
		sb.Line(`	"crypto/rand"`)
	}
	if needsWait {
		sb.Line(`	"errors"`)
	}
	sb.Line(`	"encoding/json"`)
	sb.Line(`	"fmt"`)
	sb.Line(`	"io"`)
	sb.Line(`	"net/http"`)
	sb.Line(`	"os"`)
	sb.Line(`	"sort"`)
	sb.Line(`	"strings"`)
	sb.Line(`	"text/tabwriter"`)
	sb.Line(`	"text/template"`)
	sb.Line(`	"time"`)
	sb.Line("")
	sb.Line(`	"github.com/charmbracelet/bubbles/progress"`)
	sb.Line(`	"github.com/itchyny/gojq"`)
	sb.Line(`	"github.com/spf13/cobra"`)
	sb.Line(`	toon "github.com/toon-format/toon-go"`)
	sb.Line(`	"gopkg.in/yaml.v3"`)
	if g.generateTUI && g.pkgPath != "" {
		sb.Line("")
		sb.Linef("	\"%s/internal/tui\"", g.pkgPath)
	}
	sb.Line(")")
	sb.Line("")
	sb.Line("var (")
	sb.Line(`	outputFormat  string`)
	sb.Line(`	jqFilter      string`)
	sb.Line(`	outputFile      string`)
	sb.Line(`	includeHeaders  bool`)
	sb.Line(`	serverURL     string`)
	sb.Line(`	debugMode     bool`)
	sb.Line(`	dryRunMode    bool`)
	sb.Line(`	yesMode       bool`)
	sb.Line(`	agentMode     bool`)
	sb.Line(`	noInteractive bool`)
	sb.Line(`	tuiMode       bool`)
	sb.Line(`	timeout       string`)
	sb.Line(`	templateExpr  string`)
	sb.Line(`	templateFile  string`)
	sb.Line("")
	sb.Line("	// apiClient is the shared HTTP client with auth, retry, and verbose transports.")
	sb.Line("	// Set via SetAPIClient() from main.go; falls back to a default client if nil.")
	sb.Line(`	apiClient *http.Client`)

	// Per-variable string vars for server URL template substitution
	if len(reg.Servers) > 0 && len(reg.Servers[0].Variables) > 0 {
		sb.Line("")
		sb.Line("	// Server URL template variables (from OpenAPI servers[0].variables).")
		sb.Line("	// Set via --server-<varname> persistent flags.")
		varNames := sortedStringKeys(reg.Servers[0].Variables)
		for _, varName := range varNames {
			sv := reg.Servers[0].Variables[varName]
			sb.Linef("	serverVar%s string // default: %q", toPascalCase(varName), sv.Default)
		}
	}

	sb.Line(")")
	sb.Line("")
	sb.Linef("// RootCmd returns the root Cobra command for %s.", g.appName)
	sb.Line("func RootCmd(appName string, version string) *cobra.Command {")
	sb.Line("	root := &cobra.Command{")
	sb.Linef("		Use:   %q,", g.appName)
	sb.Linef("		Short: %q,", reg.Description)
	sb.Line("		Version: version,")
	sb.Line("		SilenceUsage:  true,")
	sb.Line("		SilenceErrors: true,")
	if g.generateTUI {
		sb.Line("		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {")
		sb.Line("			if tuiMode {")
		sb.Line("				if err := tui.Run(); err != nil {")
		sb.Line(`					return fmt.Errorf("TUI error: %w", err)`)
		sb.Line("				}")
		sb.Line("				os.Exit(0)")
		sb.Line("			}")
		sb.Line("			return nil")
		sb.Line("		},")
	}
	sb.Line("	}")
	sb.Line("")
	sb.Line("	pf := root.PersistentFlags()")

	// --output-format
	if g.flagsCfg.OutputFormat.Enabled {
		def := g.flagsCfg.OutputFormat.Default
		if def == "" {
			def = "pretty"
		}
		sb.Linef(`	pf.StringVarP(&outputFormat, "output-format", "o", %q, "Output format: pretty, json, yaml, table, toon, go-template, jsonpath")`, def)
		if g.flagsCfg.OutputFormat.Hidden {
			sb.Line(`	_ = pf.MarkHidden("output-format")`)
		}
	}

	// --jq
	if g.flagsCfg.JQ.Enabled {
		sb.Line(`	pf.StringVar(&jqFilter, "jq", "", "Filter JSON output with a jq expression (gojq syntax)")`)
		if g.flagsCfg.JQ.Hidden {
			sb.Line(`	_ = pf.MarkHidden("jq")`)
		}
	}

	// --output-file
	if g.flagsCfg.OutputFile.Enabled {
		sb.Line(`	pf.StringVar(&outputFile, "output-file", "", "Write response body to a file instead of stdout")`)
		if g.flagsCfg.OutputFile.Hidden {
			sb.Line(`	_ = pf.MarkHidden("output-file")`)
		}
	}

	// --include-headers
	if g.flagsCfg.IncludeHeaders.Enabled {
		sb.Line(`	pf.BoolVar(&includeHeaders, "include-headers", false, "Print response headers alongside the body")`)
		if g.flagsCfg.IncludeHeaders.Hidden {
			sb.Line(`	_ = pf.MarkHidden("include-headers")`)
		}
	}

	// --server
	if g.flagsCfg.Server.Enabled {
		sb.Line(`	pf.StringVar(&serverURL, "server", "", "Override API server URL")`)
		if g.flagsCfg.Server.Hidden {
			sb.Line(`	_ = pf.MarkHidden("server")`)
		}
	}

	// Per-variable --server-<varname> flags (always generated; not user-configurable)
	if len(reg.Servers) > 0 && len(reg.Servers[0].Variables) > 0 {
		varNames := sortedStringKeys(reg.Servers[0].Variables)
		for _, varName := range varNames {
			sv := reg.Servers[0].Variables[varName]
			flagName := "server-" + toKebabCase(varName)
			goVar := "serverVar" + toPascalCase(varName)
			desc := sv.Description
			if desc == "" {
				desc = "Server URL variable: " + varName
			}
			sb.Linef("	pf.StringVar(&%s, %q, %q, %q)", goVar, flagName, sv.Default, desc)
		}
	}

	// --timeout
	if g.flagsCfg.Timeout.Enabled {
		def := g.flagsCfg.Timeout.Default
		if def == "" {
			def = "30s"
		}
		sb.Linef(`	pf.StringVar(&timeout, "timeout", %q, "Request timeout")`, def)
		if g.flagsCfg.Timeout.Hidden {
			sb.Line(`	_ = pf.MarkHidden("timeout")`)
		}
	}

	// --verbose / --debug
	if g.flagsCfg.Verbose.Enabled {
		sb.Line(`	pf.BoolVarP(&debugMode, "verbose", "v", false, "Log request/response to stderr (secrets redacted)")`)
		sb.Line(`	_ = pf.Bool("debug", false, "Alias for --verbose")`)
		sb.Line(`	_ = root.RegisterFlagCompletionFunc("debug", cobra.NoFileCompletions)`)
		if g.flagsCfg.Verbose.Hidden {
			sb.Line(`	_ = pf.MarkHidden("verbose")`)
			sb.Line(`	_ = pf.MarkHidden("debug")`)
		}
	}

	// --dry-run
	if g.flagsCfg.DryRun.Enabled {
		sb.Line(`	pf.BoolVar(&dryRunMode, "dry-run", false, "Display HTTP request without executing")`)
		if g.flagsCfg.DryRun.Hidden {
			sb.Line(`	_ = pf.MarkHidden("dry-run")`)
		}
	}

	// --yes
	if g.flagsCfg.Yes.Enabled {
		sb.Line(`	pf.BoolVarP(&yesMode, "yes", "y", false, "Skip all confirmations, use defaults")`)
		if g.flagsCfg.Yes.Hidden {
			sb.Line(`	_ = pf.MarkHidden("yes")`)
		}
	}

	// --agent
	if g.flagsCfg.Agent.Enabled {
		sb.Line(`	pf.BoolVar(&agentMode, "agent", false, "Force agent mode (structured JSON, no interactive)")`)
		if g.flagsCfg.Agent.Hidden {
			sb.Line(`	_ = pf.MarkHidden("agent")`)
		}
	}

	// --no-interactive
	if g.flagsCfg.NoInteractive.Enabled {
		sb.Line(`	pf.BoolVar(&noInteractive, "no-interactive", false, "Disable interactive prompts")`)
		if g.flagsCfg.NoInteractive.Hidden {
			sb.Line(`	_ = pf.MarkHidden("no-interactive")`)
		}
	}

	// --template
	if g.flagsCfg.Template.Enabled {
		sb.Line(`	pf.StringVar(&templateExpr, "template", "", "Go template or JSONPath expression (use with -o go-template|jsonpath)")`)
		if g.flagsCfg.Template.Hidden {
			sb.Line(`	_ = pf.MarkHidden("template")`)
		}
	}

	// --template-file
	if g.flagsCfg.TemplateFile.Enabled {
		sb.Line(`	pf.StringVar(&templateFile, "template-file", "", "Path to Go template or JSONPath file (use with -o go-template|jsonpath)")`)
		if g.flagsCfg.TemplateFile.Hidden {
			sb.Line(`	_ = pf.MarkHidden("template-file")`)
		}
	}

	// --tui (only when TUI is generated)
	if g.generateTUI && g.flagsCfg.TUI.Enabled {
		sb.Line(`	pf.BoolVar(&tuiMode, "tui", false, "Launch full TUI mode")`)
		if g.flagsCfg.TUI.Hidden {
			sb.Line(`	_ = pf.MarkHidden("tui")`)
		}
	}
	// Ensure pf is considered used even when all flags are disabled.
	sb.Line("	_ = pf")
	sb.Line("")

	// Sort tags for deterministic output
	var sortedTags []string
	for tag := range reg.TagGroups {
		sortedTags = append(sortedTags, tag)
	}
	sort.Strings(sortedTags)
	for _, tag := range sortedTags {
		sb.Linef("	root.AddCommand(%sCmd())", toCamelCase(tag))
	}
	sb.Line("	root.AddCommand(authCmd())")
	sb.Line("	root.AddCommand(configCmd())")
	sb.Line("	root.AddCommand(aliasCmd())")
	sb.Line("	root.AddCommand(GenerateDocsCmd())")

	if g.customCodeRegions {
		sb.Line("")
		sb.Line("	// --- CUSTOM CODE START: root:init ---")
		sb.Line("	// --- CUSTOM CODE END: root:init ---")
	}

	// Add server info to help if servers defined
	if len(reg.Servers) > 1 {
		sb.Line("")
		sb.Line(`	root.Long = root.Short + "\n\nAvailable servers:"`)
		for _, s := range reg.Servers {
			desc := s.Description
			if desc == "" {
				desc = s.URL
			}
			sb.Linef("	root.Long += %q", fmt.Sprintf("\n  --server %s  (%s)", s.URL, desc))
		}
	}

	sb.Line("")
	sb.Line("	return root")
	sb.Line("}")
	sb.Line("")

	// FormatOutput helper
	sb.Line("// FormatOutput renders data in the requested output format.")
	sb.Line("// Supports inline expressions: -o 'go-template={{.name}}' or -o 'jsonpath={.items[*].name}'.")
	sb.Line("func FormatOutput(data any, format string) error {")
	sb.Line("	// Extract inline expression: \"go-template={{.name}}\" → base=\"go-template\", expr=\"{{.name}}\"")
	sb.Line("	baseFormat := format")
	sb.Line("	inlineExpr := \"\"")
	sb.Line("	if idx := strings.IndexByte(format, '='); idx > 0 {")
	sb.Line("		baseFormat = format[:idx]")
	sb.Line("		inlineExpr = format[idx+1:]")
	sb.Line("	}")
	sb.Line("	switch strings.ToLower(baseFormat) {")
	sb.Line(`	case "json":`)
	sb.Line("		enc := json.NewEncoder(os.Stdout)")
	sb.Line(`		enc.SetIndent("", "  ")`)
	sb.Line("		return enc.Encode(data)")
	sb.Line(`	case "yaml":`)
	sb.Line("		enc := yaml.NewEncoder(os.Stdout)")
	sb.Line("		return enc.Encode(data)")
	sb.Line(`	case "table":`)
	sb.Line("		return formatTable(data)")
	sb.Line(`	case "toon":`)
	sb.Line("		s, err := toon.MarshalString(data)")
	sb.Line("		if err != nil {")
	sb.Line("			// Fall back to JSON on encoding failure")
	sb.Line("			enc := json.NewEncoder(os.Stdout)")
	sb.Line(`			enc.SetIndent("", "  ")`)
	sb.Line("			return enc.Encode(data)")
	sb.Line("		}")
	sb.Line("		fmt.Println(s)")
	sb.Line("		return nil")
	sb.Line(`	case "go-template":`)
	sb.Line("		tmplStr := inlineExpr")
	sb.Line("		if tmplStr == \"\" {")
	sb.Line("			tmplStr = templateExpr")
	sb.Line("		}")
	sb.Line("		if tmplStr == \"\" && templateFile != \"\" {")
	sb.Line("			b, err := os.ReadFile(templateFile)")
	sb.Line(`			if err != nil { return fmt.Errorf("read template file: %w", err) }`)
	sb.Line("			tmplStr = string(b)")
	sb.Line("		}")
	sb.Line("		if tmplStr == \"\" {")
	sb.Line(`			return fmt.Errorf("--output-format go-template requires --template or --template-file")`)
	sb.Line("		}")
	sb.Line("		return applyGoTemplate(data, tmplStr)")
	sb.Line(`	case "go-template-file":`)
	sb.Line("		if inlineExpr == \"\" {")
	sb.Line(`			return fmt.Errorf("--output-format go-template-file requires a file path: -o go-template-file=<path>")`)
	sb.Line("		}")
	sb.Line("		b, err := os.ReadFile(inlineExpr)")
	sb.Line(`		if err != nil { return fmt.Errorf("read template file: %w", err) }`)
	sb.Line("		return applyGoTemplate(data, string(b))")
	sb.Line(`	case "jsonpath":`)
	sb.Line("		expr := inlineExpr")
	sb.Line("		if expr == \"\" {")
	sb.Line("			expr = templateExpr")
	sb.Line("		}")
	sb.Line("		if expr == \"\" && templateFile != \"\" {")
	sb.Line("			b, err := os.ReadFile(templateFile)")
	sb.Line(`			if err != nil { return fmt.Errorf("read jsonpath file: %w", err) }`)
	sb.Line("			expr = string(b)")
	sb.Line("		}")
	sb.Line("		if expr == \"\" {")
	sb.Line(`			return fmt.Errorf("--output-format jsonpath requires --template or --template-file")`)
	sb.Line("		}")
	sb.Line("		return applyJSONPath(data, expr)")
	sb.Line(`	case "jsonpath-file":`)
	sb.Line("		if inlineExpr == \"\" {")
	sb.Line(`			return fmt.Errorf("--output-format jsonpath-file requires a file path: -o jsonpath-file=<path>")`)
	sb.Line("		}")
	sb.Line("		b, err := os.ReadFile(inlineExpr)")
	sb.Line(`		if err != nil { return fmt.Errorf("read jsonpath file: %w", err) }`)
	sb.Line("		return applyJSONPath(data, string(b))")
	sb.Line("	default:")
	sb.Line("		enc := json.NewEncoder(os.Stdout)")
	sb.Line(`		enc.SetIndent("", "  ")`)
	sb.Line("		return enc.Encode(data)")
	sb.Line("	}")
	sb.Line("}")
	sb.Line("")
	sb.Line("// formatTable renders an array of objects as a text table.")
	sb.Line("func formatTable(data any) error {")
	sb.Line("	items, ok := data.([]any)")
	sb.Line("	if !ok {")
	sb.Line("		// Not an array — fall back to JSON")
	sb.Line("		enc := json.NewEncoder(os.Stdout)")
	sb.Line(`		enc.SetIndent("", "  ")`)
	sb.Line("		return enc.Encode(data)")
	sb.Line("	}")
	sb.Line("	if len(items) == 0 {")
	sb.Line(`		fmt.Println("No results.")`)
	sb.Line("		return nil")
	sb.Line("	}")
	sb.Line("")
	sb.Line("	// Collect column headers from the first row")
	sb.Line("	firstRow, ok := items[0].(map[string]any)")
	sb.Line("	if !ok {")
	sb.Line("		enc := json.NewEncoder(os.Stdout)")
	sb.Line(`		enc.SetIndent("", "  ")`)
	sb.Line("		return enc.Encode(data)")
	sb.Line("	}")
	sb.Line("	var headers []string")
	sb.Line("	for k := range firstRow {")
	sb.Line("		headers = append(headers, k)")
	sb.Line("	}")
	sb.Line("	sort.Strings(headers)")
	sb.Line("")
	sb.Line("	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)")
	sb.Line("	// Print headers")
	sb.Line(`	fmt.Fprintln(w, strings.Join(headers, "\t"))`)
	sb.Line("	// Print separator")
	sb.Line("	seps := make([]string, len(headers))")
	sb.Line("	for i, h := range headers {")
	sb.Line("		seps[i] = strings.Repeat(\"-\", len(h))")
	sb.Line("	}")
	sb.Line(`	fmt.Fprintln(w, strings.Join(seps, "\t"))`)
	sb.Line("	// Print rows")
	sb.Line("	for _, item := range items {")
	sb.Line("		row, ok := item.(map[string]any)")
	sb.Line("		if !ok { continue }")
	sb.Line("		vals := make([]string, len(headers))")
	sb.Line("		for i, h := range headers {")
	sb.Line(`			vals[i] = fmt.Sprintf("%v", row[h])`)
	sb.Line("		}")
	sb.Line(`		fmt.Fprintln(w, strings.Join(vals, "\t"))`)
	sb.Line("	}")
	sb.Line("	return w.Flush()")
	sb.Line("}")
	sb.Line("")

	// applyJQ helper
	sb.Line("// applyJQ runs a jq expression against parsed JSON data.")
	sb.Line("// Returns a single value when the expression produces one result,")
	sb.Line("// a slice when it produces multiple, and nil when it produces none.")
	sb.Line("func applyJQ(input any, expr string) (any, error) {")
	sb.Line("	query, err := gojq.Parse(expr)")
	sb.Line("	if err != nil {")
	sb.Line(`		return nil, fmt.Errorf("parse jq expression %q: %w", expr, err)`)
	sb.Line("	}")
	sb.Line("	iter := query.Run(input)")
	sb.Line("	var results []any")
	sb.Line("	for {")
	sb.Line("		v, ok := iter.Next()")
	sb.Line("		if !ok {")
	sb.Line("			break")
	sb.Line("		}")
	sb.Line("		if jqErr, ok := v.(error); ok {")
	sb.Line("			return nil, jqErr")
	sb.Line("		}")
	sb.Line("		results = append(results, v)")
	sb.Line("	}")
	sb.Line("	switch len(results) {")
	sb.Line("	case 0:")
	sb.Line("		return nil, nil")
	sb.Line("	case 1:")
	sb.Line("		return results[0], nil")
	sb.Line("	default:")
	sb.Line("		return results, nil")
	sb.Line("	}")
	sb.Line("}")
	sb.Line("")

	// applyGoTemplate helper
	sb.Line("// applyGoTemplate renders data using a Go text/template expression.")
	sb.Line("// A \"json\" template function is available to serialize values inline.")
	sb.Line("func applyGoTemplate(data any, tmplStr string) error {")
	sb.Line("	tmpl, err := template.New(\"output\").Funcs(template.FuncMap{")
	sb.Line("		\"json\": func(v any) (string, error) {")
	sb.Line("			b, err := json.Marshal(v)")
	sb.Line("			return string(b), err")
	sb.Line("		},")
	sb.Line("	}).Parse(tmplStr)")
	sb.Line("	if err != nil {")
	sb.Line(`		return fmt.Errorf("parse template: %w", err)`)
	sb.Line("	}")
	sb.Line("	return tmpl.Execute(os.Stdout, data)")
	sb.Line("}")
	sb.Line("")

	// applyJSONPath helper
	sb.Line("// applyJSONPath evaluates a JSONPath expression against data using gojq.")
	sb.Line("// Supports kubectl-style {.items[*].name} syntax: curly braces are stripped,")
	sb.Line("// [*] is converted to the jq [] iterator, and leading $ is removed.")
	sb.Line("func applyJSONPath(data any, expr string) error {")
	sb.Line("	expr = strings.TrimSpace(expr)")
	sb.Line("	if len(expr) >= 2 && expr[0] == '{' && expr[len(expr)-1] == '}' {")
	sb.Line("		expr = expr[1 : len(expr)-1]")
	sb.Line("	}")
	sb.Line("	expr = strings.TrimPrefix(expr, \"$\")")
	sb.Line(`	jqExpr := strings.ReplaceAll(expr, "[*]", "[]")`)
	sb.Line("	result, err := applyJQ(data, jqExpr)")
	sb.Line("	if err != nil {")
	sb.Line(`		return fmt.Errorf("jsonpath: %w", err)`)
	sb.Line("	}")
	sb.Line("	if result == nil {")
	sb.Line("		return nil")
	sb.Line("	}")
	sb.Line("	switch r := result.(type) {")
	sb.Line("	case []any:")
	sb.Line("		parts := make([]string, 0, len(r))")
	sb.Line("		for _, item := range r {")
	sb.Line("			switch v := item.(type) {")
	sb.Line("			case string:")
	sb.Line("				parts = append(parts, v)")
	sb.Line("			default:")
	sb.Line("				b, _ := json.Marshal(v)")
	sb.Line("				parts = append(parts, string(b))")
	sb.Line("			}")
	sb.Line("		}")
	sb.Line(`		fmt.Fprintln(os.Stdout, strings.Join(parts, " "))`)
	sb.Line("	case string:")
	sb.Line("		fmt.Fprintln(os.Stdout, r)")
	sb.Line("	default:")
	sb.Line("		b, _ := json.Marshal(result)")
	sb.Line("		fmt.Fprintln(os.Stdout, string(b))")
	sb.Line("	}")
	sb.Line("	return nil")
	sb.Line("}")
	sb.Line("")

	// isTTYStderr helper
	sb.Line("// isTTYStderr reports whether stderr is connected to a terminal.")
	sb.Line("func isTTYStderr() bool {")
	sb.Line("	fi, _ := os.Stderr.Stat()")
	sb.Line("	return fi != nil && (fi.Mode()&os.ModeCharDevice) != 0")
	sb.Line("}")
	sb.Line("")

	// formatBytes helper
	sb.Line("// formatBytes formats a byte count as a human-readable string.")
	sb.Line("func formatBytes(n int64) string {")
	sb.Line("	const (")
	sb.Line("		KB = 1024")
	sb.Line("		MB = 1024 * KB")
	sb.Line("		GB = 1024 * MB")
	sb.Line("	)")
	sb.Line("	switch {")
	sb.Line("	case n >= GB:")
	sb.Line(`		return fmt.Sprintf("%.1f GB", float64(n)/GB)`)
	sb.Line("	case n >= MB:")
	sb.Line(`		return fmt.Sprintf("%.1f MB", float64(n)/MB)`)
	sb.Line("	case n >= KB:")
	sb.Line(`		return fmt.Sprintf("%.1f KB", float64(n)/KB)`)
	sb.Line("	default:")
	sb.Line(`		return fmt.Sprintf("%d B", n)`)
	sb.Line("	}")
	sb.Line("}")
	sb.Line("")

	// progressWriter struct
	sb.Line("// progressWriter wraps an io.Writer and calls onWrite after each write with the total bytes written.")
	sb.Line("type progressWriter struct {")
	sb.Line("	w       io.Writer")
	sb.Line("	written int64")
	sb.Line("	onWrite func(int64)")
	sb.Line("}")
	sb.Line("")
	sb.Line("func (pw *progressWriter) Write(p []byte) (int, error) {")
	sb.Line("	n, err := pw.w.Write(p)")
	sb.Line("	pw.written += int64(n)")
	sb.Line("	pw.onWrite(pw.written)")
	sb.Line("	return n, err")
	sb.Line("}")
	sb.Line("")

	// writeFileWithProgress helper
	sb.Line("// writeFileWithProgress streams body to path and shows download progress on stderr.")
	sb.Line("// Progress display adapts to the execution context:")
	sb.Line("//   - Interactive TTY: bubbles/progress bar (percentage or byte counter)")
	sb.Line("//   - Headless/--no-interactive: silent write + summary to stderr")
	sb.Line(`//   - Agent (--agent or AI env): silent write + JSON {"path","bytes","contentType"} to stdout`)
	sb.Line("func writeFileWithProgress(body io.Reader, contentLength int64, contentType, path string) error {")
	sb.Line("	f, err := os.Create(path)")
	sb.Line("	if err != nil {")
	sb.Line(`		return fmt.Errorf("create %s: %w", path, err)`)
	sb.Line("	}")
	sb.Line("	defer f.Close()")
	sb.Line("")
	sb.Line("	isInteractive := !noInteractive && !agentMode && isTTYStderr()")
	sb.Line("")
	sb.Line("	if !isInteractive {")
	sb.Line("		n, err := io.Copy(f, body)")
	sb.Line("		if err != nil {")
	sb.Line(`			return fmt.Errorf("write %s: %w", path, err)`)
	sb.Line("		}")
	sb.Line("		if agentMode {")
	sb.Line(`			fmt.Printf("{\"path\":%q,\"bytes\":%d,\"contentType\":%q}\n", path, n, contentType)`)
	sb.Line("		} else {")
	sb.Line(`			fmt.Fprintf(os.Stderr, "Wrote %s (%s)\n", path, formatBytes(n))`)
	sb.Line("		}")
	sb.Line("		return nil")
	sb.Line("	}")
	sb.Line("")
	sb.Line("	// Interactive TTY: use bubbles/progress for visual feedback")
	sb.Line("	bar := progress.New(progress.WithDefaultGradient())")
	sb.Line("	const clearWidth = 80")
	sb.Line("	pw := &progressWriter{")
	sb.Line("		w: f,")
	sb.Line("		onWrite: func(written int64) {")
	sb.Line("			if contentLength > 0 {")
	sb.Line("				pct := float64(written) / float64(contentLength)")
	sb.Line(`				fmt.Fprintf(os.Stderr, "\r%s", bar.ViewAs(pct))`)
	sb.Line("			} else {")
	sb.Line(`				fmt.Fprintf(os.Stderr, "\r%s written", formatBytes(written))`)
	sb.Line("			}")
	sb.Line("		},")
	sb.Line("	}")
	sb.Line("	n, err := io.Copy(pw, body)")
	sb.Line(`	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", clearWidth))`)
	sb.Line("	if err != nil {")
	sb.Line(`		return fmt.Errorf("write %s: %w", path, err)`)
	sb.Line("	}")
	sb.Line(`	fmt.Fprintf(os.Stderr, "Wrote %s (%s)\n", path, formatBytes(n))`)
	sb.Line("	return nil")
	sb.Line("}")

	// SetAPIClient function
	sb.Line("")
	sb.Line("// SetAPIClient sets the shared HTTP client used by all generated commands.")
	sb.Line("// The client should be pre-configured with auth, retry, and verbose transport layers.")
	sb.Line("func SetAPIClient(c *http.Client) {")
	sb.Line("	apiClient = c")
	sb.Line("}")
	sb.Line("")
	sb.Line("// GetAPIClient returns the shared HTTP client, falling back to a default if not configured.")
	sb.Line("func GetAPIClient() *http.Client {")
	sb.Line("	if apiClient != nil {")
	sb.Line("		return apiClient")
	sb.Line("	}")
	sb.Line(`	return &http.Client{Timeout: 30 * time.Second}`)
	sb.Line("}")
	sb.Line("")
	sb.Line("// SetDefaultServerURL sets the server URL from config/env.")
	sb.Line("// This is overridden by the --server CLI flag if provided.")
	sb.Line("func SetDefaultServerURL(url string) {")
	sb.Line("	if serverURL == \"\" {")
	sb.Line("		serverURL = url")
	sb.Line("	}")
	sb.Line("}")
	sb.Line("")
	sb.Line("// VerboseFlag returns a pointer to the verbose/debug mode flag.")
	sb.Line("// Pass this to client.Options.VerboseFlag to enable request/response logging.")
	sb.Line("func VerboseFlag() *bool {")
	sb.Line("	return &debugMode")
	sb.Line("}")
	sb.Line("")
	sb.Line("// promptValue prompts the user for a value on stdin.")
	sb.Line("// Returns empty string if stdin is not a terminal.")
	sb.Line("func promptValue(label string) string {")
	sb.Line("	fi, _ := os.Stdin.Stat()")
	sb.Line("	if fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {")
	sb.Line(`		return "" // not a TTY — skip prompt`)
	sb.Line("	}")
	sb.Line("	fmt.Fprint(os.Stderr, label)")
	sb.Line("	var line string")
	sb.Line("	fmt.Scanln(&line)")
	sb.Line("	return strings.TrimSpace(line)")
	sb.Line("}")
	// Spinner: configurable loading animation
	if g.spinner.Enabled {
		// Build frames literal
		var frameParts []string
		for _, f := range g.spinner.Frames {
			frameParts = append(frameParts, fmt.Sprintf("%q", f))
		}
		framesLiteral := strings.Join(frameParts, ", ")

		sb.Line("")
		sb.Line("// withSpinner displays a loading indicator on stderr while a function runs.")
		sb.Line("// It only shows when stderr is a terminal. Returns the function's results.")
		sb.Line("func withSpinner[T any](label string, fn func() (T, error)) (T, error) {")
		sb.Line("	fi, _ := os.Stderr.Stat()")
		sb.Line("	if fi == nil || (fi.Mode()&os.ModeCharDevice) == 0 || noInteractive || agentMode {")
		sb.Line("		return fn()")
		sb.Line("	}")
		sb.Line("")
		sb.Linef("	frames := []string{%s}", framesLiteral)
		sb.Line("	done := make(chan struct{})")
		sb.Line("	go func() {")
		sb.Line("		i := 0")
		sb.Line("		for {")
		sb.Line("			select {")
		sb.Line("			case <-done:")
		sb.Line(`				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", len(label)+4))`)
		sb.Line("				return")
		sb.Line("			default:")
		sb.Line(`				fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], label)`)
		sb.Line("				i++")
		sb.Linef("				time.Sleep(%d * time.Millisecond)", g.spinner.IntervalMs)
		sb.Line("			}")
		sb.Line("		}")
		sb.Line("	}()")
		sb.Line("")
		sb.Line("	result, err := fn()")
		sb.Line("	close(done)")
		sb.Line("	return result, err")
		sb.Line("}")
	} else {
		// When spinner is disabled, generate a pass-through withSpinner
		sb.Line("")
		sb.Line("// withSpinner is a no-op when spinner is disabled in cliford.yaml.")
		sb.Line("func withSpinner[T any](_ string, fn func() (T, error)) (T, error) {")
		sb.Line("	return fn()")
		sb.Line("}")
	}

	// generateRequestID helper — emitted only when at least one operation uses it.
	if needsRequestID {
		sb.Line("")
		sb.Line("// generateRequestID returns a random UUID v4 string for server-side correlation.")
		sb.Line("func generateRequestID() string {")
		sb.Line("	b := make([]byte, 16)")
		sb.Line("	_, _ = rand.Read(b)")
		sb.Line("	b[6] = (b[6] & 0x0f) | 0x40")
		sb.Line("	b[8] = (b[8] & 0x3f) | 0x80")
		sb.Line(`	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])`)
		sb.Line("}")
	}

	// Watch helpers — emitted only when at least one GET operation has watch enabled.
	if needsWatch {
		sb.Line("")
		sb.Line("// watchSleep_ blocks for d and returns true, or returns false immediately if")
		sb.Line("// ctx is cancelled (e.g. Ctrl+C). The trailing underscore avoids collisions")
		sb.Line("// with user-defined variables in custom code regions.")
		sb.Line("func watchSleep_(ctx context.Context, d time.Duration) bool {")
		sb.Line("	select {")
		sb.Line("	case <-time.After(d):")
		sb.Line("		return true")
		sb.Line("	case <-ctx.Done():")
		sb.Line("		return false")
		sb.Line("	}")
		sb.Line("}")
		sb.Line("")
		sb.Line("// isTTYStdout_ reports whether stdout is connected to a terminal.")
		sb.Line("func isTTYStdout_() bool {")
		sb.Line("	fi, _ := os.Stdout.Stat()")
		sb.Line("	return fi != nil && (fi.Mode()&os.ModeCharDevice) != 0")
		sb.Line("}")
	}

	// Wait helpers — emitted only when at least one operation has wait mode enabled.
	// watchSleep_ is reused for wait mode sleep; no duplication needed.
	if needsWait {
		if !needsWatch {
			// watchSleep_ is also used by wait mode; emit it if not already emitted.
			sb.Line("")
			sb.Line("// watchSleep_ blocks for d and returns true, or returns false immediately if")
			sb.Line("// ctx is cancelled (e.g. Ctrl+C).")
			sb.Line("func watchSleep_(ctx context.Context, d time.Duration) bool {")
			sb.Line("	select {")
			sb.Line("	case <-time.After(d):")
			sb.Line("		return true")
			sb.Line("	case <-ctx.Done():")
			sb.Line("		return false")
			sb.Line("	}")
			sb.Line("}")
		}
		sb.Line("")
		sb.Line("// errWaitNotYet_ is returned by the wait closure when the condition is not yet met.")
		sb.Line("// The outer loop catches it and continues polling without treating it as a real error.")
		sb.Line(`var errWaitNotYet_ = errors.New("wait: condition not yet met")`)
		sb.Line("")
		sb.Line("// evalWaitCondition_ evaluates a jq expression against v and returns true if")
		sb.Line("// the result is boolean true or any other non-nil, non-false value.")
		sb.Line("func evalWaitCondition_(v any, expr string) (bool, error) {")
		sb.Line("	if expr == \"\" {")
		sb.Line("		return false, nil")
		sb.Line("	}")
		sb.Line("	q, err := gojq.Parse(expr)")
		sb.Line("	if err != nil {")
		sb.Line("		return false, err")
		sb.Line("	}")
		sb.Line("	iter := q.Run(v)")
		sb.Line("	val, ok := iter.Next()")
		sb.Line("	if !ok {")
		sb.Line("		return false, nil")
		sb.Line("	}")
		sb.Line("	if err, isErr := val.(error); isErr {")
		sb.Line("		return false, err")
		sb.Line("	}")
		sb.Line("	if b, ok := val.(bool); ok {")
		sb.Line("		return b, nil")
		sb.Line("	}")
		sb.Line("	return val != nil, nil")
		sb.Line("}")
	}

	return writeFormatted(filepath.Join(cliDir, "root.go"), sb.String())
}

func (g *Generator) generateGroup(tag string, ops []registry.OperationMeta, reg *registry.Registry, cliDir string) error {
	// Determine which imports are needed
	needsBytes := false
	needsContext := false
	needsTime := false
	needsSDK := g.pkgPath != "" && g.flagsCfg.Retries.Enabled
	if needsSDK {
		needsTime = true // time.ParseDuration used in retry override block
	}
	for _, op := range ops {
		if op.RequestBody != nil {
			needsBytes = true
		}
		if op.Timeout != nil {
			needsContext = true
			needsTime = true
		}
		// Watch loop uses time.Now, time.ParseDuration, time.Second.
		// context.WithTimeout is only needed when op.Timeout != nil (already handled above).
		if isWatchEnabledForOp(op, g.watchEnabled) && g.flagsCfg.Watch.Enabled {
			needsTime = true
		}
		// Wait loop uses time.Now, time.Duration for deadline and sleep.
		if isWaitEnabledForOp(op, g.waitEnabled) && g.flagsCfg.Wait.Enabled {
			needsContext = true
			needsTime = true
		}
	}

	// needsSlices is true when any server variable defines an enum — we emit
	// slices.Contains validation in every operation RunE for those variables.
	needsSlices := false
	if len(reg.Servers) > 0 {
		for _, sv := range reg.Servers[0].Variables {
			if len(sv.Enum) > 0 {
				needsSlices = true
				break
			}
		}
	}

	var sb StringBuilder
	sb.Line("// Code generated by Cliford. DO NOT EDIT outside custom code regions.")
	sb.Line("package cli")
	sb.Line("")
	sb.Line("import (")
	if needsBytes {
		sb.Line(`	"bytes"`)
	}
	if needsContext {
		sb.Line(`	"context"`)
	}
	sb.Line(`	"encoding/json"`)
	sb.Line(`	"fmt"`)
	sb.Line(`	"io"`)
	sb.Line(`	"net/http"`)
	sb.Line(`	"net/url"`)
	sb.Line(`	"os"`)
	if needsSlices {
		sb.Line(`	"slices"`)
	}
	sb.Line(`	"strings"`)
	if needsContext || needsTime {
		sb.Line(`	"time"`)
	}
	sb.Line("")
	sb.Line(`	"github.com/spf13/cobra"`)
	if needsSDK {
		sb.Linef(`	"%s/internal/sdk"`, g.pkgPath)
	}
	sb.Line(")")
	sb.Line("")

	// Tag group command
	tagPrefix := toCamelCase(tag)
	sb.Linef("func %sCmd() *cobra.Command {", tagPrefix)
	sb.Line("	cmd := &cobra.Command{")
	sb.Linef("		Use:   %q,", strings.ToLower(tag))
	sb.Linef("		Short: %q,", tag+" operations")
	sb.Line("	}")
	sb.Line("")
	for _, op := range ops {
		sb.Linef("	cmd.AddCommand(%s%sCmd())", tagPrefix, toPascalCase(op.CLICommandName))
	}
	sb.Line("")
	sb.Line("	return cmd")
	sb.Line("}")
	sb.Line("")

	// Each operation command - prefixed with tag name to avoid collisions
	for _, op := range ops {
		g.generateOperationCmd(&sb, op, reg, tagPrefix)
	}

	return writeFormatted(filepath.Join(cliDir, toSnakeCase(tag)+".go"), sb.String())
}

func (g *Generator) generateOperationCmd(sb *StringBuilder, op registry.OperationMeta, reg *registry.Registry, tagPrefix string) {
	funcName := tagPrefix + toPascalCase(op.CLICommandName)

	// Determine effective agent output format for this operation.
	// Per-operation CLIAgentFormat (from x-cliford-cli or cliford.yaml operations) takes
	// priority over the global agentOutputFormat baked in at generation time.
	effectiveAgentFormat := g.agentOutputFormat
	if op.CLIAgentFormat != "" {
		effectiveAgentFormat = op.CLIAgentFormat
	}

	// Generator-time loop booleans: control which loop infrastructure is emitted.
	// watchLoop: emit watch flags + clear-screen header inside the loop
	// waitLoop:  emit wait flags + condition check inside the loop
	// emitLoop:  emit the for {} / closure wrapper at all
	watchLoop := isWatchEnabledForOp(op, g.watchEnabled) && g.flagsCfg.Watch.Enabled
	waitLoop := isWaitEnabledForOp(op, g.waitEnabled) && g.flagsCfg.Wait.Enabled
	emitLoop := watchLoop || waitLoop

	// Build existing param flag name set for collision detection
	existingParamFlags := make(map[string]bool)
	for _, p := range op.Parameters {
		existingParamFlags[p.FlagName] = true
	}
	// Also reserve "body" flag name to avoid collision with --body JSON flag
	if op.RequestBody != nil {
		existingParamFlags["body"] = true
	}

	// Determine if we have body properties to expand
	hasBodyProps := op.RequestBody != nil && len(op.RequestBody.Schema.Properties) > 0
	sortedProps := sortedSchemaKeys(func() map[string]registry.SchemaMeta {
		if op.RequestBody != nil {
			return op.RequestBody.Schema.Properties
		}
		return nil
	}())

	sb.Linef("func %sCmd() *cobra.Command {", funcName)

	// Flag variables for path/query/header params
	for _, p := range op.Parameters {
		goType := flagGoType(p.Schema)
		sb.Linef("	var flag%s %s", toPascalCase(p.FlagName), goType)
	}
	// Body: individual property flags or fallback JSON
	if hasBodyProps {
		for _, propName := range sortedProps {
			propSchema := op.RequestBody.Schema.Properties[propName]
			varName := "bodyProp" + toPascalCase(propName)
			sb.Linef("	var %s %s", varName, flagGoType(propSchema))
		}
	}
	if op.RequestBody != nil {
		sb.Line("	var bodyJSON string")
	}
	sb.Line("")

	sb.Line("	cmd := &cobra.Command{")
	sb.Linef("		Use:   %q,", op.CLICommandName)
	sb.Linef("		Short: %q,", op.Summary)
	if op.Description != "" {
		sb.Linef("		Long:  %q,", wrapText(op.Description, 80))
	}
	if len(op.CLIAliases) > 0 {
		sb.Linef("		Aliases: []string{%s},", quoteJoin(op.CLIAliases))
	}
	if op.CLIHidden {
		sb.Line("		Hidden: true,")
	}

	sb.Line("		RunE: func(cmd *cobra.Command, args []string) error {")

	// Prompt for missing required parameters (interactive TTY only)
	requiredParams := []registry.ParamMeta{}
	for _, p := range op.Parameters {
		if p.Required {
			requiredParams = append(requiredParams, p)
		}
	}
	if len(requiredParams) > 0 {
		sb.Line("			// Interactive prompts for missing required args")
		sb.Line("			if !noInteractive && !agentMode {")
		for _, p := range requiredParams {
			flagVar := "flag" + toPascalCase(p.FlagName)
			goType := flagGoType(p.Schema)
			if goType == "string" {
				sb.Linef("				if !cmd.Flags().Changed(%q) && %s == \"\" {", p.FlagName, flagVar)
				sb.Linef("					%s = promptValue(%q)", flagVar, p.FlagName+": ")
				sb.Linef("					if %s == \"\" {", flagVar)
				sb.Linef("						return fmt.Errorf(\"required flag --%s not provided\")", p.FlagName)
				sb.Line("					}")
				sb.Line("				}")
			}
		}
		sb.Line("			}")
		sb.Line("")
	}

	// Confirmation prompt for destructive operations
	needsConfirm := op.CLIConfirm || op.Method == "DELETE"
	if needsConfirm {
		confirmMsg := op.CLIConfirmMsg
		if confirmMsg == "" {
			confirmMsg = fmt.Sprintf("Are you sure you want to %s?", op.CLICommandName)
		}
		sb.Line("			// Confirm destructive operation")
		sb.Line("			if !yesMode {")
		sb.Linef("				answer := promptValue(%q)", confirmMsg+" [y/N]: ")
		sb.Line("				if answer != \"y\" && answer != \"Y\" && answer != \"yes\" {")
		sb.Line(`					fmt.Fprintln(os.Stderr, "Aborted.")`)
		sb.Line("					return nil")
		sb.Line("				}")
		sb.Line("			}")
		sb.Line("")
	}

	// Apply per-operation timeout via context if configured.
	// For loop-enabled ops (watch or wait) the context is created per-iteration inside the closure.
	if op.Timeout != nil && !emitLoop {
		sb.Linef("			ctx, cancel := context.WithTimeout(cmd.Context(), %d*time.Nanosecond)", op.Timeout.Nanoseconds())
		sb.Line("			defer cancel()")
	}
	sb.Line("")

	// Watch mode setup vars — emitted before URL building so they are in scope for the loop.
	if watchLoop {
		sb.Line("			// Watch/poll mode setup")
		sb.Line(`			watchMode_, _ := cmd.Flags().GetBool("watch")`)
		sb.Line(`			watchIntervalStr_, _ := cmd.Flags().GetString("poll-interval")`)
		sb.Line(`			watchMaxCount_, _ := cmd.Flags().GetInt("watch-count")`)
		sb.Line("			// --poll-interval alone implies --watch")
		sb.Line(`			if cmd.Flags().Changed("poll-interval") && !cmd.Flags().Changed("watch") {`)
		sb.Line("				watchMode_ = true")
		sb.Line("			}")
		sb.Line("			watchIter_ := 0")
		sb.Line("")
	}

	// Pre-compute wait generation-time values (used in both setup vars and loop-close blocks).
	var waitIntervalNs int64 = 15_000_000_000 // default 15s
	var waitTimeoutDefault string
	var waitBakedCondition string
	var waitBakedErrorCondition string
	var waitBakedMessage string
	if waitLoop {
		waitIntervalStr := op.CLIWaitInterval
		if waitIntervalStr == "" {
			waitIntervalStr = g.waitInterval
		}
		if waitIntervalStr == "" {
			waitIntervalStr = "15s"
		}
		if d, err := time.ParseDuration(waitIntervalStr); err == nil {
			waitIntervalNs = d.Nanoseconds()
		}
		waitTimeoutDefault = op.CLIWaitTimeout
		if waitTimeoutDefault == "" {
			waitTimeoutDefault = g.waitTimeout
		}
		waitBakedCondition = op.CLIWaitCondition
		waitBakedErrorCondition = op.CLIWaitErrorCondition
		waitBakedMessage = op.CLIWaitMessage
	}

	// Wait mode setup vars — emitted before the loop so they are in scope for the closure.
	if waitLoop {
		sb.Line("			// Wait mode setup")
		sb.Line(`			waitMode_, _ := cmd.Flags().GetBool("wait")`)
		sb.Line(`			waitForExpr_, _ := cmd.Flags().GetString("wait-for")`)
		sb.Line(`			waitTimeoutStr_, _ := cmd.Flags().GetString("wait-timeout")`)
		sb.Line("			// --wait-for alone implies --wait")
		sb.Line(`			if cmd.Flags().Changed("wait-for") && !cmd.Flags().Changed("wait") {`)
		sb.Line("				waitMode_ = true")
		sb.Line("			}")
		// Bake in the default condition if one is configured
		if waitBakedCondition != "" {
			sb.Linef(`			if waitForExpr_ == "" { waitForExpr_ = %q }`, waitBakedCondition)
		}
		// Validate: --wait without any condition is an error
		sb.Line(`			if waitMode_ && waitForExpr_ == "" {`)
		sb.Line(`				return fmt.Errorf("--wait requires a condition expression; use --wait-for to specify one, or configure x-cliford-wait.condition in the spec")`)
		sb.Line("			}")
		// Compute deadline
		sb.Line("			var waitDeadline_ time.Time")
		if waitTimeoutDefault != "" {
			sb.Linef(`			waitTimeoutDefault_ := %q`, waitTimeoutDefault)
		} else {
			sb.Line(`			waitTimeoutDefault_ := ""`)
		}
		sb.Line(`			if waitTimeoutStr_ == "" { waitTimeoutStr_ = waitTimeoutDefault_ }`)
		sb.Line("			if waitMode_ && waitTimeoutStr_ != \"\" {")
		sb.Line("				if d_, err_ := time.ParseDuration(waitTimeoutStr_); err_ == nil && d_ > 0 {")
		sb.Line("					waitDeadline_ = time.Now().Add(d_)")
		sb.Line("				}")
		sb.Line("			}")
		sb.Line("			waitDone_ := false")
		sb.Line("")
	}

	// Determine base URL
	defaultURL := "http://localhost:8080"
	if len(reg.Servers) > 0 {
		defaultURL = reg.Servers[0].URL
	}
	sb.Linef("			baseURL := %q", defaultURL)
	sb.Line("			if serverURL != \"\" {")
	sb.Line("				baseURL = serverURL")
	sb.Line("			} else {")
	if len(reg.Servers) > 0 && len(reg.Servers[0].Variables) > 0 {
		sb.Line("				// Apply server URL template variable substitution")
		sb.Line("				baseURL = strings.NewReplacer(")
		varNames := sortedStringKeys(reg.Servers[0].Variables)
		for _, varName := range varNames {
			goVar := "serverVar" + toPascalCase(varName)
			sb.Linef("					%q, %s,", "{"+varName+"}", goVar)
		}
		sb.Line("				).Replace(baseURL)")
	}
	sb.Line("			}")
	sb.Line("")

	// Enum validation for server URL template variables
	if len(reg.Servers) > 0 && len(reg.Servers[0].Variables) > 0 {
		varNames := sortedStringKeys(reg.Servers[0].Variables)
		for _, varName := range varNames {
			sv := reg.Servers[0].Variables[varName]
			if len(sv.Enum) > 0 {
				goVar := "serverVar" + toPascalCase(varName)
				flagName := "server-" + toKebabCase(varName)
				sb.Linef("			if !slices.Contains([]string{%s}, %s) {", quoteJoin(sv.Enum), goVar)
				sb.Linef("				return fmt.Errorf(\"invalid --%s value %%q: allowed values are %v\", %s)", flagName, sv.Enum, goVar)
				sb.Line("			}")
		}
		}
		sb.Line("")
	}

	// Build path with parameter substitution
	sb.Linef("			reqPath := %q", op.Path)
	for _, p := range op.Parameters {
		if p.In == registry.ParamLocationPath {
			sb.Linef(`			reqPath = strings.Replace(reqPath, "{%s}", fmt.Sprintf("%%v", flag%s), 1)`, p.Name, toPascalCase(p.FlagName))
		}
	}
	sb.Line("")

	sb.Line("			reqURL, err := url.Parse(baseURL + reqPath)")
	sb.Line("			if err != nil {")
	sb.Line(`				return fmt.Errorf("invalid URL: %w", err)`)
	sb.Line("			}")
	sb.Line("")

	// Query parameters
	hasQuery := false
	for _, p := range op.Parameters {
		if p.In == registry.ParamLocationQuery {
			hasQuery = true
			break
		}
	}
	if hasQuery {
		sb.Line("			q := reqURL.Query()")
		for _, p := range op.Parameters {
			if p.In != registry.ParamLocationQuery {
				continue
			}
			goType := flagGoType(p.Schema)
			flagVar := "flag" + toPascalCase(p.FlagName)
			switch goType {
			case "string":
				sb.Linef("			if %s != \"\" {", flagVar)
				sb.Linef("				q.Set(%q, %s)", p.Name, flagVar)
				sb.Line("			}")
			case "int", "int64":
				sb.Linef("			if cmd.Flags().Changed(%q) {", p.FlagName)
				sb.Linef("				q.Set(%q, fmt.Sprintf(\"%%d\", %s))", p.Name, flagVar)
				sb.Line("			}")
			case "bool":
				sb.Linef("			if cmd.Flags().Changed(%q) {", p.FlagName)
				sb.Linef("				q.Set(%q, fmt.Sprintf(\"%%t\", %s))", p.Name, flagVar)
				sb.Line("			}")
			}
		}
		sb.Line("			reqURL.RawQuery = q.Encode()")
		sb.Line("")
	}

	// Request body
	sb.Line("			var reqBody io.Reader")
	if hasBodyProps {
		// Merge stdin → --body → individual flags, then validate
		sb.Line("			{")
		sb.Line("				bodyFields := make(map[string]any)")
		sb.Line("				// stdin base layer")
		sb.Line("				{")
		sb.Line("					stat, _ := os.Stdin.Stat()")
		sb.Line("					if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {")
		sb.Line("						if data, err := io.ReadAll(os.Stdin); err == nil && len(data) > 0 {")
		sb.Line("							_ = json.Unmarshal(data, &bodyFields)")
		sb.Line("						}")
		sb.Line("					}")
		sb.Line("				}")
		sb.Line("				// --body JSON override")
		sb.Line("				if bodyJSON != \"\" {")
		sb.Line("					var override map[string]any")
		sb.Line("					if err := json.Unmarshal([]byte(bodyJSON), &override); err != nil {")
		sb.Line(`					    return fmt.Errorf("invalid --body JSON: %w", err)`)
		sb.Line("					}")
		sb.Line("					for k, v := range override {")
		sb.Line("						bodyFields[k] = v")
		sb.Line("					}")
		sb.Line("				}")
		sb.Line("				// individual flags (highest priority)")
		for _, propName := range sortedProps {
			propSchema := op.RequestBody.Schema.Properties[propName]
			flagName := bodyPropFlagName(propName, existingParamFlags)
			varName := "bodyProp" + toPascalCase(propName)
			if len(propSchema.Enum) > 0 {
				enumVals := enumStrings(propSchema.Enum)
				sb.Linef("				if cmd.Flags().Changed(%q) {", flagName)
				sb.Linef("					valid%s := []string{%s}", toPascalCase(propName), quoteJoin(enumVals))
				sb.Line("					found := false")
				sb.Linef("					for _, v := range valid%s {", toPascalCase(propName))
				sb.Linef("						if v == %s { found = true; break }", varName)
				sb.Line("					}")
				sb.Line("					if !found {")
				sb.Linef("						return fmt.Errorf(%q+\": must be one of: %%s\", strings.Join(valid%s, \", \"))", propName, toPascalCase(propName))
				sb.Line("					}")
				sb.Linef("					bodyFields[%q] = %s", propName, varName)
				sb.Line("				}")
			} else {
				sb.Linef("				if cmd.Flags().Changed(%q) { bodyFields[%q] = %s }", flagName, propName, varName)
			}
		}
		// Validate required fields after merge
		if len(op.RequestBody.Schema.Required) > 0 {
			sb.Line("				// validate required fields")
			for _, reqProp := range op.RequestBody.Schema.Required {
				flagName := bodyPropFlagName(reqProp, existingParamFlags)
				sb.Linef("				if _, ok := bodyFields[%q]; !ok {", reqProp)
				sb.Linef("					return fmt.Errorf(\"required field --%s is missing; use --%s or --body\")", flagName, flagName)
				sb.Line("				}")
			}
		}
		sb.Line("				if len(bodyFields) > 0 {")
		sb.Line("					b, err := json.Marshal(bodyFields)")
		sb.Line("					if err != nil {")
		sb.Line(`						return fmt.Errorf("marshal body: %w", err)`)
		sb.Line("					}")
		sb.Line("					reqBody = bytes.NewReader(b)")
		sb.Line("				}")
		sb.Line("			}")
	} else if op.RequestBody != nil {
		// No schema properties: accept raw --body JSON or stdin
		sb.Line("			if bodyJSON != \"\" {")
		sb.Line("				reqBody = strings.NewReader(bodyJSON)")
		sb.Line("			} else {")
		sb.Line("				stat, _ := os.Stdin.Stat()")
		sb.Line("				if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {")
		sb.Line("					data, err := io.ReadAll(os.Stdin)")
		sb.Line("					if err == nil && len(data) > 0 {")
		sb.Line("						reqBody = bytes.NewReader(data)")
		sb.Line("					}")
		sb.Line("				}")
		sb.Line("			}")
	}
	sb.Line("")

	// For watch/wait-enabled ops: wrap the request-execute-display block in a for loop.
	// The loop runs once for non-watch/wait mode, repeatedly otherwise.
	// An immediately-invoked closure captures the iteration body so existing return
	// statements work correctly; the outer loop handles continue vs. stop logic.
	if emitLoop {
		sb.Line("			for {")
		sb.Line("				// Clear screen and print watch header on each update (TTY only).")
		sb.Line("				if watchMode_ && watchIter_ > 0 && isTTYStdout_() && !agentMode && !noInteractive {")
		sb.Line(`					fmt.Print("\033[2J\033[H")`)
		sb.Line("				}")
		sb.Line("				if watchMode_ && isTTYStdout_() && !agentMode && !noInteractive {")
		sb.Linef(`					fmt.Fprintf(os.Stdout, "Every %%s: %%s  %%s\n\n", watchIntervalStr_, strings.Join(os.Args[1:], " "), time.Now().Format("Mon Jan  2 15:04:05 2006"))`)
		sb.Line("				}")
		sb.Line("				iterErr_ := func() error {")
		// Per-iteration context: timeout (if configured) or root context.
		if op.Timeout != nil {
			sb.Linef("					ctx, cancel := context.WithTimeout(cmd.Context(), %d*time.Nanosecond)", op.Timeout.Nanoseconds())
			sb.Line("					defer cancel()")
		} else {
			sb.Line("					ctx := cmd.Context()")
		}
		sb.Line("")
	}

	// Create request with context.
	// Loop-enabled ops (watch/wait): always use ctx (defined per-iteration inside closure above).
	// Non-loop ops with timeout: use ctx (defined at top of RunE).
	// Non-loop ops without timeout: use cmd.Context() directly.
	if emitLoop {
		sb.Linef("			req, err := http.NewRequestWithContext(ctx, %q, reqURL.String(), reqBody)", op.Method)
	} else if op.Timeout != nil {
		sb.Linef("			req, err := http.NewRequestWithContext(ctx, %q, reqURL.String(), reqBody)", op.Method)
	} else {
		sb.Linef("			req, err := http.NewRequestWithContext(cmd.Context(), %q, reqURL.String(), reqBody)", op.Method)
	}
	sb.Line("			if err != nil {")
	sb.Line(`				return fmt.Errorf("create request: %w", err)`)
	sb.Line("			}")

	// Header parameters
	for _, p := range op.Parameters {
		if p.In == registry.ParamLocationHeader {
			flagVar := "flag" + toPascalCase(p.FlagName)
			sb.Linef("			if %s != \"\" {", flagVar)
			sb.Linef("				req.Header.Set(%q, %s)", p.Name, flagVar)
			sb.Line("			}")
		}
	}
	if op.RequestBody != nil {
		sb.Line("			if reqBody != nil {")
		sb.Line(`				req.Header.Set("Content-Type", "application/json")`)
		sb.Line("			}")
	}

	// Request ID injection — generate a UUID and attach it as a header for correlation.
	if op.CLIRequestID {
		header := g.requestIDHeader
		if header == "" {
			header = "X-Request-ID"
		}
		sb.Line("")
		sb.Line("			// Attach request ID for server-side log correlation.")
		sb.Line("			requestID := generateRequestID()")
		sb.Linef("			req.Header.Set(%q, requestID)", header)
	}
	sb.Line("")

	// Apply per-request retry overrides from CLI flags
	if g.pkgPath != "" && g.flagsCfg.Retries.Enabled {
		sb.Line("			// Apply per-request retry overrides from CLI flags")
		sb.Line("			if cmd.Flags().Changed(\"no-retries\") || cmd.Flags().Changed(\"retry-max-attempts\") || cmd.Flags().Changed(\"retry-max-elapsed\") {")
		sb.Line("				override := sdk.RetryOverride{}")
		sb.Line("				noRetries, _ := cmd.Flags().GetBool(\"no-retries\")")
		sb.Line("				override.Disabled = noRetries")
		sb.Line("				if maxAttempts, _ := cmd.Flags().GetInt(\"retry-max-attempts\"); maxAttempts > 0 {")
		sb.Line("					override.MaxAttempts = maxAttempts")
		sb.Line("				}")
		sb.Line("				if maxElapsedStr, _ := cmd.Flags().GetString(\"retry-max-elapsed\"); maxElapsedStr != \"\" {")
		sb.Line("					if d, err := time.ParseDuration(maxElapsedStr); err == nil {")
		sb.Line("						override.MaxElapsedTime = d")
		sb.Line("					}")
		sb.Line("				}")
		sb.Line("				req = req.WithContext(sdk.WithRetryOverride(req.Context(), override))")
		sb.Line("			}")
		sb.Line("")
	}

	// Custom code region: pre-request
	if g.customCodeRegions {
		sb.Linef("			// --- CUSTOM CODE START: %s:pre ---", op.OperationID)
		sb.Linef("			// --- CUSTOM CODE END: %s:pre ---", op.OperationID)
		sb.Line("")
	}

	// Dry-run
	sb.Line("			if dryRunMode {")
	sb.Line(`				fmt.Fprintf(os.Stdout, "%s %s\n", req.Method, req.URL.String())`)
	sb.Line("				for k, v := range req.Header {")
	sb.Line(`					fmt.Fprintf(os.Stdout, "%s: %s\n", k, strings.Join(v, ", "))`)
	sb.Line("				}")
	if op.RequestBody != nil {
		sb.Line("				if req.Body != nil {")
		sb.Line(`					fmt.Fprintln(os.Stdout)`)
		sb.Line("					bodyBytes, _ := io.ReadAll(req.Body)")
		sb.Line("					req.Body.Close()")
		sb.Line("					os.Stdout.Write(bodyBytes)")
		sb.Line(`					fmt.Fprintln(os.Stdout)`)
		sb.Line("				}")
	}
	sb.Line("				return nil")
	sb.Line("			}")
	sb.Line("")

	// Pagination: --all fetch loop (early return path for paginated operations)
	if op.Pagination != nil && op.Pagination.OutputResults != "" {
		sb.Line(`			fetchAll, _ := cmd.Flags().GetBool("all")`)
		sb.Line(`			maxPages, _ := cmd.Flags().GetInt("max-pages")`)
		sb.Line("			if fetchAll {")
		sb.Line("				if maxPages <= 0 { maxPages = 1000 }")
		sb.Line("				var allResults []any")
		sb.Line("				cursor := \"\"")
		sb.Line("				for page := 0; page < maxPages; page++ {")
		sb.Line("					pageReq := req.Clone(req.Context())")
		sb.Line("					if cursor != \"\" {")
		sb.Linef("						q := pageReq.URL.Query()")
		sb.Linef("						q.Set(%q, cursor)", op.Pagination.InputCursor.Name)
		sb.Line("						pageReq.URL.RawQuery = q.Encode()")
		sb.Line("					}")
		sb.Line("					resp, err := GetAPIClient().Do(pageReq)")
		sb.Line("					if err != nil {")
		if op.CLIRequestID {
			sb.Line(`						return fmt.Errorf("request failed on page %d (request-id: %s): %w", page+1, requestID, err)`)
		} else {
			sb.Line(`						return fmt.Errorf("request failed on page %d: %w", page+1, err)`)
		}
		sb.Line("					}")
		sb.Line("					body, _ := io.ReadAll(resp.Body)")
		sb.Line("					resp.Body.Close()")
		sb.Line("					if resp.StatusCode >= 400 {")
		if op.CLIRequestID {
			sb.Line(`						return fmt.Errorf("HTTP %d (request-id: %s): %s", resp.StatusCode, requestID, string(body))`)
		} else {
			sb.Line(`						return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))`)
		}
		sb.Line("					}")
		sb.Line("					var pageData map[string]any")
		sb.Line("					if err := json.Unmarshal(body, &pageData); err != nil {")
		sb.Line("						break")
		sb.Line("					}")
		// Extract results array
		resultField := strings.TrimPrefix(op.Pagination.OutputResults, "$.")
		sb.Linef("					if items, ok := pageData[%q]; ok {", resultField)
		sb.Line("						if arr, ok := items.([]any); ok {")
		sb.Line("							allResults = append(allResults, arr...)")
		sb.Line("						}")
		sb.Line("					}")
		// Extract next cursor
		if op.Pagination.OutputNextKey != "" {
			nextField := strings.TrimPrefix(op.Pagination.OutputNextKey, "$.")
			sb.Linef("					if next, ok := pageData[%q]; ok && next != nil {", nextField)
			sb.Line("						if s, ok := next.(string); ok && s != \"\" {")
			sb.Line("							cursor = s")
			sb.Line("							continue")
			sb.Line("						}")
			sb.Line("					}")
		}
		sb.Line("					break // No more pages")
		sb.Line("				}")
		hasAgentFmtPag := effectiveAgentFormat != ""
		hasOpFmtPag := op.CLIDefaultOutputFormat != ""
		switch {
		case hasAgentFmtPag && hasOpFmtPag:
			sb.Line("				{")
			sb.Linef("					agentFmt := %q", effectiveAgentFormat)
			sb.Linef("					opFmt := %q", op.CLIDefaultOutputFormat)
			sb.Line(`					if agentMode && !cmd.Root().PersistentFlags().Changed("output-format") {`)
			sb.Line("						return FormatOutput(allResults, agentFmt)")
			sb.Line("					}")
			sb.Line(`					if !cmd.Root().PersistentFlags().Changed("output-format") {`)
			sb.Line("						return FormatOutput(allResults, opFmt)")
			sb.Line("					}")
			sb.Line("					return FormatOutput(allResults, outputFormat)")
			sb.Line("				}")
		case hasAgentFmtPag:
			sb.Line("				{")
			sb.Linef("					agentFmt := %q", effectiveAgentFormat)
			sb.Line(`					if agentMode && !cmd.Root().PersistentFlags().Changed("output-format") {`)
			sb.Line("						return FormatOutput(allResults, agentFmt)")
			sb.Line("					}")
			sb.Line("					return FormatOutput(allResults, outputFormat)")
			sb.Line("				}")
		case hasOpFmtPag:
			sb.Line("				{")
			sb.Linef("					opFmt := %q", op.CLIDefaultOutputFormat)
			sb.Line(`					if !cmd.Root().PersistentFlags().Changed("output-format") {`)
			sb.Line("						return FormatOutput(allResults, opFmt)")
			sb.Line("					}")
			sb.Line("					return FormatOutput(allResults, outputFormat)")
			sb.Line("				}")
		default:
			sb.Line("				return FormatOutput(allResults, outputFormat)")
		}
		sb.Line("			}")
		sb.Line("")
	}

	// Execute using shared API client (pre-configured with auth/retry/verbose transports)
	sb.Line("			resp, err := withSpinner(\"Loading...\", func() (*http.Response, error) {")
	sb.Line("				return GetAPIClient().Do(req)")
	sb.Line("			})")
	sb.Line("			if err != nil {")
	if op.CLIRequestID {
		sb.Line(`				return fmt.Errorf("request failed (request-id: %s): %w", requestID, err)`)
	} else {
		sb.Line(`				return fmt.Errorf("request failed: %w", err)`)
	}
	sb.Line("			}")
	sb.Line("			defer resp.Body.Close()")
	sb.Line("")
	sb.Line("			if outputFile != \"\" {")
	sb.Line("				if resp.StatusCode >= 400 {")
	sb.Line("					errBody, _ := io.ReadAll(resp.Body)")
	if op.CLIRequestID {
		sb.Line(`					return fmt.Errorf("HTTP %d (request-id: %s): %s", resp.StatusCode, requestID, string(errBody))`)
	} else {
		sb.Line(`					return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errBody))`)
	}
	sb.Line("				}")
	sb.Line("				if includeHeaders {")
	sb.Line("					for k, vs := range resp.Header {")
	sb.Line(`						fmt.Fprintf(os.Stderr, "%s: %s\n", k, strings.Join(vs, ", "))`)
	sb.Line("					}")
	sb.Line(`					fmt.Fprintln(os.Stderr)`)
	sb.Line("				}")
	sb.Line(`				return writeFileWithProgress(resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"), outputFile)`)
	sb.Line("			}")
	sb.Line("")
	sb.Line("			respBody, err := io.ReadAll(resp.Body)")
	sb.Line("			if err != nil {")
	sb.Line(`				return fmt.Errorf("read response: %w", err)`)
	sb.Line("			}")
	sb.Line("")

	// Handle errors
	sb.Line("			if resp.StatusCode >= 400 {")
	if op.CLIRequestID {
		sb.Line(`				return fmt.Errorf("HTTP %d (request-id: %s): %s", resp.StatusCode, requestID, string(respBody))`)
	} else {
		sb.Line(`				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))`)
	}
	sb.Line("			}")
	sb.Line("")

	// Handle 204 No Content
	sb.Line("			if resp.StatusCode == 204 || len(respBody) == 0 {")
	sb.Line(`				fmt.Println("OK")`)
	sb.Line("				return nil")
	sb.Line("			}")
	sb.Line("")

	// Format output
	sb.Line("			var data any")
	sb.Line("			if err := json.Unmarshal(respBody, &data); err != nil {")
	sb.Line("				if includeHeaders {")
	sb.Line("					for k, vs := range resp.Header {")
	sb.Line(`						fmt.Fprintf(os.Stdout, "%s: %s\n", k, strings.Join(vs, ", "))`)
	sb.Line("					}")
	sb.Line(`					fmt.Println()`)
	sb.Line("				}")
	sb.Line("				fmt.Println(string(respBody))")
	sb.Line("				return nil")
	sb.Line("			}")
	// Custom code region: post-response
	if g.customCodeRegions {
		sb.Line("")
		sb.Linef("			// --- CUSTOM CODE START: %s:post ---", op.OperationID)
		sb.Linef("			// --- CUSTOM CODE END: %s:post ---", op.OperationID)
	}

	// Wait condition check — evaluated against the raw parsed response (before jq filtering)
	// so that API shape assumptions in conditions are stable regardless of user display flags.
	if waitLoop {
		sb.Line("")
		sb.Line("			// Wait: evaluate error and success conditions against the raw parsed response.")
		// Error condition: baked in, not user-overridable (safety net)
		if waitBakedErrorCondition != "" {
			sb.Linef(`			if met_, _ := evalWaitCondition_(data, %q); met_ {`, waitBakedErrorCondition)
			sb.Linef(`				return fmt.Errorf("wait: error condition met: %s", %q)`, waitBakedErrorCondition, waitBakedErrorCondition)
			sb.Line("			}")
		}
		// Success condition
		sb.Line("			if waitMode_ {")
		sb.Line("				met_, evalErr_ := evalWaitCondition_(data, waitForExpr_)")
		sb.Line("				if evalErr_ != nil {")
		sb.Line(`					return fmt.Errorf("wait: condition evaluation failed: %w", evalErr_)`)
		sb.Line("				}")
		sb.Line("				if !met_ {")
		// Print message in wait-only mode (not when watch is also active)
		if waitBakedMessage != "" {
			if watchLoop {
				// watchMode_ is declared — guard so message only prints in wait-only mode.
				sb.Line("					if !watchMode_ {")
				sb.Linef(`						fmt.Fprintf(os.Stderr, %q+"\n")`, waitBakedMessage)
				sb.Line("					}")
			} else {
				// wait-only op: watchMode_ is undeclared, print unconditionally.
				sb.Linef(`					fmt.Fprintf(os.Stderr, %q+"\n")`, waitBakedMessage)
			}
		}
		sb.Line("					return errWaitNotYet_")
		sb.Line("				}")
		sb.Line("				waitDone_ = true")
		sb.Line("			}")
		sb.Line("")
	}

	// Emit jq filter block — simple form (no default) or effectiveJQ form (with baked default)
	if op.CLIDefaultJQ == "" {
		sb.Line("			if jqFilter != \"\" {")
		sb.Line("				filtered, jqErr := applyJQ(data, jqFilter)")
		sb.Line("				if jqErr != nil {")
		sb.Line(`					return fmt.Errorf("--jq: %w", jqErr)`)
		sb.Line("				}")
		sb.Line("				data = filtered")
		sb.Line("			}")
	} else {
		sb.Line("			{")
		sb.Line("				effectiveJQ := jqFilter")
		sb.Line("				if effectiveJQ == \"\" {")
		sb.Linef("					effectiveJQ = %q", op.CLIDefaultJQ)
		sb.Line("				}")
		sb.Line("				filtered, jqErr := applyJQ(data, effectiveJQ)")
		sb.Line("				if jqErr != nil {")
		sb.Line(`					return fmt.Errorf("--jq: %w", jqErr)`)
		sb.Line("				}")
		sb.Line("				data = filtered")
		sb.Line("			}")
	}
	sb.Line("			if includeHeaders {")
	sb.Line("				hdrs := make(map[string]any)")
	sb.Line("				for k, vs := range resp.Header {")
	sb.Line("					if len(vs) == 1 {")
	sb.Line("						hdrs[k] = vs[0]")
	sb.Line("					} else {")
	sb.Line("						hdrs[k] = vs")
	sb.Line("					}")
	sb.Line("				}")
	sb.Line("				data = map[string]any{\"headers\": hdrs, \"body\": data}")
	sb.Line("			}")
	hasAgentFmt := effectiveAgentFormat != ""
	hasOpFmt := op.CLIDefaultOutputFormat != ""
	switch {
	case hasAgentFmt && hasOpFmt:
		sb.Line("			{")
		sb.Linef("				agentFmt := %q", effectiveAgentFormat)
		sb.Linef("				opFmt := %q", op.CLIDefaultOutputFormat)
		sb.Line(`				if agentMode && !cmd.Root().PersistentFlags().Changed("output-format") {`)
		sb.Line("					return FormatOutput(data, agentFmt)")
		sb.Line("				}")
		sb.Line(`				if !cmd.Root().PersistentFlags().Changed("output-format") {`)
		sb.Line("					return FormatOutput(data, opFmt)")
		sb.Line("				}")
		sb.Line("				return FormatOutput(data, outputFormat)")
		sb.Line("			}")
	case hasAgentFmt:
		sb.Line("			{")
		sb.Linef("				agentFmt := %q", effectiveAgentFormat)
		sb.Line(`				if agentMode && !cmd.Root().PersistentFlags().Changed("output-format") {`)
		sb.Line("					return FormatOutput(data, agentFmt)")
		sb.Line("				}")
		sb.Line("				return FormatOutput(data, outputFormat)")
		sb.Line("			}")
	case hasOpFmt:
		sb.Line("			{")
		sb.Linef("				opFmt := %q", op.CLIDefaultOutputFormat)
		sb.Line(`				if !cmd.Root().PersistentFlags().Changed("output-format") {`)
		sb.Line("					return FormatOutput(data, opFmt)")
		sb.Line("				}")
		sb.Line("				return FormatOutput(data, outputFormat)")
		sb.Line("			}")
	default:
		sb.Line("			return FormatOutput(data, outputFormat)")
	}

	// Close the immediately-invoked closure and the for loop.
	// Three cases based on which loop modes are enabled for this operation.
	if emitLoop {
		sb.Line("			return nil")
		sb.Line("			}()")

		switch {
		case watchLoop && !waitLoop:
			// Case A: watch-only (original behavior, unchanged)
			sb.Line("			if iterErr_ != nil {")
			sb.Line(`				if watchMode_ { fmt.Fprintf(os.Stderr, "error: %v\n", iterErr_) } else { return iterErr_ }`)
			sb.Line("			}")
			sb.Line("			// Exit after one iteration when not in watch mode, or when --dry-run was used.")
			sb.Line("			if !watchMode_ || dryRunMode { break }")
			sb.Line("			watchIter_++")
			sb.Line("			if watchMaxCount_ > 0 && watchIter_ >= watchMaxCount_ { break }")
			sb.Line("			watchDuration_, dErr_ := time.ParseDuration(watchIntervalStr_)")
			sb.Line("			if dErr_ != nil { watchDuration_ = 5 * time.Second }")
			sb.Line("			if !watchSleep_(cmd.Context(), watchDuration_) { break }")

		case !watchLoop && waitLoop:
			// Case B: wait-only
			sb.Line("			if iterErr_ == errWaitNotYet_ {")
			sb.Line("				// condition not yet met — continue polling")
			sb.Line("			} else if iterErr_ != nil {")
			sb.Line(`				if waitMode_ { fmt.Fprintf(os.Stderr, "error: %v\n", iterErr_) } else { return iterErr_ }`)
			sb.Line("			}")
			sb.Line("			// Exit after one iteration when not in wait mode, or when --dry-run was used.")
			sb.Line("			if !waitMode_ || dryRunMode { break }")
			sb.Line("			// Exit when wait condition was met (iterErr_ == nil and waitDone_ == true).")
			sb.Line("			if waitDone_ { break }")
			sb.Line("			// Timeout check.")
			sb.Line("			if !waitDeadline_.IsZero() && time.Now().After(waitDeadline_) {")
			sb.Line(`				return fmt.Errorf("wait: timed out after %s", waitTimeoutStr_)`)
			sb.Line("			}")
			sb.Linef("			if !watchSleep_(cmd.Context(), %d*time.Nanosecond) { break }", waitIntervalNs)

		case watchLoop && waitLoop:
			// Case C: both watch and wait enabled
			sb.Line("			if iterErr_ == errWaitNotYet_ {")
			sb.Line("				// condition not yet met — continue polling")
			sb.Line("			} else if iterErr_ != nil {")
			sb.Line(`				if watchMode_ || waitMode_ { fmt.Fprintf(os.Stderr, "error: %v\n", iterErr_) } else { return iterErr_ }`)
			sb.Line("			}")
			sb.Line("			// Exit after one iteration when not in watch/wait mode, or when --dry-run was used.")
			sb.Line("			if (!watchMode_ && !waitMode_) || dryRunMode { break }")
			sb.Line("			// Exit when wait condition was met.")
			sb.Line("			if waitMode_ && waitDone_ { break }")
			sb.Line("			watchIter_++")
			sb.Line("			if watchMode_ && watchMaxCount_ > 0 && watchIter_ >= watchMaxCount_ { break }")
			sb.Line("			// Timeout check.")
			sb.Line("			if waitMode_ && !waitDeadline_.IsZero() && time.Now().After(waitDeadline_) {")
			sb.Line(`				return fmt.Errorf("wait: timed out after %s", waitTimeoutStr_)`)
			sb.Line("			}")
			sb.Line("			// Use watch interval when in watch mode; use baked wait interval otherwise.")
			sb.Line("			if watchMode_ {")
			sb.Line("				watchDuration_, dErr_ := time.ParseDuration(watchIntervalStr_)")
			sb.Line("				if dErr_ != nil { watchDuration_ = 5 * time.Second }")
			sb.Line("				if !watchSleep_(cmd.Context(), watchDuration_) { break }")
			sb.Line("			} else {")
			sb.Linef("				if !watchSleep_(cmd.Context(), %d*time.Nanosecond) { break }", waitIntervalNs)
			sb.Line("			}")
		}

		sb.Line("		}") // end for
		sb.Line("		return nil")
	}

	sb.Line("		},")
	sb.Line("	}")
	sb.Line("")

	// Register path/query/header flags
	for _, p := range op.Parameters {
		goType := flagGoType(p.Schema)
		flagVar := "flag" + toPascalCase(p.FlagName)
		desc := p.Description
		if len(p.Enum) > 0 {
			var vals []string
			for _, e := range p.Enum {
				vals = append(vals, fmt.Sprintf("%v", e))
			}
			desc += " (" + strings.Join(vals, ", ") + ")"
		}

		switch goType {
		case "string":
			defVal := ""
			if p.Default != nil {
				defVal = fmt.Sprintf("%v", p.Default)
			}
			sb.Linef("	cmd.Flags().StringVar(&%s, %q, %q, %q)", flagVar, p.FlagName, defVal, desc)
		case "int":
			defVal := 0
			if p.Default != nil {
				if f, ok := p.Default.(float64); ok {
					defVal = int(f)
				}
			}
			sb.Linef("	cmd.Flags().IntVar(&%s, %q, %d, %q)", flagVar, p.FlagName, defVal, desc)
		case "int64":
			defVal := int64(0)
			if p.Default != nil {
				if f, ok := p.Default.(float64); ok {
					defVal = int64(f)
				}
			}
			sb.Linef("	cmd.Flags().Int64Var(&%s, %q, %d, %q)", flagVar, p.FlagName, defVal, desc)
		case "bool":
			sb.Linef("	cmd.Flags().BoolVar(&%s, %q, false, %q)", flagVar, p.FlagName, desc)
		case "[]string":
			sb.Linef("	cmd.Flags().StringSliceVar(&%s, %q, nil, %q)", flagVar, p.FlagName, desc)
		case "float64":
			sb.Linef("	cmd.Flags().Float64Var(&%s, %q, 0, %q)", flagVar, p.FlagName, desc)
		}

		if p.Required {
			sb.Linef("	_ = cmd.MarkFlagRequired(%q)", p.FlagName)
		}
	}

	// Register body property flags (individual)
	if hasBodyProps {
		sb.Line("")
		// Build required set
		requiredSet := make(map[string]bool)
		for _, r := range op.RequestBody.Schema.Required {
			requiredSet[r] = true
		}
		for _, propName := range sortedProps {
			propSchema := op.RequestBody.Schema.Properties[propName]
			flagName := bodyPropFlagName(propName, existingParamFlags)
			varName := "bodyProp" + toPascalCase(propName)
			desc := propSchema.Description
			if len(propSchema.Enum) > 0 {
				enumDesc := strings.Join(enumStrings(propSchema.Enum), ", ")
				if desc == "" {
					desc = "One of: " + enumDesc
				} else {
					desc += " (" + enumDesc + ")"
				}
			}
			switch flagGoType(propSchema) {
			case "string":
				sb.Linef("	cmd.Flags().StringVar(&%s, %q, %q, %q)", varName, flagName, "", desc)
			case "int":
				sb.Linef("	cmd.Flags().IntVar(&%s, %q, 0, %q)", varName, flagName, desc)
			case "int64":
				sb.Linef("	cmd.Flags().Int64Var(&%s, %q, 0, %q)", varName, flagName, desc)
			case "bool":
				sb.Linef("	cmd.Flags().BoolVar(&%s, %q, false, %q)", varName, flagName, desc)
			case "[]string":
				sb.Linef("	cmd.Flags().StringSliceVar(&%s, %q, nil, %q)", varName, flagName, desc)
			case "float64":
				sb.Linef("	cmd.Flags().Float64Var(&%s, %q, 0, %q)", varName, flagName, desc)
			}
		}
		sb.Line(`	cmd.Flags().StringVar(&bodyJSON, "body", "", "Request body as JSON (overrides individual flags)")`)
	} else if op.RequestBody != nil {
		sb.Line(`	cmd.Flags().StringVar(&bodyJSON, "body", "", "Request body as JSON")`)
	}

	// Pagination flags for paginated operations
	if op.Pagination != nil {
		sb.Line("")
		sb.Line("	// Pagination flags")
		sb.Line(`	cmd.Flags().Bool("all", false, "Fetch all pages")`)
		sb.Line(`	cmd.Flags().Int("max-pages", 0, "Maximum pages to fetch (with --all, 0=unlimited)")`)
		switch op.Pagination.Type {
		case registry.PaginationCursor:
			sb.Linef("	cmd.Flags().String(%q, \"\", \"Pagination cursor for next page\")", op.Pagination.InputCursor.Name)
		case registry.PaginationOffset:
			sb.Linef("	cmd.Flags().Int(%q, 0, \"Offset for pagination\")", op.Pagination.InputCursor.Name)
		case registry.PaginationPage:
			sb.Linef("	cmd.Flags().Int(%q, 1, \"Page number\")", op.Pagination.InputCursor.Name)
		}
		if op.Pagination.InputLimit.Name != "" {
			sb.Linef("	cmd.Flags().Int(%q, %d, \"Number of items per page\")", op.Pagination.InputLimit.Name, op.Pagination.DefaultLimit)
		}
	}

	// Retry flags on all commands (conditional on flags config)
	if g.flagsCfg.Retries.Enabled {
		sb.Line("")
		sb.Line("	// Retry flags")
		sb.Line(`	cmd.Flags().Bool("no-retries", false, "Disable retries for this request")`)
		sb.Line(`	cmd.Flags().Int("retry-max-attempts", 0, "Override max retry attempts")`)
		sb.Line(`	cmd.Flags().String("retry-max-elapsed", "", "Override max elapsed retry time (e.g. 5m)")`)
		if g.flagsCfg.Retries.Hidden {
			sb.Line(`	_ = cmd.Flags().MarkHidden("no-retries")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("retry-max-attempts")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("retry-max-elapsed")`)
		}
	}

	// Watch/poll flags — only for GET operations.
	if isWatchEnabledForOp(op, g.watchEnabled) && g.flagsCfg.Watch.Enabled {
		sb.Line("")
		sb.Line("	// Watch/poll flags (GET operations only)")
		sb.Line(`	cmd.Flags().Bool("watch", false, "Re-run on a timer and update the display (like watch(1))")`)
		// Determine baked-in default interval (per-op > global flag default > global feature default).
		pollDefault := g.watchInterval
		if op.CLIWatchInterval != "" {
			pollDefault = op.CLIWatchInterval
		}
		if g.flagsCfg.PollInterval.Default != "" {
			pollDefault = g.flagsCfg.PollInterval.Default
		}
		sb.Linef(`	cmd.Flags().String("poll-interval", %q, "Interval between watch iterations (e.g. 5s, 1m)")`, pollDefault)
		// Determine baked-in default max count.
		watchCountDefault := g.watchMaxCount
		if op.CLIWatchMaxCount > 0 {
			watchCountDefault = op.CLIWatchMaxCount
		}
		sb.Linef(`	cmd.Flags().Int("watch-count", %d, "Max iterations before stopping (0 = infinite)")`, watchCountDefault)
		if g.flagsCfg.Watch.Hidden {
			sb.Line(`	_ = cmd.Flags().MarkHidden("watch")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("poll-interval")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("watch-count")`)
		}
	}

	// Wait flags — registered for any operation with wait enabled.
	if waitLoop && g.flagsCfg.Wait.Enabled {
		bakedConditionHelp := ""
		if waitBakedCondition != "" {
			bakedConditionHelp = fmt.Sprintf(" (default condition: %s)", waitBakedCondition)
		}
		sb.Line("")
		sb.Line("	// Wait flags")
		sb.Line(`	cmd.Flags().Bool("wait", false, "Block until a condition is met (use --wait-for to set or override the condition)")`)
		sb.Linef(`	cmd.Flags().String("wait-for", "", "jq expression to wait for%s")`, bakedConditionHelp)
		sb.Linef(`	cmd.Flags().String("wait-timeout", %q, "Max time to wait (e.g. 5m, 1h); empty = no timeout")`, waitTimeoutDefault)
		if g.flagsCfg.Wait.Hidden {
			sb.Line(`	_ = cmd.Flags().MarkHidden("wait")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("wait-for")`)
			sb.Line(`	_ = cmd.Flags().MarkHidden("wait-timeout")`)
		}
	}

	sb.Line("")
	sb.Line("	return cmd")
	sb.Line("}")
	sb.Line("")
}

// --- String builder helper ---

// StringBuilder is a convenience wrapper for building Go source code.
type StringBuilder struct {
	buf strings.Builder
}

func (s *StringBuilder) Line(line string) {
	s.buf.WriteString(line)
	s.buf.WriteByte('\n')
}

func (s *StringBuilder) Linef(format string, args ...any) {
	s.buf.WriteString(fmt.Sprintf(format, args...))
	s.buf.WriteByte('\n')
}

func (s *StringBuilder) String() string {
	return s.buf.String()
}

// --- Helpers ---

func toCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return s
	}
	result := strings.ToLower(parts[0])
	for _, p := range parts[1:] {
		if len(p) > 0 {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	var result string
	for _, p := range parts {
		if len(p) > 0 {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result
}

func flagGoType(s registry.SchemaMeta) string {
	switch s.Type {
	case "integer":
		if s.Format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]string"
	default:
		return "string"
	}
}

func quoteJoin(ss []string) string {
	var parts []string
	for _, s := range ss {
		parts = append(parts, fmt.Sprintf("%q", s))
	}
	return strings.Join(parts, ", ")
}

func writeFormatted(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	// Try gofmt
	formatted, err := goformat.Source([]byte(content))
	if err != nil {
		// Write unformatted for debugging
		_ = os.WriteFile(path, []byte(content), 0o644)
		return fmt.Errorf("gofmt %s: %w", path, err)
	}

	return os.WriteFile(path, formatted, 0o644)
}

// RootData holds template data for the root command.
type RootData struct {
	AppName     string
	EnvPrefix   string
	Description string
	Servers     []registry.ServerConfig
}

// GroupData holds template data for a tag-group command file.
type GroupData struct {
	Tag        string
	AppName    string
	Operations []registry.OperationMeta
}

// bodyPropFlagName returns the CLI flag name for a body property,
// prefixing with "body-" only if the name collides with an existing param flag.
func bodyPropFlagName(propName string, existingParamFlags map[string]bool) string {
	name := toKebabCase(propName)
	if existingParamFlags[name] {
		return "body-" + name
	}
	return name
}

// toKebabCase converts camelCase or PascalCase to kebab-case.
func toKebabCase(s string) string {
	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '-')
			}
			result = append(result, byte(r+32))
		} else if r == '_' || r == ' ' {
			result = append(result, '-')
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// sortedSchemaKeys returns the keys of a schema properties map in sorted order.
func sortedSchemaKeys(m map[string]registry.SchemaMeta) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// enumStrings converts []any enum values to []string.
func enumStrings(enum []any) []string {
	result := make([]string, len(enum))
	for i, e := range enum {
		result[i] = fmt.Sprintf("%v", e)
	}
	return result
}

// wrapText wraps text at the given column width, preserving existing newlines.
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line == "" {
				line = word
			} else if len(line)+1+len(word) > width {
				result.WriteString(line)
				result.WriteByte('\n')
				line = word
			} else {
				line += " " + word
			}
		}
		result.WriteString(line)
	}
	return result.String()
}

func toSnakeCase(s string) string {
	var result []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(r+32))
		} else if r == '-' || r == ' ' {
			result = append(result, '_')
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// sortedStringKeys returns the keys of any map[string]V sorted alphabetically.
// Used wherever we iterate over maps to produce deterministic generated output.
func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
