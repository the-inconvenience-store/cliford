// Package hybrid handles mode detection and the adapter pattern for
// routing between CLI, TUI, and headless execution modes.
package hybrid

import (
	"os"
	"path/filepath"
	"strings"
)

// Mode represents the execution mode of the generated app.
type Mode string

const (
	ModeCLI      Mode = "cli"
	ModeTUI      Mode = "tui"
	ModeHybrid   Mode = "hybrid"
	ModeHeadless Mode = "headless"
)

// agentEnvVars are environment variables that indicate an AI agent context.
var agentEnvVars = []string{
	"CLAUDE_CODE",        // Claude Code
	"CURSOR_SESSION_ID",  // Cursor
	"CODEX_SESSION",      // OpenAI Codex CLI
	"AIDER_MODEL",        // Aider
	"CLINE_TASK",         // Cline
	"WINDSURF_SESSION",   // Windsurf
	"COPILOT_AGENT",      // GitHub Copilot
	"AMAZON_Q_SESSION",   // Amazon Q
	"GEMINI_CODE_ASSIST", // Gemini Code Assist
	"CODY_SESSION",       // Sourcegraph Cody
}

// agentProcessNames detects agent parent processes.
var agentProcessNames = []string{
	"claude", "cursor", "codex", "aider", "windsurf",
}

// ModeDetector resolves the execution mode from flags, env, config, and auto-detection.
type ModeDetector struct {
	envPrefix string
}

// NewModeDetector creates a mode detector with the given env prefix.
func NewModeDetector(envPrefix string) *ModeDetector {
	return &ModeDetector{envPrefix: envPrefix}
}

// Detect resolves the execution mode using the precedence chain:
//  1. Explicit flags (--tui, --no-interactive, -y, --agent)
//  2. Environment variable (<PREFIX>_MODE)
//  3. Auto-detection (agent env, TTY check)
//
// Returns the resolved mode.
func (d *ModeDetector) Detect(tuiFlag, noInteractive, yesFlag, agentFlag bool) Mode {
	// Priority 1: explicit flags
	if agentFlag {
		return ModeHeadless
	}
	if tuiFlag {
		return ModeTUI
	}
	if noInteractive || yesFlag {
		return ModeHeadless
	}

	// Priority 2: environment variable
	envMode := os.Getenv(d.envPrefix + "_MODE")
	switch strings.ToLower(envMode) {
	case "tui":
		return ModeTUI
	case "cli":
		return ModeCLI
	case "headless":
		return ModeHeadless
	}

	// Priority 3: auto-detection
	if d.isAgentEnvironment() {
		return ModeHeadless
	}
	if !d.isTTY() {
		return ModeHeadless
	}

	// Default: hybrid (CLI with inline TUI prompts)
	return ModeHybrid
}

// isAgentEnvironment checks for known AI agent environment indicators.
func (d *ModeDetector) isAgentEnvironment() bool {
	for _, envVar := range agentEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	// Check parent process name
	ppidPath := parentProcessPath()
	if ppidPath != "" {
		base := strings.ToLower(filepath.Base(ppidPath))
		for _, name := range agentProcessNames {
			if strings.Contains(base, name) {
				return true
			}
		}
	}

	return false
}

// isTTY checks if stdin and stdout are connected to a terminal.
func (d *ModeDetector) isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return false // stdin is piped
	}

	fo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if fo.Mode()&os.ModeCharDevice == 0 {
		return false // stdout is piped
	}

	return true
}

// parentProcessPath attempts to read the parent process executable path.
func parentProcessPath() string {
	ppid := os.Getppid()
	// On macOS/Linux, try /proc/<pid>/exe
	exePath, err := os.Readlink("/proc/" + itoa(ppid) + "/exe")
	if err == nil {
		return exePath
	}
	return ""
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
