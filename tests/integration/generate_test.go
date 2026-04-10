package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cliford/cliford/internal/pipeline"
)

func TestGeneratePetstore(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/petstore.yaml")
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

	// Verify expected files exist
	expectedFiles := []string{
		"cmd/petstore/main.go",
		"go.mod",
		"internal/cli/root.go",
		"internal/cli/pets.go",
		"internal/cli/users.go",
		"internal/cli/system.go",
		"internal/sdk/sdk.gen.go",
		"internal/config/config.go",
		"internal/auth/middleware.go",
		"internal/auth/keychain.go",
		"internal/hybrid/mode.go",
		"internal/sdk/pagination.go",
		"internal/sdk/retry.go",
		"internal/sdk/errors.go",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file not generated: %s", f)
		}
	}

	// Run go mod tidy + go build on the generated output
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = outputDir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = outputDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	vet := exec.Command("go", "vet", "./...")
	vet.Dir = outputDir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("go vet failed: %v\n%s", err, out)
	}

	// Build binary and test --help
	binary := filepath.Join(outputDir, "petstore-test")
	buildBin := exec.Command("go", "build", "-o", binary, "./cmd/petstore/")
	buildBin.Dir = outputDir
	if out, err := buildBin.CombinedOutput(); err != nil {
		t.Fatalf("go build binary failed: %v\n%s", err, out)
	}

	helpOut, err := exec.Command(binary, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, helpOut)
	}

	helpStr := string(helpOut)
	for _, expected := range []string{"pets", "users", "system", "auth", "config"} {
		if !strings.Contains(helpStr, expected) {
			t.Errorf("--help output missing %q", expected)
		}
	}

	// Verify stutter removal: should be "pets list" not "pets list-pets"
	petsHelp, _ := exec.Command(binary, "pets", "--help").CombinedOutput()
	if strings.Contains(string(petsHelp), "list-pets") {
		t.Error("stutter removal not applied: found 'list-pets' instead of 'list'")
	}
	if !strings.Contains(string(petsHelp), "list") {
		t.Error("'list' command not found under pets")
	}
}

func TestGenerateMultiAuth(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/complex-auth.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()

	cfg := pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "multiauth",
		PackageName:   "github.com/test/multiauth",
		EnvVarPrefix:  "MA",
	}

	p := pipeline.New(cfg)
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Verify OAuth file generated (spec has oauth2 scheme)
	oauthPath := filepath.Join(outputDir, "internal", "auth", "oauth.go")
	if _, err := os.Stat(oauthPath); os.IsNotExist(err) {
		t.Error("OAuth file not generated for spec with oauth2 scheme")
	}

	// Compile
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = outputDir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = outputDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}
