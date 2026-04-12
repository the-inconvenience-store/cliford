// Package config handles loading and merging Cliford configuration
// from cliford.yaml, OpenAPI extensions, and built-in defaults.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/the-inconvenience-store/cliford/pkg/theme"
)

// ClifordConfig holds the complete configuration for a Cliford generation run.
type ClifordConfig struct {
	Version string `mapstructure:"version"`
	Spec    string `mapstructure:"spec"`

	App AppConfig `mapstructure:"app"`

	Generation GenerationConfig `mapstructure:"generation"`

	Auth AuthConfig `mapstructure:"auth"`

	Theme theme.Config `mapstructure:"theme"`

	Features FeaturesConfig `mapstructure:"features"`

	Operations map[string]OperationOverride `mapstructure:"operations"`

	Hooks map[string][]HookDef `mapstructure:"hooks"`

	GlobalParams []GlobalParamDef `mapstructure:"globalParams"`
}

// AppConfig holds identity settings for the generated app.
type AppConfig struct {
	Name         string `mapstructure:"name"`
	Package      string `mapstructure:"package"`
	EnvVarPrefix string `mapstructure:"envVarPrefix"`
	Version      string `mapstructure:"version"`
	Description  string `mapstructure:"description"`
}

// GenerationConfig controls what gets generated.
type GenerationConfig struct {
	Mode string       `mapstructure:"mode"` // pure-cli, pure-tui, hybrid
	SDK  SDKConfig    `mapstructure:"sdk"`
	CLI  CLIGenConfig `mapstructure:"cli"`
	TUI  TUIGenConfig `mapstructure:"tui"`
}

// SDKConfig controls SDK generation.
type SDKConfig struct {
	Generator string `mapstructure:"generator"` // oapi-codegen
	OutputDir string `mapstructure:"outputDir"`
	Package   string `mapstructure:"package"`
}

// CLIGenConfig controls CLI generation.
type CLIGenConfig struct {
	OutputDir     string `mapstructure:"outputDir"`
	RemoveStutter bool   `mapstructure:"removeStutter"`
}

// TUIGenConfig controls TUI generation.
type TUIGenConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	OutputDir string `mapstructure:"outputDir"`
}

// AuthConfig controls authentication generation.
type AuthConfig struct {
	Interactive bool     `mapstructure:"interactive"`
	Keychain    bool     `mapstructure:"keychain"`
	Methods     []string `mapstructure:"methods"`
}

// FeaturesConfig controls optional feature generation.
type FeaturesConfig struct {
	Pagination        bool               `mapstructure:"pagination"`
	Retries           RetryDefaults      `mapstructure:"retries"`
	Spinner           SpinnerConfig      `mapstructure:"spinner"`
	CustomCodeRegions bool               `mapstructure:"customCodeRegions"`
	Documentation     DocsConfig         `mapstructure:"documentation"`
	Distribution      DistConfig         `mapstructure:"distribution"`
	Hooks             RuntimeHooksConfig `mapstructure:"hooks"`
}

// SpinnerConfig controls the loading animation displayed during HTTP requests.
type SpinnerConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	Frames     []string `mapstructure:"frames"`
	IntervalMs int      `mapstructure:"intervalMs"`
}

// RetryDefaults holds global retry defaults.
type RetryDefaults struct {
	Enabled         bool          `mapstructure:"enabled"`
	MaxAttempts     int           `mapstructure:"maxAttempts"`
	InitialInterval time.Duration `mapstructure:"initialInterval"`
	MaxInterval     time.Duration `mapstructure:"maxInterval"`
	MaxElapsedTime  time.Duration `mapstructure:"maxElapsedTime"`
}

// DocsConfig controls documentation generation.
type DocsConfig struct {
	Markdown bool `mapstructure:"markdown"`
	LLMsTxt  bool `mapstructure:"llmsTxt"`
	ManPages bool `mapstructure:"manPages"`
}

// DistConfig controls distribution configuration.
type DistConfig struct {
	GoReleaser bool `mapstructure:"goreleaser"`
	Homebrew   bool `mapstructure:"homebrew"`
}

// OperationOverride holds per-operation configuration.
type OperationOverride struct {
	CLI OperationCLIOverride `mapstructure:"cli"`
	TUI OperationTUIOverride `mapstructure:"tui"`
}

// OperationCLIOverride holds CLI-specific per-operation config.
type OperationCLIOverride struct {
	Aliases    []string `mapstructure:"aliases"`
	Group      string   `mapstructure:"group"`
	Hidden     bool     `mapstructure:"hidden"`
	Confirm    bool     `mapstructure:"confirm"`
	ConfirmMsg string   `mapstructure:"confirmMessage"`
}

// OperationTUIOverride holds TUI-specific per-operation config.
type OperationTUIOverride struct {
	DisplayAs   string `mapstructure:"displayAs"`
	Refreshable bool   `mapstructure:"refreshable"`
}

// HookDef describes a pipeline hook command (used in cliford.yaml hooks section).
type HookDef struct {
	Run string `mapstructure:"run"`
}

// RuntimeHooksConfig holds before/after request hooks baked into the generated app.
type RuntimeHooksConfig struct {
	BeforeRequest []RuntimeHookDef `mapstructure:"beforeRequest"`
	AfterResponse []RuntimeHookDef `mapstructure:"afterResponse"`
}

// RuntimeHookDef describes a single runtime hook embedded at generation time.
type RuntimeHookDef struct {
	Type       string `mapstructure:"type"`       // "shell" or "go-plugin"
	Command    string `mapstructure:"command"`    // shell hook command
	PluginPath string `mapstructure:"pluginPath"` // go-plugin binary path
}

// GlobalParamDef describes a global parameter added to all requests.
type GlobalParamDef struct {
	Name     string `mapstructure:"name"`
	In       string `mapstructure:"in"`
	Generate string `mapstructure:"generate"` // uuid, timestamp, etc.
	Source   string `mapstructure:"source"`   // config, env, static
	EnvVar   string `mapstructure:"envVar"`
	Default  string `mapstructure:"default"`
	Flag     string `mapstructure:"flag"`
}

// Load reads cliford.yaml from the given path using Viper and returns
// the parsed configuration with defaults applied.
func Load(path string) (*ClifordConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Bind environment variables
	v.SetEnvPrefix("CLIFORD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			// Config file not found — return defaults
			cfg := DefaultConfig()
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg ClifordConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("version", "1")
	v.SetDefault("app.name", "app")
	v.SetDefault("app.package", "github.com/example/app")
	v.SetDefault("app.envVarPrefix", "APP")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("generation.mode", "hybrid")
	v.SetDefault("generation.sdk.generator", "oapi-codegen")
	v.SetDefault("generation.sdk.outputDir", "internal/sdk")
	v.SetDefault("generation.sdk.package", "sdk")
	v.SetDefault("generation.cli.outputDir", "internal/cli")
	v.SetDefault("generation.cli.removeStutter", true)
	v.SetDefault("generation.tui.enabled", false)
	v.SetDefault("generation.tui.outputDir", "internal/tui")
	v.SetDefault("auth.interactive", true)
	v.SetDefault("auth.keychain", true)
	v.SetDefault("features.pagination", true)
	v.SetDefault("features.retries.enabled", true)
	v.SetDefault("features.retries.maxAttempts", 3)
	v.SetDefault("features.spinner.enabled", true)
	v.SetDefault("features.spinner.frames", []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"})
	v.SetDefault("features.spinner.intervalMs", 80)
	v.SetDefault("features.customCodeRegions", false)
	v.SetDefault("features.documentation.markdown", true)
	v.SetDefault("features.documentation.llmsTxt", true)
	v.SetDefault("features.distribution.goreleaser", true)
}

// DefaultConfig returns a ClifordConfig with all defaults applied.
func DefaultConfig() ClifordConfig {
	return ClifordConfig{
		Version: "1",
		App: AppConfig{
			Name:         "app",
			Package:      "github.com/example/app",
			EnvVarPrefix: "APP",
			Version:      "0.1.0",
		},
		Generation: GenerationConfig{
			Mode: "hybrid",
			SDK: SDKConfig{
				Generator: "oapi-codegen",
				OutputDir: "internal/sdk",
				Package:   "sdk",
			},
			CLI: CLIGenConfig{
				OutputDir:     "internal/cli",
				RemoveStutter: true,
			},
			TUI: TUIGenConfig{
				Enabled:   false,
				OutputDir: "internal/tui",
			},
		},
		Auth: AuthConfig{
			Interactive: true,
			Keychain:    true,
		},
		Theme: theme.DefaultConfig(),
		Features: FeaturesConfig{
			Pagination: true,
			Retries: RetryDefaults{
				Enabled:     true,
				MaxAttempts: 3,
			},
			Spinner: SpinnerConfig{
				Enabled:    true,
				Frames:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
				IntervalMs: 80,
			},
			Documentation: DocsConfig{
				Markdown: true,
				LLMsTxt:  true,
			},
			Distribution: DistConfig{
				GoReleaser: true,
			},
		},
		Operations: make(map[string]OperationOverride),
	}
}
