package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/the-inconvenience-store/cliford/internal/cli"
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
	if err := cliGen.Generate(p.Registry); err != nil {
		return fmt.Errorf("generate CLI commands: %w", err)
	}

	// Generate auth commands and infrastructure
	authGen := cli.NewAuthGenerator(p.Config.OutputDir, p.Config.AppName, p.Registry.SecuritySchemes)
	authGen.SetPackagePath(p.Config.PackageName + "/internal/auth")
	if err := authGen.Generate(); err != nil {
		return fmt.Errorf("generate auth commands: %w", err)
	}

	// Generate config commands
	configGen := cli.NewConfigCmdGenerator(p.Config.OutputDir, p.Config.AppName)
	if err := configGen.Generate(); err != nil {
		return fmt.Errorf("generate config commands: %w", err)
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

	// Generate HTTP client factory (layered transport: verbose → auth → retry → default)
	factoryGen := sdk.NewClientFactoryGenerator(p.Config.OutputDir, p.Config.AppName, p.Config.EnvVarPrefix, p.Config.PackageName)
	if err := factoryGen.Generate(); err != nil {
		return fmt.Errorf("generate client factory: %w", err)
	}

	// Generate OAuth support if any OAuth schemes exist
	for _, scheme := range p.Registry.SecuritySchemes {
		if scheme.Flows != nil {
			oauthGen := sdk.NewOAuthEnhancer(p.Config.OutputDir, p.Config.AppName, scheme.Flows)
			if err := oauthGen.Generate(); err != nil {
				return fmt.Errorf("generate OAuth: %w", err)
			}
			break
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
	// Generate main.go with layered HTTP client setup
	mainGo := fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"%s/internal/auth"
	"%s/internal/cli"
	"%s/internal/client"
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
`, p.Config.PackageName, p.Config.PackageName, p.Config.PackageName, p.Config.PackageName, p.Config.AppName, p.Config.AppName, p.Config.AppName)

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
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
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
