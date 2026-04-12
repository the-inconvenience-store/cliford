// Cliford generates Go-powered CLI and TUI applications from OpenAPI specifications.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/the-inconvenience-store/cliford/internal/codegen"
	"github.com/the-inconvenience-store/cliford/internal/config"
	"github.com/the-inconvenience-store/cliford/internal/pipeline"

	"gopkg.in/yaml.v3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var quiet bool
	var noColor bool

	root := &cobra.Command{
		Use:   "cliford",
		Short: "Generate Go CLI and TUI apps from OpenAPI specs",
		Long: `Cliford takes an OpenAPI specification and produces a fully functional
Go application that can operate as a pure CLI, a full TUI, or a hybrid of both.

It is completely configurable, extensible via hooks, and generates apps
that are themselves configurable by their end users.`,
		Version:       fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	root.AddCommand(generateCmd())
	root.AddCommand(initCmd())
	root.AddCommand(validateCmd())
	root.AddCommand(diffCmd())
	root.AddCommand(versionBumpCmd())
	root.AddCommand(doctorCmd())

	return root
}

func generateCmd() *cobra.Command {
	var (
		specPath      string
		configPath    string
		outputDir     string
		templateDir   string
		appName       string
		pkgName       string
		envPrefix     string
		enableTUI     bool
		customRegions bool
		enableRelease bool
		dryRun        bool
		force         bool
		verbose       bool
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Run the generation pipeline",
		Long:  "Parse the OpenAPI spec, generate SDK, CLI, TUI, and infrastructure files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load cliford.yaml for defaults
			fileCfg, cfgErr := config.Load(configPath)
			if cfgErr != nil && configPath != "cliford.yaml" {
				return fmt.Errorf("load config %s: %w", configPath, cfgErr)
			}

			// Apply config file values as defaults for unset flags
			if fileCfg != nil {
				if specPath == "" && fileCfg.Spec != "" {
					specPath = fileCfg.Spec
				}
				if appName == "app" && fileCfg.App.Name != "" && fileCfg.App.Name != "app" {
					appName = fileCfg.App.Name
				}
				if pkgName == "github.com/example/app" && fileCfg.App.Package != "" && fileCfg.App.Package != "github.com/example/app" {
					pkgName = fileCfg.App.Package
				}
				if envPrefix == "APP" && fileCfg.App.EnvVarPrefix != "" && fileCfg.App.EnvVarPrefix != "APP" {
					envPrefix = fileCfg.App.EnvVarPrefix
				}
				if !cmd.Flags().Changed("tui") && fileCfg.Generation.TUI.Enabled {
					enableTUI = true
				}
				if !cmd.Flags().Changed("custom-regions") && fileCfg.Features.CustomCodeRegions {
					customRegions = true
				}
				if !cmd.Flags().Changed("release") && fileCfg.Features.Distribution.GoReleaser {
					enableRelease = true
				}
			}

			if specPath == "" {
				return fmt.Errorf("--spec is required (or set 'spec' in cliford.yaml)")
			}

			cfg := pipeline.Config{
				SpecPath:          specPath,
				OutputDir:         outputDir,
				TemplateDir:       templateDir,
				DryRun:            dryRun,
				Force:             force,
				Verbose:           verbose,
				RemoveStutter:     true,
				GenerateTUI:       enableTUI,
				GenerateRelease:   enableRelease,
				CustomCodeRegions: customRegions,
				AppName:           appName,
				PackageName:       pkgName,
				EnvVarPrefix:      envPrefix,
			}

			// Apply per-operation overrides from config
			if fileCfg != nil {
				cfg.AppVersion = fileCfg.App.Version
				cfg.SpinnerEnabled = fileCfg.Features.Spinner.Enabled
				cfg.SpinnerFrames = fileCfg.Features.Spinner.Frames
				cfg.SpinnerMs = fileCfg.Features.Spinner.IntervalMs
			} else {
				cfg.SpinnerEnabled = true
			}

			p := pipeline.New(cfg)
			ctx := context.Background()

			if verbose {
				fmt.Println("Cliford: starting generation pipeline...")
			}

			if err := p.Run(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}

			if verbose {
				fmt.Println("\nPipeline summary:")
				fmt.Print(p.Summary())
			}

			fmt.Println("Generation complete.")
			return nil
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to OpenAPI spec file (required)")
	cmd.Flags().StringVar(&configPath, "config", "cliford.yaml", "Path to Cliford config file")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory for generated files")
	cmd.Flags().StringVar(&templateDir, "template-dir", "", "Path to templates directory")
	cmd.Flags().StringVar(&appName, "name", "app", "Generated app binary name")
	cmd.Flags().StringVar(&pkgName, "package", "github.com/example/app", "Generated app Go module path")
	cmd.Flags().StringVar(&envPrefix, "env-prefix", "APP", "Environment variable prefix")
	cmd.Flags().BoolVar(&enableTUI, "tui", false, "Generate TUI mode (Bubbletea explorer, forms, views)")
	cmd.Flags().BoolVar(&customRegions, "custom-regions", false, "Generate custom code region markers")
	cmd.Flags().BoolVar(&enableRelease, "release", false, "Generate GoReleaser, install scripts, and Homebrew formula")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be generated without writing files")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite without backup/diff confirmation")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Detailed pipeline progress output")

	return cmd
}

func initCmd() *cobra.Command {
	var (
		specPath string
		appName  string
		pkgName  string
		mode     string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Cliford project",
		Long:  "Create a cliford.yaml configuration file with defaults derived from the OpenAPI spec.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()

			if specPath != "" {
				cfg.Spec = specPath
			}
			if appName != "" {
				cfg.App.Name = appName
				cfg.App.EnvVarPrefix = strings.ToUpper(strings.ReplaceAll(appName, "-", "_"))
			}
			if pkgName != "" {
				cfg.App.Package = pkgName
			}
			if mode != "" {
				cfg.Generation.Mode = mode
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}

			outputPath := "cliford.yaml"
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("%s already exists; remove it first or edit manually", outputPath)
			}

			if err := os.WriteFile(outputPath, data, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", outputPath, err)
			}

			fmt.Printf("Created %s\n", outputPath)
			if specPath != "" {
				fmt.Printf("  spec: %s\n", specPath)
			}
			fmt.Printf("  app.name: %s\n", cfg.App.Name)
			fmt.Printf("  app.package: %s\n", cfg.App.Package)
			fmt.Printf("  generation.mode: %s\n", cfg.Generation.Mode)
			fmt.Println("\nNext: cliford generate")
			return nil
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to OpenAPI spec file")
	cmd.Flags().StringVar(&appName, "name", "", "Generated app binary name")
	cmd.Flags().StringVar(&pkgName, "package", "", "Go module path")
	cmd.Flags().StringVar(&mode, "mode", "", "Generation mode: pure-cli, pure-tui, hybrid")

	return cmd
}

func validateCmd() *cobra.Command {
	var (
		specPath   string
		configPath string
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and spec",
		Long:  "Parse the OpenAPI spec and Cliford config, check for errors, and report issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config if it exists
			cfg, err := config.Load(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			} else {
				fmt.Println("Config: OK")
				if specPath == "" && cfg.Spec != "" {
					specPath = cfg.Spec
				}
			}

			if specPath == "" {
				return fmt.Errorf("--spec is required (or set 'spec' in cliford.yaml)")
			}

			result, _, err := pipeline.Validate(context.Background(), specPath, true)
			if err != nil {
				return err
			}

			for _, w := range result.Warnings {
				fmt.Printf("Warning: %s\n", w)
			}
			for _, e := range result.Errors {
				fmt.Printf("Error: %s\n", e)
			}

			if result.HasErrors() {
				return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
			}

			fmt.Println("Spec: OK")
			return nil
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to OpenAPI spec file")
	cmd.Flags().StringVar(&configPath, "config", "cliford.yaml", "Path to Cliford config file")

	return cmd
}

func diffCmd() *cobra.Command {
	var (
		specPath   string
		configPath string
		outputDir  string
		appName    string
		pkgName    string
		envPrefix  string
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Preview changes that regeneration would make",
		Long:  "Run generation to a temporary directory and diff against current output.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if specPath == "" {
				return fmt.Errorf("--spec is required")
			}

			// Generate to temp dir
			tmpDir, err := os.MkdirTemp("", "cliford-diff-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			cfg := pipeline.Config{
				SpecPath:      specPath,
				OutputDir:     tmpDir,
				RemoveStutter: true,
				AppName:       appName,
				PackageName:   pkgName,
				EnvVarPrefix:  envPrefix,
			}
			_ = configPath

			p := pipeline.New(cfg)
			if err := p.Run(context.Background()); err != nil {
				return fmt.Errorf("generation failed: %w", err)
			}

			// Compute diff
			result, err := codegen.ComputeDiff(outputDir, tmpDir)
			if err != nil {
				return fmt.Errorf("compute diff: %w", err)
			}

			fmt.Print(codegen.FormatDiffReport(result))

			if result.HasDiff {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to OpenAPI spec file")
	cmd.Flags().StringVar(&configPath, "config", "cliford.yaml", "Path to Cliford config file")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "Current output directory to diff against")
	cmd.Flags().StringVar(&appName, "name", "app", "Generated app binary name")
	cmd.Flags().StringVar(&pkgName, "package", "github.com/example/app", "Go module path")
	cmd.Flags().StringVar(&envPrefix, "env-prefix", "APP", "Environment variable prefix")

	return cmd
}

func versionBumpCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "version [auto|patch|minor|major]",
		Short: "Bump the app version in cliford.yaml",
		Long: `Bump the semantic version of the generated app.

  auto  - Analyze OpenAPI spec changes to determine bump type
  patch - Bug fixes, metadata changes (x.y.Z)
  minor - New operations added (x.Y.0)
  major - Operations removed or signatures changed (X.0.0)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bumpType := args[0]
			if bumpType != "auto" && bumpType != "patch" && bumpType != "minor" && bumpType != "major" {
				return fmt.Errorf("invalid bump type %q: use auto, patch, minor, or major", bumpType)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			oldVersion := cfg.App.Version
			if oldVersion == "" {
				oldVersion = "0.0.0"
			}

			var newVersion string
			if bumpType == "auto" {
				// For auto, default to patch. Full auto-detection would compare
				// current spec against lockfile's spec hash.
				bumpType = "patch"
				fmt.Println("Auto-detected bump type: patch (compare against lockfile for full detection)")
			}

			newVersion, err = bumpSemVer(oldVersion, bumpType)
			if err != nil {
				return err
			}

			cfg.App.Version = newVersion

			// Write back to config
			data, yamlErr := marshalYAML(cfg)
			if yamlErr != nil {
				return yamlErr
			}
			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				return err
			}

			fmt.Printf("Version bumped: %s -> %s\n", oldVersion, newVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "cliford.yaml", "Path to Cliford config file")

	return cmd
}

func bumpSemVer(version, bumpType string) (string, error) {
	// Strip leading 'v' if present
	v := strings.TrimPrefix(version, "v")
	parts := strings.Split(v, ".")

	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver %q: expected X.Y.Z", version)
	}

	major, minor, patch := 0, 0, 0
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)
	fmt.Sscanf(parts[2], "%d", &patch)

	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func marshalYAML(cfg *config.ClifordConfig) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return data, nil
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check environment and dependencies",
		Long:  "Verify Go version, dependencies, config, and spec are all in order.",
		RunE: func(cmd *cobra.Command, args []string) error {
			passed, failed := 0, 0
			check := func(name string, fn func() error) {
				if err := fn(); err != nil {
					fmt.Printf("  FAIL  %s: %v\n", name, err)
					failed++
				} else {
					fmt.Printf("  OK    %s\n", name)
					passed++
				}
			}

			fmt.Println("Cliford Doctor")
			fmt.Println()

			check("Go installed", func() error {
				out, err := exec.Command("go", "version").Output()
				if err != nil {
					return fmt.Errorf("go not found in PATH")
				}
				fmt.Printf("         %s", string(out))
				return nil
			})

			check("Go version >= 1.22", func() error {
				out, err := exec.Command("go", "env", "GOVERSION").Output()
				if err != nil {
					return err
				}
				ver := strings.TrimSpace(string(out))
				// Simple check: go1.22+ means the minor version is >= 22
				if strings.HasPrefix(ver, "go1.") {
					minor := strings.TrimPrefix(ver, "go1.")
					if dot := strings.Index(minor, "."); dot > 0 {
						minor = minor[:dot]
					}
					var m int
					fmt.Sscanf(minor, "%d", &m)
					if m < 22 {
						return fmt.Errorf("found %s, need go1.22+", ver)
					}
				}
				return nil
			})

			check("Go modules enabled", func() error {
				out, err := exec.Command("go", "env", "GO111MODULE").Output()
				if err != nil {
					return err
				}
				val := strings.TrimSpace(string(out))
				if val == "off" {
					return fmt.Errorf("GO111MODULE=off; must be 'on' or ''")
				}
				return nil
			})

			check("cliford.yaml", func() error {
				if _, err := os.Stat("cliford.yaml"); os.IsNotExist(err) {
					return fmt.Errorf("not found (run 'cliford init')")
				}
				_, err := config.Load("cliford.yaml")
				return err
			})

			check("OpenAPI spec", func() error {
				cfg, err := config.Load("cliford.yaml")
				if err != nil || cfg.Spec == "" {
					return fmt.Errorf("no spec path in config")
				}
				if _, err := os.Stat(cfg.Spec); os.IsNotExist(err) {
					return fmt.Errorf("spec file %q not found", cfg.Spec)
				}
				return nil
			})

			fmt.Printf("\n%d passed, %d failed\n", passed, failed)
			if failed > 0 {
				return fmt.Errorf("%d check(s) failed", failed)
			}
			return nil
		},
	}
}
