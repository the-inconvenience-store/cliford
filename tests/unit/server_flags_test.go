package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-inconvenience-store/cliford/internal/config"
	"github.com/the-inconvenience-store/cliford/internal/pipeline"
)

func TestGenerateUsesConfiguredServerDefault(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	flags := config.DefaultFlagsConfig()
	flags.Server.Default = "https://api.example.com/v1"

	cfg := pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "petstore",
		PackageName:   "github.com/test/petstore",
		EnvVarPrefix:  "PET",
		CLIFlags:      flags,
	}

	p := pipeline.New(cfg)
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	rootGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/root.go"))
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	if !strings.Contains(string(rootGo), `pf.StringVar(&serverURL, "server", "https://api.example.com/v1", "Override API server URL")`) {
		t.Fatalf("root.go did not use configured server default")
	}
}

func TestGeneratedMainAppliesConfigServerURLAfterRootFlags(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	p := pipeline.New(pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "petstore",
		PackageName:   "github.com/test/petstore",
		EnvVarPrefix:  "PET",
	})
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	mainGo, err := os.ReadFile(filepath.Join(outputDir, "cmd/petstore/main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	got := string(mainGo)
	rootIdx := strings.Index(got, `rootCmd := cli.RootCmd("petstore"`)
	setIdx := strings.Index(got, `cli.SetDefaultServerURL(serverOverride)`)
	execIdx := strings.Index(got, `rootCmd.Execute()`)
	if rootIdx < 0 || setIdx < 0 || execIdx < 0 {
		t.Fatalf("generated main.go missing expected root/server/execute wiring")
	}
	if !(rootIdx < setIdx && setIdx < execIdx) {
		t.Fatalf("server_url must be applied after RootCmd registers flag defaults and before Execute parses CLI flags")
	}
	if !strings.Contains(got, `fmt.Fprintln(os.Stderr, err)`) {
		t.Fatalf("generated main.go should print Execute errors")
	}
}

func TestGeneratedConfigSetCreatesConfigDir(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	p := pipeline.New(pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "petstore",
		PackageName:   "github.com/test/petstore",
		EnvVarPrefix:  "PET",
	})
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	configGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/config_cmd.go"))
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	if !strings.Contains(string(configGo), `os.MkdirAll(filepath.Dir(configPath), 0o700)`) {
		t.Fatalf("config set should create the config directory before writing")
	}
}
