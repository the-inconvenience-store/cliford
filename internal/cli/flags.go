package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cliford/cliford/pkg/registry"
)

// FlagDef describes a CLI flag to be generated for an operation parameter.
type FlagDef struct {
	Name        string
	ShortName   string
	GoType      string // string, int, int64, bool, []string
	Default     any
	Description string
	Required    bool
	EnumValues  []string
	ParamIn     string // path, query, header, cookie
}

// BuildFlags converts operation parameters to flag definitions.
func BuildFlags(params []registry.ParamMeta) []FlagDef {
	var flags []FlagDef
	for _, p := range params {
		f := FlagDef{
			Name:        p.FlagName,
			GoType:      schemaToGoFlagType(p.Schema),
			Default:     p.Default,
			Description: p.Description,
			Required:    p.Required,
			ParamIn:     string(p.In),
		}

		if len(p.Enum) > 0 {
			var vals []string
			for _, e := range p.Enum {
				vals = append(vals, fmt.Sprintf("%v", e))
			}
			f.EnumValues = vals
			f.Description += " (" + strings.Join(vals, ", ") + ")"
		}

		flags = append(flags, f)
	}
	return flags
}

// schemaToGoFlagType maps OpenAPI schema types to Go flag types.
func schemaToGoFlagType(s registry.SchemaMeta) string {
	switch s.Type {
	case "integer":
		if s.Format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]string"
	default:
		return "string"
	}
}

// MergeInputs implements the three-source input precedence:
// 1. Individual flags (highest)
// 2. --body JSON payload
// 3. stdin JSON (lowest)
// Returns the merged map of parameter name -> value.
func MergeInputs(flagValues map[string]any, bodyJSON string, stdinReader io.Reader) (map[string]any, error) {
	result := make(map[string]any)

	// Layer 1: stdin (lowest priority)
	if stdinReader != nil {
		stat, _ := os.Stdin.Stat()
		if stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(stdinReader)
			if err == nil && len(data) > 0 {
				var stdinMap map[string]any
				if err := json.Unmarshal(data, &stdinMap); err == nil {
					for k, v := range stdinMap {
						result[k] = v
					}
				}
			}
		}
	}

	// Layer 2: --body JSON
	if bodyJSON != "" {
		var bodyMap map[string]any
		if err := json.Unmarshal([]byte(bodyJSON), &bodyMap); err != nil {
			return nil, fmt.Errorf("invalid --body JSON: %w", err)
		}
		for k, v := range bodyMap {
			result[k] = v
		}
	}

	// Layer 3: individual flags (highest priority)
	for k, v := range flagValues {
		result[k] = v
	}

	return result, nil
}
