package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-inconvenience-store/cliford/internal/pipeline"
)

func TestGenerateDisambiguatesDuplicateParameterFlagNames(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Duplicate Param API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /discover/movies/language/{language}:
    get:
      operationId: getMoviesByLanguage
      tags:
        - search
      parameters:
        - in: path
          name: language
          required: true
          schema:
            type: string
        - in: query
          name: language
          schema:
            type: string
        - in: query
          name: page
          schema:
            type: integer
      responses:
        "200":
          description: ok
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	outputDir := t.TempDir()
	p := pipeline.New(pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "dupe",
		PackageName:   "github.com/test/dupe",
		EnvVarPrefix:  "DUPE",
	})
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	searchGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/search.go"))
	if err != nil {
		t.Fatalf("read search.go: %v", err)
	}
	got := string(searchGo)
	if strings.Count(got, "var flagLanguage string") != 1 {
		t.Fatalf("expected one path language variable, got:\n%s", got)
	}
	if !strings.Contains(got, "var flagQueryLanguage string") {
		t.Fatalf("expected duplicate query parameter to use flagQueryLanguage")
	}
	if !strings.Contains(got, `cmd.Flags().StringVar(&flagQueryLanguage, "query-language", "", "")`) {
		t.Fatalf("expected duplicate query parameter to register --query-language")
	}
	if !strings.Contains(got, `q.Set("language", flagQueryLanguage)`) {
		t.Fatalf("expected query-language flag to still set the language query parameter")
	}
}
