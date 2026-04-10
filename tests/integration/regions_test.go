package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cliford/cliford/internal/codegen"
	"github.com/cliford/cliford/internal/pipeline"
)

func TestCustomCodeRegionPreservation(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()

	// First generation with custom code regions enabled
	cfg := pipeline.Config{
		SpecPath:          specPath,
		OutputDir:         outputDir,
		RemoveStutter:     true,
		CustomCodeRegions: true,
		AppName:           "petstore",
		PackageName:       "github.com/test/petstore",
		EnvVarPrefix:      "PET",
	}

	p := pipeline.New(cfg)
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("first generation failed: %v", err)
	}

	// Verify region markers exist
	petsFile := filepath.Join(outputDir, "internal", "cli", "pets.go")
	content, err := os.ReadFile(petsFile)
	if err != nil {
		t.Fatalf("read pets.go: %v", err)
	}
	if !strings.Contains(string(content), "CUSTOM CODE START: listPets:pre") {
		t.Error("region marker not found in generated file")
	}

	// Inject custom code into a region
	modified := strings.Replace(string(content),
		"// --- CUSTOM CODE START: listPets:pre ---",
		"// --- CUSTOM CODE START: listPets:pre ---\n\t\t\t// Custom telemetry injection",
		1)
	if err := os.WriteFile(petsFile, []byte(modified), 0o644); err != nil {
		t.Fatalf("write modified pets.go: %v", err)
	}

	// Extract regions before regeneration
	regions, err := codegen.ExtractAllRegions(outputDir)
	if err != nil {
		t.Fatalf("extract regions: %v", err)
	}

	// Verify our custom code was extracted
	key := "internal/cli/pets.go:listPets:pre"
	region, found := regions[key]
	if !found {
		t.Fatalf("region %q not found after extraction", key)
	}
	if !strings.Contains(region.Content, "Custom telemetry") {
		t.Errorf("custom code not preserved in extracted region, got: %q", region.Content)
	}
}
