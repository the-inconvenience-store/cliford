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
