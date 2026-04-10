// Package sdk handles generating the Go SDK layer from OpenAPI specifications
// using oapi-codegen as a library.
package sdk

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	oapiutil "github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
)

// Generator produces Go SDK code from an OpenAPI spec using oapi-codegen.
type Generator struct {
	specPath   string
	outputDir  string
	pkgName    string
}

// NewGenerator creates an SDK generator.
func NewGenerator(specPath, outputDir, pkgName string) *Generator {
	return &Generator{
		specPath:  specPath,
		outputDir: outputDir,
		pkgName:   pkgName,
	}
}

// Generate produces the SDK files: types and client code.
func (g *Generator) Generate() error {
	if err := os.MkdirAll(g.outputDir, 0o755); err != nil {
		return fmt.Errorf("create SDK output dir: %w", err)
	}

	swagger, err := oapiutil.LoadSwagger(g.specPath)
	if err != nil {
		return fmt.Errorf("load spec for SDK generation: %w", err)
	}

	// Generate types + client in a single file
	code, err := g.generateCode(swagger)
	if err != nil {
		return fmt.Errorf("generate SDK code: %w", err)
	}

	outputPath := filepath.Join(g.outputDir, "sdk.gen.go")
	if err := os.WriteFile(outputPath, []byte(code), 0o644); err != nil {
		return fmt.Errorf("write SDK file: %w", err)
	}

	return nil
}

func (g *Generator) generateCode(swagger *openapi3.T) (string, error) {
	opts := codegen.Configuration{
		PackageName: g.pkgName,
		Generate: codegen.GenerateOptions{
			Models:       true,
			Client:       true,
			EmbeddedSpec: false,
		},
		OutputOptions: codegen.OutputOptions{
			SkipPrune: false,
		},
	}

	code, err := codegen.Generate(swagger, opts)
	if err != nil {
		return "", fmt.Errorf("oapi-codegen generation failed: %w", err)
	}

	return code, nil
}
