package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cliford/cliford/internal/pipeline"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func TestGoldenFiles(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}

	goldenDir, err := filepath.Abs("../../testdata/golden")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()

	cfg := pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "petstore",
		PackageName:   "github.com/test/petstore",
		EnvVarPrefix:  "PET",
	}

	p := pipeline.New(cfg)
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Files to compare against golden copies
	goldenFiles := []string{
		"internal/cli/root.go",
	}

	for _, relPath := range goldenFiles {
		t.Run(relPath, func(t *testing.T) {
			actual, err := os.ReadFile(filepath.Join(outputDir, relPath))
			if err != nil {
				t.Fatalf("read generated file: %v", err)
			}

			goldenPath := filepath.Join(goldenDir, strings.ReplaceAll(relPath, "/", "_"))

			if updateGolden {
				os.MkdirAll(goldenDir, 0o755)
				if err := os.WriteFile(goldenPath, actual, 0o644); err != nil {
					t.Fatalf("update golden file: %v", err)
				}
				t.Logf("Updated golden file: %s", goldenPath)
				return
			}

			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				if os.IsNotExist(err) {
					t.Skipf("golden file %s not found (run with UPDATE_GOLDEN=1 to create)", goldenPath)
					return
				}
				t.Fatalf("read golden file: %v", err)
			}

			if string(actual) != string(expected) {
				t.Errorf("generated output differs from golden file %s\n"+
					"Run with UPDATE_GOLDEN=1 to refresh golden files", goldenPath)
			}
		})
	}
}
