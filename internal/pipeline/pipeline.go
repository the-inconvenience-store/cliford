package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchyny/gojq"

	"github.com/the-inconvenience-store/cliford/internal/cli"
	"github.com/the-inconvenience-store/cliford/internal/config"
	"github.com/the-inconvenience-store/cliford/internal/overlay"
	"github.com/the-inconvenience-store/cliford/internal/codegen"
	"github.com/the-inconvenience-store/cliford/internal/distribution"
	"github.com/the-inconvenience-store/cliford/internal/docs"
	"github.com/the-inconvenience-store/cliford/internal/hybrid"
	"github.com/the-inconvenience-store/cliford/internal/sdk"
	appTui "github.com/the-inconvenience-store/cliford/internal/tui"
	"github.com/the-inconvenience-store/cliford/pkg/registry"
	"github.com/the-inconvenience-store/cliford/pkg/theme"
)

// StageStatus represents the state of a pipeline stage.
type StageStatus string

const (
	StagePending  StageStatus = "pending"
	StageRunning  StageStatus = "running"
	StageComplete StageStatus = "complete"
	StageFailed   StageStatus = "failed"
	StageSkipped  StageStatus = "skipped"
)

// Stage represents a single step in the generation pipeline.
type Stage struct {
	Name    string
	Status  StageStatus
	RunFunc func(ctx context.Context, p *Pipeline) error
	Err     error
	Start   time.Time
	End     time.Time
}

// Config holds the configuration for a pipeline run.
type Config struct {
	SpecPath    string
	OutputDir   string
	TemplateDir string
	DryRun      bool
	Force       bool
	Verbose     bool

	// Generation options
	RemoveStutter     bool
	GenerateTUI       bool
	GenerateRelease   bool
	CustomCodeRegions bool

	// App identity
	AppName      string
	PackageName  string
	EnvVarPrefix string
	AppVersion   string

	// Spinner config for generated CLI loading animation
	SpinnerEnabled bool
	SpinnerFrames  []string
	SpinnerMs      int

	// Runtime hooks baked into the generated app at generation time.
	BeforeRequestHooks []RuntimeHookDef
	AfterResponseHooks []RuntimeHookDef

	// GlobalParamGenerators are global params baked in at generation time that
	// produce a fresh value (uuid, timestamp) for every request.
	GlobalParamGenerators []GlobalParamGeneratorDef

	// AuthAllowedMethods restricts which auth methods are offered in the
	// generated login command. Empty means all spec-derived methods are offered.
	AuthAllowedMethods []string

	// OperationDefaultJQs maps operation IDs to their default jq expression.
	// Applied to the registry before CLI generation; cliford.yaml takes highest priority.
	OperationDefaultJQs map[string]string

	// AgentOutputFormat is the global default output format used when --agent is active
	// and --output-format was not explicitly set by the user (e.g. "toon").
	AgentOutputFormat string

	// OperationAgentFormats maps operation IDs to per-operation agent output format overrides.
	// Applied to the registry before CLI generation; overrides AgentOutputFormat per operation.
	OperationAgentFormats map[string]string

	// OverlayPaths lists OAI Overlay Specification files to apply to the spec
	// before all other stages. Applied in order; later overlays see the result
	// of earlier ones. When set via --overlay flags, takes full priority over
	// cliford.yaml overlays.
	OverlayPaths []string

	// CLIFlags controls which global flags are generated and their defaults.
	// Defaults to all flags enabled with built-in defaults when zero value.
	CLIFlags config.CLIFlagsConfig

	// OperationDefaultOutputFormats maps operation IDs to per-operation default
	// output format overrides (cliford.yaml highest priority, same pattern as
	// OperationDefaultJQs).
	OperationDefaultOutputFormats map[string]string

	// RequestIDEnabled enables automatic UUID injection into every generated RunE.
	// When true, a requestID variable is generated, set as a request header, and
	// embedded in all error messages for server-side log correlation.
	RequestIDEnabled bool

	// RequestIDHeader is the HTTP header name used for request ID injection.
	// Defaults to "X-Request-ID" when empty.
	RequestIDHeader string

	// OperationRequestIDOverrides maps operation IDs to per-operation request ID
	// enable overrides. A true value enables request ID for that operation even
	// when RequestIDEnabled is false.
	OperationRequestIDOverrides map[string]bool

	// Watch feature config
	WatchEnabled bool   // features.watch.enabled
	WatchInterval string // features.watch.defaultInterval (e.g. "5s")
	WatchMaxCount int    // features.watch.maxCount (0 = infinite)

	// OperationWatchOverrides maps operation IDs to per-operation watch overrides.
	// Applied in stageCLI after x-cliford-cli extensions; cliford.yaml takes priority.
	OperationWatchOverrides map[string]OperationWatchOverride

	// Wait feature config
	WaitEnabled  bool   // features.wait.enabled
	WaitInterval string // features.wait.defaultInterval (e.g. "15s")
	WaitTimeout  string // features.wait.defaultTimeout ("" = no timeout)

	// OperationWaitOverrides maps operation IDs to per-operation wait overrides.
	// Applied in stageCLI after x-cliford-wait extensions; cliford.yaml takes priority.
	OperationWaitOverrides map[string]OperationWaitOverride
}

// OperationWatchOverride holds per-operation watch configuration from cliford.yaml.
type OperationWatchOverride struct {
	Enabled  *bool
	Interval string
	MaxCount int
}

// OperationWaitOverride holds per-operation wait configuration from cliford.yaml.
type OperationWaitOverride struct {
	Enabled        *bool
	Condition      string
	ErrorCondition string
	Interval       string
	Timeout        string
	Message        string
}

// RuntimeHookDef describes a hook baked into the generated app at generation time.
type RuntimeHookDef struct {
	Type       string // "shell" or "go-plugin"
	Command    string // shell hook command
	PluginPath string // go-plugin binary path
}

// GlobalParamGeneratorDef describes a global param whose value is generated
// fresh for every request. Baked into the generated app at generation time.
type GlobalParamGeneratorDef struct {
	Name     string // Header/query param name, e.g. "X-Request-ID"
	In       string // "header" or "query"
	Strategy string // "uuid" or "timestamp"
}

// Pipeline orchestrates the full code generation process.
type Pipeline struct {
	Config   Config
	Registry *registry.Registry
	Stages   []*Stage
	Results  map[string]any // Shared state between stages
}

// New creates a pipeline with the standard generation stages.
func New(cfg Config) *Pipeline {
	p := &Pipeline{
		Config:  cfg,
		Results: make(map[string]any),
	}

	p.Stages = []*Stage{
		{Name: "overlay", Status: StagePending, RunFunc: stageOverlay},
		{Name: "validate", Status: StagePending, RunFunc: stageValidate},
		{Name: "sdk", Status: StagePending, RunFunc: stageSDK},
		{Name: "cli", Status: StagePending, RunFunc: stageCLI},
		{Name: "tui", Status: StagePending, RunFunc: stageTUI},
		{Name: "infra", Status: StagePending, RunFunc: stageInfra},
	}

	return p
}

// Run executes all pipeline stages in sequence.
func (p *Pipeline) Run(ctx context.Context) error {
	// Clean up any temp spec file written by stageOverlay.
	defer func() {
		if tmp, ok := p.Results["_tempSpecPath"].(string); ok {
			os.Remove(tmp)
		}
	}()

	for _, stage := range p.Stages {
		if ctx.Err() != nil {
			stage.Status = StageSkipped
			continue
		}

		stage.Status = StageRunning
		stage.Start = time.Now()

		if err := stage.RunFunc(ctx, p); err != nil {
			stage.Status = StageFailed
			stage.Err = err
			stage.End = time.Now()
			return fmt.Errorf("stage '%s' failed: %w", stage.Name, err)
		}

		stage.Status = StageComplete
		stage.End = time.Now()
	}

	return nil
}

// Summary returns a human-readable summary of the pipeline run.
func (p *Pipeline) Summary() string {
	var result string
	for _, s := range p.Stages {
		duration := s.End.Sub(s.Start).Round(time.Millisecond)
		switch s.Status {
		case StageComplete:
			result += fmt.Sprintf("  %-12s %s (%s)\n", s.Name, "OK", duration)
		case StageFailed:
			result += fmt.Sprintf("  %-12s %s: %v\n", s.Name, "FAILED", s.Err)
		case StageSkipped:
			result += fmt.Sprintf("  %-12s %s\n", s.Name, "SKIPPED")
		case StagePending:
			result += fmt.Sprintf("  %-12s %s\n", s.Name, "PENDING")
		}
	}
	return result
}

// --- Stage implementations (stubs for now, wired in Phase 10) ---

func stageOverlay(_ context.Context, p *Pipeline) error {
	if len(p.Config.OverlayPaths) == 0 {
		return nil
	}

	merged, err := overlay.Apply(p.Config.SpecPath, p.Config.OverlayPaths)
	if err != nil {
		return fmt.Errorf("apply overlays: %w", err)
	}

	tmp, err := os.CreateTemp("", "cliford-merged-spec-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file for merged spec: %w", err)
	}
	if _, err := tmp.Write(merged); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write merged spec: %w", err)
	}
	tmp.Close()

	p.Config.SpecPath = tmp.Name()
	p.Results["_tempSpecPath"] = tmp.Name()
	return nil
}

func stageValidate(ctx context.Context, p *Pipeline) error {
	result, reg, err := Validate(ctx, p.Config.SpecPath, p.Config.RemoveStutter)
	if err != nil {
		return err
	}
	if result.HasErrors() {
		return fmt.Errorf("validation errors:\n  %s", joinLines(result.Errors))
	}
	p.Registry = reg
	return nil
}

func stageSDK(ctx context.Context, p *Pipeline) error {
	if p.Config.DryRun {
		return nil
	}
	sdkDir := filepath.Join(p.Config.OutputDir, "internal", "sdk")
	gen := sdk.NewGenerator(p.Config.SpecPath, sdkDir, "sdk")
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generate SDK: %w", err)
	}

	// Generate pagination helpers
	pagGen := sdk.NewPaginationEnhancer(p.Config.OutputDir)
	if err := pagGen.Generate(); err != nil {
		return fmt.Errorf("generate pagination: %w", err)
	}

	// Generate retry middleware
	retryGen := sdk.NewRetryEnhancer(p.Config.OutputDir)
	if err := retryGen.Generate(); err != nil {
		return fmt.Errorf("generate retry: %w", err)
	}

	// Generate error types
	errGen := sdk.NewErrorEnhancer(p.Config.OutputDir)
	if err := errGen.Generate(); err != nil {
		return fmt.Errorf("generate errors: %w", err)
	}

	return nil
}

func stageCLI(ctx context.Context, p *Pipeline) error {
	if p.Config.DryRun || p.Registry == nil {
		return nil
	}
	engine := codegen.NewEngine(p.Config.TemplateDir)
	cliGen := cli.NewGenerator(engine, p.Config.OutputDir, p.Config.AppName, p.Config.EnvVarPrefix)
	cliGen.SetCustomCodeRegions(p.Config.CustomCodeRegions)
	cliGen.SetGenerateTUI(p.Config.GenerateTUI)
	cliGen.SetPackagePath(p.Config.PackageName)
	if len(p.Config.SpinnerFrames) > 0 || p.Config.SpinnerMs > 0 || !p.Config.SpinnerEnabled {
		cliGen.SetSpinnerConfig(cli.SpinnerConfig{
			Enabled:    p.Config.SpinnerEnabled,
			Frames:     p.Config.SpinnerFrames,
			IntervalMs: p.Config.SpinnerMs,
		})
	}
	// Apply per-operation default JQ expressions from cliford.yaml (highest priority).
	// x-cliford-cli defaultJQ (set during spec parsing) is only overwritten when
	// the cliford.yaml value is non-empty.
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if jq, ok := p.Config.OperationDefaultJQs[op.OperationID]; ok && jq != "" {
			op.CLIDefaultJQ = jq
		}
	}

	// Apply per-operation agent format overrides from cliford.yaml (highest priority).
	// x-cliford-cli agentFormat (set during spec parsing) is only overwritten when
	// the cliford.yaml value is non-empty.
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if af, ok := p.Config.OperationAgentFormats[op.OperationID]; ok && af != "" {
			op.CLIAgentFormat = af
		}
	}

	// Apply per-operation default output format overrides from cliford.yaml.
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if f, ok := p.Config.OperationDefaultOutputFormats[op.OperationID]; ok && f != "" {
			op.CLIDefaultOutputFormat = f
		}
	}

	// Resolve per-operation request ID (global baseline + per-op overrides).
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if p.Config.RequestIDEnabled {
			op.CLIRequestID = true
		}
		if override, ok := p.Config.OperationRequestIDOverrides[op.OperationID]; ok {
			op.CLIRequestID = override
		}
	}

	// Pass flags config to the CLI generator.
	cliGen.SetFlagsConfig(p.Config.CLIFlags)

	// Pass request ID config to the CLI generator when any operation uses it.
	if p.Config.RequestIDEnabled || len(p.Config.OperationRequestIDOverrides) > 0 {
		cliGen.SetRequestID(p.Config.RequestIDEnabled, p.Config.RequestIDHeader)
	}

	// Apply watch feature config.
	if p.Config.WatchEnabled {
		cliGen.SetWatchConfig(true, p.Config.WatchInterval, p.Config.WatchMaxCount)
	}

	// Apply per-op watch overrides from cliford.yaml (highest priority; overwrites x-cliford-cli).
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if override, ok := p.Config.OperationWatchOverrides[op.OperationID]; ok {
			if override.Enabled != nil {
				op.CLIWatchEnabled = override.Enabled
			}
			if override.Interval != "" {
				op.CLIWatchInterval = override.Interval
			}
			if override.MaxCount > 0 {
				op.CLIWatchMaxCount = override.MaxCount
			}
		}
	}

	// Apply wait feature config.
	if p.Config.WaitEnabled {
		cliGen.SetWaitConfig(true, p.Config.WaitInterval, p.Config.WaitTimeout)
	}

	// Apply per-op wait overrides from cliford.yaml (highest priority; overwrites x-cliford-wait).
	for i := range p.Registry.Operations {
		op := &p.Registry.Operations[i]
		if override, ok := p.Config.OperationWaitOverrides[op.OperationID]; ok {
			if override.Enabled != nil {
				op.CLIWaitEnabled = override.Enabled
			}
			if override.Condition != "" {
				op.CLIWaitCondition = override.Condition
			}
			if override.ErrorCondition != "" {
				op.CLIWaitErrorCondition = override.ErrorCondition
			}
			if override.Interval != "" {
				op.CLIWaitInterval = override.Interval
			}
			if override.Timeout != "" {
				op.CLIWaitTimeout = override.Timeout
			}
			if override.Message != "" {
				op.CLIWaitMessage = override.Message
			}
		}
	}

	// Set the global agent output format on the generator.
	if p.Config.AgentOutputFormat != "" {
		cliGen.SetAgentOutputFormat(p.Config.AgentOutputFormat)
	}

	// Validate all non-empty defaultJQ expressions eagerly so misconfigurations
	// are caught at generation time rather than at runtime.
	for _, op := range p.Registry.Operations {
		if op.CLIDefaultJQ != "" {
			if _, err := gojq.Parse(op.CLIDefaultJQ); err != nil {
				return fmt.Errorf("operation %q: invalid defaultJQ expression %q: %w",
					op.OperationID, op.CLIDefaultJQ, err)
			}
		}
	}

	if err := cliGen.Generate(p.Registry); err != nil {
		return fmt.Errorf("generate CLI commands: %w", err)
	}

	// Generate auth commands and infrastructure
	authGen := cli.NewAuthGenerator(p.Config.OutputDir, p.Config.AppName, p.Registry.SecuritySchemes)
	authGen.SetPackagePath(p.Config.PackageName + "/internal/auth")
	if len(p.Config.AuthAllowedMethods) > 0 {
		authGen.SetAllowedMethods(p.Config.AuthAllowedMethods)
	}
	authGen.SetEnvPrefix(p.Config.EnvVarPrefix)
	if err := authGen.Generate(); err != nil {
		return fmt.Errorf("generate auth commands: %w", err)
	}

	// Generate config commands
	configGen := cli.NewConfigCmdGenerator(p.Config.OutputDir, p.Config.AppName)
	if err := configGen.Generate(); err != nil {
		return fmt.Errorf("generate config commands: %w", err)
	}

	// Generate alias commands and ResolveAliases helper
	aliasGen := cli.NewAliasCmdGenerator(p.Config.OutputDir, p.Config.AppName)
	if err := aliasGen.Generate(); err != nil {
		return fmt.Errorf("generate alias commands: %w", err)
	}

	// Generate auth middleware and keychain
	authEnhancer := sdk.NewAuthEnhancer(p.Config.OutputDir, p.Config.AppName, p.Config.EnvVarPrefix, p.Registry.SecuritySchemes)
	if err := authEnhancer.Generate(); err != nil {
		return fmt.Errorf("generate auth middleware: %w", err)
	}

	// Generate credential redaction
	redactGen := sdk.NewRedactGenerator(p.Config.OutputDir)
	if err := redactGen.Generate(); err != nil {
		return fmt.Errorf("generate redaction: %w", err)
	}

	// Generate verbose transport
	verboseGen := sdk.NewVerboseEnhancer(p.Config.OutputDir, p.Config.PackageName)
	if err := verboseGen.Generate(); err != nil {
		return fmt.Errorf("generate verbose transport: %w", err)
	}

	// Generate runtime hooks (before_request / after_response)
	hooksGen := sdk.NewHooksEnhancer(p.Config.OutputDir, p.Config.PackageName)
	if err := hooksGen.Generate(); err != nil {
		return fmt.Errorf("generate hooks runner: %w", err)
	}

	// Determine whether any OAuth 2.0 schemes are present (affects factory and middleware generation).
	hasOAuth := false
	for _, scheme := range p.Registry.SecuritySchemes {
		if scheme.Flows != nil {
			hasOAuth = true
			break
		}
	}

	// Generate HTTP client factory (layered transport: verbose → auth → retry → default)
	factoryGen := sdk.NewClientFactoryGenerator(p.Config.OutputDir, p.Config.AppName, p.Config.EnvVarPrefix, p.Config.PackageName)
	factoryGen.SetHasOAuth(hasOAuth)
	if err := factoryGen.Generate(); err != nil {
		return fmt.Errorf("generate client factory: %w", err)
	}

	// Generate OAuth support if any OAuth schemes exist
	if hasOAuth {
		for _, scheme := range p.Registry.SecuritySchemes {
			if scheme.Flows != nil {
				oauthGen := sdk.NewOAuthEnhancer(p.Config.OutputDir, p.Config.AppName, scheme.Flows)
				if err := oauthGen.Generate(); err != nil {
					return fmt.Errorf("generate OAuth: %w", err)
				}
				break
			}
		}
	}

	// Generate error formatting
	errOutGen := cli.NewErrorOutputGenerator(p.Config.OutputDir)
	if err := errOutGen.Generate(); err != nil {
		return fmt.Errorf("generate error output: %w", err)
	}

	return nil
}

func stageTUI(ctx context.Context, p *Pipeline) error {
	if p.Config.DryRun || p.Registry == nil {
		return nil
	}

	// Always generate hybrid mode detection + prompts
	adapterGen := hybrid.NewAdapterGenerator(p.Config.OutputDir, p.Config.EnvVarPrefix)
	if err := adapterGen.Generate(); err != nil {
		return fmt.Errorf("generate mode detector: %w", err)
	}

	promptGen := hybrid.NewPromptGenerator(p.Config.OutputDir, p.Config.PackageName)
	if err := promptGen.Generate(); err != nil {
		return fmt.Errorf("generate prompts: %w", err)
	}

	// Generate full TUI if enabled
	if p.Config.GenerateTUI {
		thm := theme.DefaultConfig()
		tuiGen := appTui.NewGenerator(p.Config.OutputDir, p.Config.AppName, p.Config.PackageName, thm)
		if err := tuiGen.Generate(p.Registry); err != nil {
			return fmt.Errorf("generate TUI: %w", err)
		}
	}

	return nil
}

func stageInfra(ctx context.Context, p *Pipeline) error {
	if p.Config.DryRun {
		return nil
	}

	if err := generateInfra(p); err != nil {
		return err
	}

	// Distribution: GoReleaser, install scripts, Homebrew
	if p.Config.GenerateRelease {
		grGen := distribution.NewGoReleaserGenerator(p.Config.OutputDir, p.Config.AppName, p.Config.PackageName)
		if err := grGen.Generate(); err != nil {
			return fmt.Errorf("generate GoReleaser: %w", err)
		}

		installGen := distribution.NewInstallScriptGenerator(p.Config.OutputDir, p.Config.AppName)
		if err := installGen.Generate(); err != nil {
			return fmt.Errorf("generate install scripts: %w", err)
		}

		brewGen := distribution.NewHomebrewGenerator(p.Config.OutputDir, p.Config.AppName, p.Registry.Description)
		if err := brewGen.Generate(); err != nil {
			return fmt.Errorf("generate Homebrew formula: %w", err)
		}
	}

	// Documentation
	if p.Registry != nil {
		mdGen := docs.NewMarkdownGenerator(p.Config.OutputDir, p.Config.AppName, p.Config.PackageName)
		if err := mdGen.Generate(p.Registry); err != nil {
			return fmt.Errorf("generate Markdown docs: %w", err)
		}

		llmsGen := docs.NewLLMsGenerator(p.Config.OutputDir, p.Config.AppName)
		if err := llmsGen.Generate(p.Registry); err != nil {
			return fmt.Errorf("generate llms.txt: %w", err)
		}
	}

	// Write lockfile
	WriteLockfile(p.Config.OutputDir, p.Config.SpecPath, "", "dev")

	return nil
}

func generateInfra(p *Pipeline) error {
	// Build conditional OAuth2 auto-refresh block for main.go.
	// auth.OAuthConfig only exists in generated apps with OAuth 2.0 schemes.
	oauthBlock := ""
	if p.Registry != nil {
		for schemeName, scheme := range p.Registry.SecuritySchemes {
			if scheme.Type == registry.SecurityTypeOAuth2 || scheme.Type == registry.SecurityTypeOpenIDConnect {
				// Derive env var prefix: same sanitization as the resolver.
				envPfx := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(p.Config.EnvVarPrefix))
				if envPfx == "" {
					envPfx = strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(p.Config.AppName))
				}
				schemeSegment := strings.ToUpper(strings.NewReplacer("-", "_", " ", "_").Replace(schemeName))
				tokenURLVar := envPfx + "_" + schemeSegment + "_TOKEN_URL"
				clientIDVar := envPfx + "_" + schemeSegment + "_CLIENT_ID"
				clientSecretVar := envPfx + "_" + schemeSegment + "_CLIENT_SECRET"

				oauthBlock = fmt.Sprintf(`
	// OAuth2 auto-refresh: build OAuthConfig from env vars so the auth middleware
	// can proactively refresh near-expiry tokens on every request.
	if oauthTokenURL := os.Getenv(%q); oauthTokenURL != "" {
		if oauthClientID := os.Getenv(%q); oauthClientID != "" {
			opts.OAuthConfig = &auth.OAuthConfig{
				TokenURL:     oauthTokenURL,
				ClientID:     oauthClientID,
				ClientSecret: os.Getenv(%q),
			}
			opts.CredentialStore = auth.NewStore()
		}
	}
`, tokenURLVar, clientIDVar, clientSecretVar)
				break
			}
		}
	}

	hooksBlock := buildHooksBlock(p.Config.BeforeRequestHooks, p.Config.AfterResponseHooks)
	hooksBlock += buildGeneratorsBlock(p.Config.GlobalParamGenerators)

	// Only import the hooks package when hooks are actually baked in.
	// An unconditional import with no references is a compile error in Go.
	hooksImport := ""
	if len(p.Config.BeforeRequestHooks) > 0 || len(p.Config.AfterResponseHooks) > 0 {
		hooksImport = fmt.Sprintf("\n\t\"%s/internal/hooks\"", p.Config.PackageName)
	}

	// Generate main.go with layered HTTP client setup
	mainGo := fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"%s/internal/auth"
	"%s/internal/cli"
	"%s/internal/client"%s
	"%s/internal/config"
	"%s/internal/sdk"

	"github.com/spf13/viper"
)

var (
	version  = "dev"
	commit   = "none"
	date     = "unknown"
	appTitle = "%s"
)

func main() {
	// Initialize configuration from ~/.config/<app>/config.yaml and env vars.
	config.Init("%s", "%s")

	// Expand any user-defined alias before Cobra parses the command tree.
	os.Args = cli.ResolveAliases(os.Args)

	// Create the shared HTTP client with auth and retry transport layers.
	// This is the SDK-first architecture: all commands share a single,
	// pre-configured client with 5-tier credential resolution:
	// flags > env vars > OS keychain > encrypted file > config file.
	resolver := auth.NewResolver("%s", auth.DefaultSchemes())
	opts := client.DefaultOptions()
	opts.AuthResolver = resolver
	opts.VerboseFlag = cli.VerboseFlag()

	// FeaturesConfig: timeout override from Viper / env
	if timeout := viper.GetDuration("request_timeout"); timeout > 0 {
		opts.BaseTimeout = timeout
	}

	// FeaturesConfig: retry overrides from Viper
	if !viper.GetBool("features.retry.enabled") && viper.IsSet("features.retry.enabled") {
		opts.RetryEnabled = false
	}
	if maxAttempts := viper.GetInt("features.retry.max_attempts"); maxAttempts > 0 {
		cfg := sdk.DefaultRetryConfig()
		cfg.MaxAttempts = maxAttempts
		if interval := viper.GetDuration("features.retry.initial_interval"); interval > 0 {
			cfg.InitialInterval = interval
		}
		opts.RetryConfig = &cfg
	}

	// Load global params from config (global_params.headers / global_params.query)
	opts.GlobalHeaders = viper.GetStringMapString("global_params.headers")
	opts.GlobalQueryParams = viper.GetStringMapString("global_params.query")
%s%s
	cli.SetAPIClient(client.NewHTTPClient(opts))

	// Apply server_url from config/env if --server flag not used
	if serverOverride := viper.GetString("server_url"); serverOverride != "" {
		cli.SetDefaultServerURL(serverOverride)
	}

	rootCmd := cli.RootCmd("%s", fmt.Sprintf("%%s (commit: %%s, built: %%s)", version, commit, date))
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
`, p.Config.PackageName, p.Config.PackageName, p.Config.PackageName, hooksImport, p.Config.PackageName, p.Config.PackageName, p.Config.AppName, p.Config.AppName, p.Config.EnvVarPrefix, p.Config.AppName, oauthBlock, hooksBlock, p.Config.AppName)

	cmdDir := filepath.Join(p.Config.OutputDir, "cmd", p.Config.AppName)
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return fmt.Errorf("create cmd dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	// Generate go.mod
	charmDeps := ""
	if p.Config.GenerateTUI {
		charmDeps = `
	github.com/charmbracelet/bubbletea v1.3.5
	github.com/charmbracelet/bubbles v0.21.0
	github.com/charmbracelet/lipgloss v1.1.0`
	} else {
		// Hybrid mode still needs bubbles for inline prompts
		charmDeps = `
	github.com/charmbracelet/bubbletea v1.3.5
	github.com/charmbracelet/bubbles v0.21.0`
	}

	goMod := fmt.Sprintf(`module %s

go 1.22

require (
	github.com/hashicorp/go-plugin v1.6.2
	github.com/itchyny/gojq v0.12.16
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
	github.com/toon-format/toon-go v0.0.0-20251202084852-7ca0e27c4e8c
	github.com/zalando/go-keyring v0.2.8
	golang.org/x/oauth2 v0.36.0
	gopkg.in/yaml.v3 v3.0.1%s
)
`, p.Config.PackageName, charmDeps)

	if err := os.WriteFile(filepath.Join(p.Config.OutputDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	// Generate config package stub
	configDir := filepath.Join(p.Config.OutputDir, "internal", "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	configGo := fmt.Sprintf(`package config

import (
	"path/filepath"
	"os"

	"github.com/spf13/viper"
)

// Init initializes Viper configuration for the app.
func Init(appName, envPrefix string) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	configDir, _ := os.UserConfigDir()
	viper.AddConfigPath(filepath.Join(configDir, appName))
	viper.AddConfigPath(".")

	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()
}
`)
	if err := os.WriteFile(filepath.Join(configDir, "config.go"), []byte(configGo), 0o644); err != nil {
		return fmt.Errorf("write config.go: %w", err)
	}

	return nil
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += "  - " + l + "\n"
	}
	return result
}

// buildHooksBlock generates the Go source block that bakes runtime hooks into
// the generated main.go. Returns an empty string when no hooks are configured.
func buildHooksBlock(before, after []RuntimeHookDef) string {
	if len(before) == 0 && len(after) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\t// Runtime hooks — baked in at generation time.\n")
	sb.WriteString("\topts.HooksEnabled = true\n")
	if len(before) > 0 {
		sb.WriteString("\topts.BeforeRequestHooks = []hooks.HookDef{\n")
		for _, h := range before {
			writeHookLiteral(&sb, h)
		}
		sb.WriteString("\t}\n")
	}
	if len(after) > 0 {
		sb.WriteString("\topts.AfterResponseHooks = []hooks.HookDef{\n")
		for _, h := range after {
			writeHookLiteral(&sb, h)
		}
		sb.WriteString("\t}\n")
	}
	return sb.String()
}

// buildGeneratorsBlock generates the Go source that wires baked-in global
// param generators into the generated main.go. Returns "" when none configured.
func buildGeneratorsBlock(gens []GlobalParamGeneratorDef) string {
	if len(gens) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\t// Global param generators — baked in at generation time.\n")
	sb.WriteString("\topts.GlobalParamGenerators = []client.ParamGenerator{\n")
	for _, g := range gens {
		switch g.Strategy {
		case "uuid":
			fmt.Fprintf(&sb, "\t\t{Name: %q, In: %q, Fn: client.GenerateUUID},\n", g.Name, g.In)
		case "timestamp":
			fmt.Fprintf(&sb, "\t\t{Name: %q, In: %q, Fn: client.GenerateTimestamp},\n", g.Name, g.In)
		default:
			fmt.Fprintf(&sb, "\t\t// skipped: unknown generate strategy %q for param %q\n", g.Strategy, g.Name)
		}
	}
	sb.WriteString("\t}\n")
	return sb.String()
}

func writeHookLiteral(sb *strings.Builder, h RuntimeHookDef) {
	hookType := h.Type
	if hookType == "" {
		hookType = "shell"
	}
	switch hookType {
	case "shell":
		fmt.Fprintf(sb, "\t\t{Type: %q, Command: %q},\n", hookType, h.Command)
	case "go-plugin":
		fmt.Fprintf(sb, "\t\t{Type: %q, PluginPath: %q},\n", hookType, h.PluginPath)
	}
}
