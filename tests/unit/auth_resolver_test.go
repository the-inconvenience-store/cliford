package unit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-inconvenience-store/cliford/internal/pipeline"
)

func TestGeneratedResolverAddsAPIKeyMetadataToStoredCredentials(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: API Key API
  version: 1.0.0
servers:
  - url: https://api.example.com
components:
  securitySchemes:
    apiKey:
      type: apiKey
      in: header
      name: X-Api-Key
paths:
  /me:
    get:
      operationId: getMe
      tags:
        - users
      security:
        - apiKey: []
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
		AppName:       "apikey",
		PackageName:   "github.com/test/apikey",
		EnvVarPrefix:  "APIKEY",
	})
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	resolverGo, err := os.ReadFile(filepath.Join(outputDir, "internal/auth/resolver.go"))
	if err != nil {
		t.Fatalf("read resolver.go: %v", err)
	}
	got := string(resolverGo)
	for _, want := range []string{
		`resolveFromStore(scheme)`,
		`func (r *Resolver) applySchemeDefaults(cred *Credential, scheme SchemeConfig)`,
		`cred.HeaderName = scheme.HeaderName`,
		`cred.QueryParam = scheme.QueryParam`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("resolver.go missing %q", want)
		}
	}
}
