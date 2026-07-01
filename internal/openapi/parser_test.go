package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAllowsInvalidSchemaExample(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Example API
  version: 1.0.0
paths:
  /discover/movies:
    get:
      operationId: discoverMovies
      tags:
        - search
      parameters:
        - in: query
          name: genre
          schema:
            type: string
            example: 18
      responses:
        "200":
          description: ok
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	reg, err := NewParser(specPath).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if got := reg.Operations[0].Parameters[0].Example; got != "18" {
		t.Fatalf("example = %q, want %q", got, "18")
	}
}

func TestParseAllowsNullableSiblingOnRef(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Example API
  version: 1.0.0
paths:
  /keyword/{keywordId}:
    get:
      operationId: getKeyword
      tags:
        - tmdb
      parameters:
        - in: path
          name: keywordId
          required: true
          schema:
            type: number
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Keyword"
                nullable: true
components:
  schemas:
    Keyword:
      type: object
      properties:
        id:
          type: number
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	if _, err := NewParser(specPath).Parse(context.Background()); err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
}

func TestParseAllowsArraySchemaWithoutItems(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Example API
  version: 1.0.0
paths:
  /settings/cache:
    get:
      operationId: getCache
      tags:
        - settings
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  entries:
                    type: array
components: {}
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	reg, err := NewParser(specPath).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	entries := reg.Operations[0].Responses[200].Schema.Properties["entries"]
	if entries.Type != "array" {
		t.Fatalf("entries type = %q, want array", entries.Type)
	}
	if entries.Items == nil {
		t.Fatal("entries items is nil, want unconstrained item schema")
	}
}

func TestParseAllowsRecursiveSchemas(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Example API
  version: 1.0.0
paths:
  /nodes:
    post:
      operationId: createNode
      tags:
        - nodes
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Node"
      responses:
        "200":
          description: ok
components:
  schemas:
    Node:
      type: object
      properties:
        name:
          type: string
        parent:
          $ref: "#/components/schemas/Node"
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	reg, err := NewParser(specPath).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	parent := reg.Operations[0].RequestBody.Schema.Properties["parent"]
	if parent.Type != "object" {
		t.Fatalf("parent type = %q, want object", parent.Type)
	}
	if len(parent.Properties) != 0 {
		t.Fatal("recursive parent schema should be represented without expanding forever")
	}
}

func TestDeriveOperationIDIncludesPathParameterNames(t *testing.T) {
	withoutParam := deriveOperationID("post", "/auth/reset-password")
	withParam := deriveOperationID("post", "/auth/reset-password/{guid}")

	if withoutParam == withParam {
		t.Fatalf("derived operation IDs should differ, both were %q", withoutParam)
	}
	if withParam != "postAuthResetPasswordGuid" {
		t.Fatalf("withParam = %q, want %q", withParam, "postAuthResetPasswordGuid")
	}
}

func TestParseSecuritySchemesPreservesAPIKeyParameterName(t *testing.T) {
	spec := []byte(`
openapi: 3.0.2
info:
  title: Example API
  version: 1.0.0
components:
  securitySchemes:
    apiKey:
      type: apiKey
      in: header
      name: X-Api-Key
paths:
  /status:
    get:
      operationId: getStatus
      responses:
        "200":
          description: ok
`)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	reg, err := NewParser(specPath).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	scheme := reg.SecuritySchemes["apiKey"]
	if scheme.ParamName != "X-Api-Key" {
		t.Fatalf("ParamName = %q, want %q", scheme.ParamName, "X-Api-Key")
	}
}
