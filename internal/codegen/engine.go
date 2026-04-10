// Package codegen provides the template engine and code generation utilities
// for producing Go source files from the Operation Registry.
package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Engine loads and executes Go text templates to produce generated source files.
type Engine struct {
	templateDir string
	funcMap     template.FuncMap
}

// NewEngine creates a template engine that loads templates from the given directory.
func NewEngine(templateDir string) *Engine {
	return &Engine{
		templateDir: templateDir,
		funcMap:     defaultFuncMap(),
	}
}

// RenderToFile executes a template with the given data and writes the formatted
// output to the specified file path. If the file has a .go extension, the output
// is formatted with gofmt.
func (e *Engine) RenderToFile(templatePath string, data any, outputPath string) error {
	content, err := e.Render(templatePath, data)
	if err != nil {
		return fmt.Errorf("render template %s: %w", templatePath, err)
	}

	if strings.HasSuffix(outputPath, ".go") {
		formatted, fmtErr := format.Source(content)
		if fmtErr != nil {
			// Write unformatted so the user can debug
			if writeErr := writeFile(outputPath, content); writeErr != nil {
				return fmt.Errorf("write unformatted output: %w", writeErr)
			}
			return fmt.Errorf("gofmt failed for %s: %w (unformatted file written)", outputPath, fmtErr)
		}
		content = formatted
	}

	return writeFile(outputPath, content)
}

// Render executes a template and returns the raw bytes.
func (e *Engine) Render(templatePath string, data any) ([]byte, error) {
	fullPath := filepath.Join(e.templateDir, templatePath)

	tmplContent, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", fullPath, err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).
		Funcs(e.funcMap).
		Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", templatePath, err)
	}

	return buf.Bytes(), nil
}

// RenderString executes an inline template string with the given data.
func (e *Engine) RenderString(name, tmplStr string, data any) ([]byte, error) {
	tmpl, err := template.New(name).
		Funcs(e.funcMap).
		Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parse inline template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute inline template %s: %w", name, err)
	}

	return buf.Bytes(), nil
}

func writeFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	return os.WriteFile(path, content, 0o644)
}

// AddFunc registers a custom template function.
func (e *Engine) AddFunc(name string, fn any) {
	e.funcMap[name] = fn
}

func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"toLower":      strings.ToLower,
		"toUpper":      strings.ToUpper,
		"title":        strings.Title,
		"kebabCase":    toKebab,
		"camelCase":    toCamel,
		"pascalCase":   toPascal,
		"snakeCase":    toSnake,
		"join":         strings.Join,
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"trimPrefix":   strings.TrimPrefix,
		"trimSuffix":   strings.TrimSuffix,
		"contains":     strings.Contains,
		"replace":      strings.ReplaceAll,
		"quote":        func(s string) string { return fmt.Sprintf("%q", s) },
		"add":          func(a, b int) int { return a + b },
		"sub":          func(a, b int) int { return a - b },
		"schemaToGoType": schemaToGoType,
		"defaultStr": func(v any) string {
			if v == nil {
				return ""
			}
			return fmt.Sprintf("%v", v)
		},
		"defaultInt": func(v any) int {
			if v == nil {
				return 0
			}
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			default:
				return 0
			}
		},
		"defaultInt64": func(v any) int64 {
			if v == nil {
				return 0
			}
			switch n := v.(type) {
			case float64:
				return int64(n)
			case int64:
				return n
			default:
				return 0
			}
		},
		"serverURLs": func() []string {
			// Placeholder — overridden per-render via AddFunc
			return []string{"http://localhost:8080"}
		},
	}
}

// schemaToGoType maps registry SchemaMeta to Go type strings for templates.
func schemaToGoType(s any) string {
	// Handle the registry.SchemaMeta type
	type schemaMeta interface {
		GetType() string
		GetFormat() string
	}
	// Use reflection-free approach: templates pass the struct directly
	switch v := s.(type) {
	case struct{ Type, Format string }:
		return goTypeFromSchema(v.Type, v.Format)
	default:
		return "string"
	}
}

func goTypeFromSchema(typ, format string) string {
	switch typ {
	case "integer":
		if format == "int64" {
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

func toKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r + 32)
		} else if r == '_' || r == ' ' {
			result.WriteByte('-')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func toCamel(s string) string {
	parts := splitWords(s)
	if len(parts) == 0 {
		return s
	}
	var result strings.Builder
	result.WriteString(strings.ToLower(parts[0]))
	for _, p := range parts[1:] {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
		}
	}
	return result.String()
}

func toPascal(s string) string {
	parts := splitWords(s)
	var result strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
		}
	}
	return result.String()
}

func toSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32)
		} else if r == '-' || r == ' ' {
			result.WriteByte('_')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
}
