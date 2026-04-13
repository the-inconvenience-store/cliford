package unit

import (
	"strings"
	"testing"
)

// resolveAliases mirrors the logic emitted into generated alias.go.
// It is reproduced here so unit tests run without invoking code generation.
func resolveAliases(args []string, aliases map[string]string) []string {
	if len(args) < 2 {
		return args
	}
	expansion, ok := aliases[args[1]]
	if !ok {
		return args
	}
	expanded := strings.Fields(expansion)
	if len(expanded) == 0 {
		return args
	}
	result := make([]string, 0, 1+len(expanded)+len(args[2:]))
	result = append(result, args[0])
	result = append(result, expanded...)
	result = append(result, args[2:]...)
	return result
}

func TestResolveAliases_NoMatch(t *testing.T) {
	args := []string{"myapp", "pets", "list"}
	aliases := map[string]string{"lp": "pets list --limit 10"}
	got := resolveAliases(args, aliases)
	if len(got) != 3 || got[1] != "pets" {
		t.Errorf("expected unchanged args, got %v", got)
	}
}

func TestResolveAliases_Match(t *testing.T) {
	args := []string{"myapp", "lp"}
	aliases := map[string]string{"lp": "pets list --limit 10"}
	got := resolveAliases(args, aliases)
	want := []string{"myapp", "pets", "list", "--limit", "10"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: want %q, got %q", i, v, got[i])
		}
	}
}

func TestResolveAliases_UserArgsAppended(t *testing.T) {
	// Extra user flags come AFTER alias tokens so they can override alias defaults.
	args := []string{"myapp", "lp", "--output-format", "yaml"}
	aliases := map[string]string{"lp": "pets list --limit 10"}
	got := resolveAliases(args, aliases)
	want := []string{"myapp", "pets", "list", "--limit", "10", "--output-format", "yaml"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: want %q, got %q", i, v, got[i])
		}
	}
}

func TestResolveAliases_UserFlagOverridesAlias(t *testing.T) {
	// When the user supplies --limit after the alias, the last --limit wins in cobra.
	args := []string{"myapp", "lp", "--limit", "99"}
	aliases := map[string]string{"lp": "pets list --limit 10"}
	got := resolveAliases(args, aliases)
	// The alias --limit 10 comes before user's --limit 99; cobra uses the last value.
	want := []string{"myapp", "pets", "list", "--limit", "10", "--limit", "99"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: want %q, got %q", i, v, got[i])
		}
	}
}

func TestResolveAliases_EmptyExpansion(t *testing.T) {
	// An empty-string expansion should be treated as no-op.
	args := []string{"myapp", "lp"}
	aliases := map[string]string{"lp": ""}
	got := resolveAliases(args, aliases)
	if len(got) != 2 || got[1] != "lp" {
		t.Errorf("expected unchanged args for empty expansion, got %v", got)
	}
}

func TestResolveAliases_WhitespaceOnlyExpansion(t *testing.T) {
	// Whitespace-only expansion tokenises to zero fields → no-op.
	args := []string{"myapp", "lp"}
	aliases := map[string]string{"lp": "   "}
	got := resolveAliases(args, aliases)
	if len(got) != 2 || got[1] != "lp" {
		t.Errorf("expected unchanged args for whitespace expansion, got %v", got)
	}
}

func TestResolveAliases_TooFewArgs(t *testing.T) {
	// Only the program name — nothing to resolve.
	args := []string{"myapp"}
	aliases := map[string]string{"lp": "pets list"}
	got := resolveAliases(args, aliases)
	if len(got) != 1 {
		t.Errorf("expected unchanged single-element args, got %v", got)
	}
}

func TestResolveAliases_EmptyArgs(t *testing.T) {
	got := resolveAliases([]string{}, map[string]string{"lp": "pets list"})
	if len(got) != 0 {
		t.Errorf("expected empty slice back, got %v", got)
	}
}

func TestResolveAliases_NoAliasesConfigured(t *testing.T) {
	args := []string{"myapp", "lp", "--limit", "5"}
	got := resolveAliases(args, map[string]string{})
	if len(got) != 4 || got[1] != "lp" {
		t.Errorf("expected unchanged args with empty aliases map, got %v", got)
	}
}

func TestResolveAliases_ProgramNameIsNeverAliased(t *testing.T) {
	// args[0] is the program path — it is never checked against aliases.
	args := []string{"lp"}
	aliases := map[string]string{"lp": "pets list"}
	got := resolveAliases(args, aliases)
	if len(got) != 1 || got[0] != "lp" {
		t.Errorf("expected args[0] unchanged, got %v", got)
	}
}

func TestResolveAliases_MultiTokenAlias(t *testing.T) {
	// Deep command path with flags.
	args := []string{"myapp", "nc"}
	aliases := map[string]string{"nc": "pets create --name Whiskers --species cat"}
	got := resolveAliases(args, aliases)
	want := []string{"myapp", "pets", "create", "--name", "Whiskers", "--species", "cat"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: want %q, got %q", i, v, got[i])
		}
	}
}
