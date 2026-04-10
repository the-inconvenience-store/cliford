package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Context provides information to hooks about the current pipeline state.
type Context struct {
	SpecPath  string
	OutputDir string
	StageName string
	AppName   string
}

// RunLifecycle executes all hooks registered at the given point.
// Hooks run sequentially. A non-zero exit code fails the pipeline.
func RunLifecycle(reg *Registry, point HookPoint, ctx Context) error {
	hooks := reg.Get(point)
	if len(hooks) == 0 {
		return nil
	}

	for _, h := range hooks {
		if err := runShellCommand(h.Command, ctx); err != nil {
			return fmt.Errorf("hook %s failed (%q): %w", point, h.Command, err)
		}
	}
	return nil
}

func runShellCommand(command string, ctx Context) error {
	// Expand context variables in the command
	expanded := command
	expanded = strings.ReplaceAll(expanded, "$SPEC_PATH", ctx.SpecPath)
	expanded = strings.ReplaceAll(expanded, "$OUTPUT_DIR", ctx.OutputDir)
	expanded = strings.ReplaceAll(expanded, "$STAGE", ctx.StageName)
	expanded = strings.ReplaceAll(expanded, "$APP_NAME", ctx.AppName)

	cmd := exec.Command("sh", "-c", expanded)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = ctx.OutputDir

	return cmd.Run()
}
