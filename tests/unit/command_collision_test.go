package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-inconvenience-store/cliford/internal/pipeline"
)

func TestGeneratedSpecTagsConflictingWithBuiltinCommandsAreNamespaced(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Command Collision API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /auth/me:
    get:
      operationId: getAuthMe
      summary: Get current API user
      tags:
        - auth
      parameters:
        - in: query
          name: include
          schema:
            type: string
          example: profile
      responses:
        "200":
          description: ok
  /pets:
    get:
      operationId: listPets
      summary: List pets
      tags:
        - pets
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
		AppName:       "collide",
		PackageName:   "github.com/test/collide",
		EnvVarPrefix:  "COLLIDE",
	})
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	rootGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/root.go"))
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	root := string(rootGo)
	if !strings.Contains(root, `root.AddCommand(apiCmd())`) {
		t.Fatalf("expected root to add api namespace command")
	}
	if !strings.Contains(root, `root.AddCommand(authCmd())`) {
		t.Fatalf("expected built-in auth command to remain at root")
	}
	if strings.Contains(root, `root.AddCommand(apiAuthCmd())`) {
		t.Fatalf("spec auth command should be added under api, not root")
	}
	if !strings.Contains(root, `api.AddCommand(apiAuthCmd())`) {
		t.Fatalf("expected spec auth tag to be added under api namespace")
	}
	if !strings.Contains(root, `root.AddCommand(petsCmd())`) {
		t.Fatalf("non-conflicting spec tag should remain at root")
	}

	apiAuthGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/api_auth.go"))
	if err != nil {
		t.Fatalf("read api_auth.go: %v", err)
	}
	apiAuth := string(apiAuthGo)
	if !strings.Contains(apiAuth, `func apiAuthCmd() *cobra.Command`) {
		t.Fatalf("expected namespaced Go function for spec auth tag")
	}
	if !strings.Contains(apiAuth, `Use:   "auth"`) {
		t.Fatalf("expected API child command to still be named auth")
	}
	if strings.Contains(apiAuth, `func authCmd() *cobra.Command`) {
		t.Fatalf("spec auth tag must not generate built-in auth function name")
	}
	if !strings.Contains(apiAuth, `Example: "  api auth get-auth-me`) {
		t.Fatalf("operation examples should include api namespace")
	}

	if _, err := os.Stat(filepath.Join(outputDir, "internal/cli/auth.go")); err != nil {
		t.Fatalf("built-in auth.go should still be generated: %v", err)
	}
}
