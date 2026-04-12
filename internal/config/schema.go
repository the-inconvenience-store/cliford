package config

import (
	"encoding/json"
	"fmt"
)

// GenerateJSONSchema produces a JSON Schema for cliford.yaml.
// This enables IDE autocompletion and config validation.
func GenerateJSONSchema() ([]byte, error) {
	schema := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"title":       "Cliford Configuration",
		"description": "Configuration schema for cliford.yaml",
		"type":        "object",
		"properties": map[string]any{
			"version": map[string]any{
				"type":        "string",
				"description": "Config schema version",
				"default":     "1",
			},
			"spec": map[string]any{
				"type":        "string",
				"description": "Path to OpenAPI spec file",
			},
			"app": map[string]any{
				"type":        "object",
				"description": "Generated app identity",
				"properties": map[string]any{
					"name":         map[string]any{"type": "string", "description": "Binary name"},
					"package":      map[string]any{"type": "string", "description": "Go module path"},
					"envVarPrefix": map[string]any{"type": "string", "description": "Environment variable prefix"},
					"version":      map[string]any{"type": "string", "description": "App semantic version"},
					"description":  map[string]any{"type": "string", "description": "App description"},
				},
			},
			"generation": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"mode": map[string]any{
						"type":        "string",
						"enum":        []string{"pure-cli", "pure-tui", "hybrid"},
						"default":     "hybrid",
						"description": "Generation mode",
					},
					"sdk": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"generator": map[string]any{"type": "string", "default": "oapi-codegen"},
							"outputDir": map[string]any{"type": "string", "default": "internal/sdk"},
							"package":   map[string]any{"type": "string", "default": "sdk"},
						},
					},
					"cli": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"outputDir":     map[string]any{"type": "string", "default": "internal/cli"},
							"removeStutter": map[string]any{"type": "boolean", "default": true},
						},
					},
					"tui": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"enabled":   map[string]any{"type": "boolean", "default": false},
							"outputDir": map[string]any{"type": "string", "default": "internal/tui"},
						},
					},
				},
			},
			"theme": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"colors": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"primary":    map[string]any{"type": "string", "format": "color"},
							"secondary":  map[string]any{"type": "string", "format": "color"},
							"accent":     map[string]any{"type": "string", "format": "color"},
							"background": map[string]any{"type": "string", "format": "color"},
							"text":       map[string]any{"type": "string", "format": "color"},
							"dimmed":     map[string]any{"type": "string", "format": "color"},
							"error":      map[string]any{"type": "string", "format": "color"},
							"success":    map[string]any{"type": "string", "format": "color"},
							"warning":    map[string]any{"type": "string", "format": "color"},
						},
					},
					"borders": map[string]any{"type": "string", "enum": []string{"normal", "rounded", "thick", "double"}},
					"spinner": map[string]any{"type": "string"},
				},
			},
			"features": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"hooks": map[string]any{
						"type":        "object",
						"description": "Runtime hooks baked into the generated app at generation time",
						"properties": map[string]any{
							"beforeRequest": map[string]any{
								"type":        "array",
								"description": "Hooks executed before every HTTP request",
								"items":       runtimeHookSchema(),
							},
							"afterResponse": map[string]any{
								"type":        "array",
								"description": "Hooks executed after every HTTP response",
								"items":       runtimeHookSchema(),
							},
						},
					},
				},
			},
			"operations": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"cli": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"aliases":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
								"group":          map[string]any{"type": "string"},
								"hidden":         map[string]any{"type": "boolean"},
								"confirm":        map[string]any{"type": "boolean"},
								"confirmMessage": map[string]any{"type": "string"},
							},
						},
						"tui": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"displayAs":   map[string]any{"type": "string", "enum": []string{"table", "detail", "form", "custom"}},
								"refreshable": map[string]any{"type": "boolean"},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON Schema: %w", err)
	}
	return data, nil
}

func runtimeHookSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"required": []string{"type"},
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"shell", "go-plugin"},
				"description": "Hook execution mechanism",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute (type: shell)",
			},
			"pluginPath": map[string]any{
				"type":        "string",
				"description": "Path to go-plugin binary (type: go-plugin)",
			},
		},
	}
}
