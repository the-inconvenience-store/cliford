package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/the-inconvenience-store/cliford/internal/pipeline"
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

	// Verify expected files exist (including new Phase 2-20 files)
	expectedFiles := []string{
		"cmd/petstore/main.go",
		"go.mod",
		"internal/cli/root.go",
		"internal/cli/pets.go",
		"internal/cli/users.go",
		"internal/cli/system.go",
		"internal/cli/generate_docs.go",
		"internal/sdk/sdk.gen.go",
		"internal/config/config.go",
		"internal/auth/middleware.go",
		"internal/auth/keychain.go",
		"internal/auth/resolver.go",
		"internal/auth/profiles.go",
		"internal/client/factory.go",
		"internal/hooks/runner.go",
		"internal/hybrid/mode.go",
		"internal/sdk/pagination.go",
		"internal/sdk/retry.go",
		"internal/sdk/errors.go",
		"internal/sdk/verbose.go",
		"docs/llms.txt",
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
	for _, expected := range []string{"pets", "users", "system", "auth", "config", "generate-docs", "completion"} {
		if !strings.Contains(helpStr, expected) {
			t.Errorf("--help output missing %q", expected)
		}
	}

	// Verify --verbose/-v flag is present
	if !strings.Contains(helpStr, "--verbose") && !strings.Contains(helpStr, "-v") {
		t.Error("--help missing --verbose flag")
	}

	// Verify stutter removal: should be "pets list" not "pets list-pets"
	petsHelp, _ := exec.Command(binary, "pets", "--help").CombinedOutput()
	if strings.Contains(string(petsHelp), "list-pets") {
		t.Error("stutter removal not applied: found 'list-pets' instead of 'list'")
	}
	if !strings.Contains(string(petsHelp), "list") {
		t.Error("'list' command not found under pets")
	}

	// Verify generate-docs subcommand exists
	docsHelp, _ := exec.Command(binary, "generate-docs", "--help").CombinedOutput()
	if !strings.Contains(string(docsHelp), "--format") {
		t.Error("generate-docs missing --format flag")
	}

	// Verify completion subcommand works
	compOut, err := exec.Command(binary, "completion", "bash").CombinedOutput()
	if err != nil {
		t.Errorf("completion bash failed: %v", err)
	}
	if !strings.Contains(string(compOut), "bash completion") {
		// Cobra generates bash completion with this header
		if len(compOut) < 100 {
			t.Error("completion bash output too short — likely not a valid completion script")
		}
	}

	// Verify version flag works
	verOut, _ := exec.Command(binary, "--version").CombinedOutput()
	if !strings.Contains(string(verOut), "dev") {
		t.Error("--version output missing version string")
	}
}

func TestGenerateServerVariables(t *testing.T) {
	specPath, err := filepath.Abs("../../testdata/specs/server-vars.yaml")
	if err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()

	cfg := pipeline.Config{
		SpecPath:      specPath,
		OutputDir:     outputDir,
		RemoveStutter: true,
		AppName:       "svtest",
		PackageName:   "github.com/test/svtest",
		EnvVarPrefix:  "SV",
	}

	p := pipeline.New(cfg)
	if err := p.Run(context.Background()); err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Read generated root.go and verify server variable declarations
	rootGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/root.go"))
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	rootStr := string(rootGo)

	for _, want := range []string{
		"serverVarTenant",
		"serverVarVersion",
		`"server-tenant"`,
		`"server-version"`,
	} {
		if !strings.Contains(rootStr, want) {
			t.Errorf("root.go missing %q", want)
		}
	}

	// Read generated items.go and verify substitution + enum validation
	itemsGo, err := os.ReadFile(filepath.Join(outputDir, "internal/cli/items.go"))
	if err != nil {
		t.Fatalf("read items.go: %v", err)
	}
	itemsStr := string(itemsGo)

	for _, want := range []string{
		"strings.NewReplacer",
		`"{tenant}", serverVarTenant`,
		`"{version}", serverVarVersion`,
		"slices.Contains",
		"--server-tenant",
	} {
		if !strings.Contains(itemsStr, want) {
			t.Errorf("items.go missing %q", want)
		}
	}

	// Compile the generated app
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

	// Build binary and verify --server-tenant / --server-version appear in help
	binary := filepath.Join(outputDir, "svtest-bin")
	buildBin := exec.Command("go", "build", "-o", binary, "./cmd/svtest/")
	buildBin.Dir = outputDir
	if out, err := buildBin.CombinedOutput(); err != nil {
		t.Fatalf("build binary failed: %v\n%s", err, out)
	}

	helpOut, _ := exec.Command(binary, "--help").CombinedOutput()
	helpStr := string(helpOut)
	for _, flag := range []string{"--server-tenant", "--server-version"} {
		if !strings.Contains(helpStr, flag) {
			t.Errorf("--help missing flag %q", flag)
		}
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

	// Verify resolver generated with scheme configs
	resolverPath := filepath.Join(outputDir, "internal", "auth", "resolver.go")
	if _, err := os.Stat(resolverPath); os.IsNotExist(err) {
		t.Error("Resolver file not generated")
	}

	// Verify client factory generated
	factoryPath := filepath.Join(outputDir, "internal", "client", "factory.go")
	if _, err := os.Stat(factoryPath); os.IsNotExist(err) {
		t.Error("Client factory file not generated")
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
