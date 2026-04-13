package unit

// template_helpers_test.go validates the logic emitted by the generator for
// applyGoTemplate, applyJSONPath, and the FormatOutput inline-expression parser.
// The helpers are reimplemented here using the same algorithm so that changes
// to the generator can be cross-checked against these tests.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/itchyny/gojq"
)

// ---- helpers that mirror the generated code ----

func testApplyGoTemplate(data any, tmplStr string) (string, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("output").Funcs(template.FuncMap{
		"json": func(v any) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
	}).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func testApplyJQ(input any, expr string) (any, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("parse jq expression %q: %w", expr, err)
	}
	iter := query.Run(input)
	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if jqErr, ok := v.(error); ok {
			return nil, jqErr
		}
		results = append(results, v)
	}
	switch len(results) {
	case 0:
		return nil, nil
	case 1:
		return results[0], nil
	default:
		return results, nil
	}
}

func testApplyJSONPath(data any, expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if len(expr) >= 2 && expr[0] == '{' && expr[len(expr)-1] == '}' {
		expr = expr[1 : len(expr)-1]
	}
	expr = strings.TrimPrefix(expr, "$")
	jqExpr := strings.ReplaceAll(expr, "[*]", "[]")
	result, err := testApplyJQ(data, jqExpr)
	if err != nil {
		return "", fmt.Errorf("jsonpath: %w", err)
	}
	if result == nil {
		return "", nil
	}
	var buf bytes.Buffer
	switch r := result.(type) {
	case []any:
		parts := make([]string, 0, len(r))
		for _, item := range r {
			switch v := item.(type) {
			case string:
				parts = append(parts, v)
			default:
				b, _ := json.Marshal(v)
				parts = append(parts, string(b))
			}
		}
		fmt.Fprintln(&buf, strings.Join(parts, " "))
	case string:
		fmt.Fprintln(&buf, r)
	default:
		b, _ := json.Marshal(result)
		fmt.Fprintln(&buf, string(b))
	}
	return buf.String(), nil
}

// parseFormatString mirrors the inline-expression splitting done inside
// FormatOutput: "go-template={{.name}}" → ("go-template", "{{.name}}")
func parseFormatString(format string) (base, inline string) {
	if idx := strings.IndexByte(format, '='); idx > 0 {
		return format[:idx], format[idx+1:]
	}
	return format, ""
}

// ---- tests ----

func TestApplyGoTemplate_SingleField(t *testing.T) {
	data := map[string]any{"name": "Fluffy", "species": "cat"}
	got, err := testApplyGoTemplate(data, "{{.name}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Fluffy" {
		t.Errorf("got %q, want %q", got, "Fluffy")
	}
}

func TestApplyGoTemplate_NestedField(t *testing.T) {
	data := map[string]any{
		"metadata": map[string]any{"name": "my-pod"},
	}
	got, err := testApplyGoTemplate(data, "{{.metadata.name}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-pod" {
		t.Errorf("got %q, want %q", got, "my-pod")
	}
}

func TestApplyGoTemplate_RangeLoop(t *testing.T) {
	data := []any{
		map[string]any{"name": "alpha"},
		map[string]any{"name": "beta"},
	}
	got, err := testApplyGoTemplate(data, `{{range .}}{{.name}}
{{end}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "alpha\nbeta\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyGoTemplate_JSONFunc(t *testing.T) {
	data := map[string]any{"tags": []any{"a", "b"}}
	got, err := testApplyGoTemplate(data, `{{json .tags}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `["a","b"]` {
		t.Errorf("got %q, want %q", got, `["a","b"]`)
	}
}

func TestApplyGoTemplate_InvalidTemplate(t *testing.T) {
	_, err := testApplyGoTemplate(nil, "{{.unclosed")
	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}
	if !strings.Contains(err.Error(), "parse template") {
		t.Errorf("error should mention 'parse template', got: %v", err)
	}
}

func TestApplyGoTemplate_EmptyTemplate(t *testing.T) {
	got, err := testApplyGoTemplate(map[string]any{"x": 1}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestApplyJSONPath_BasicDotNotation(t *testing.T) {
	data := map[string]any{"name": "Fluffy"}
	got, err := testApplyJSONPath(data, ".name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "Fluffy" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "Fluffy")
	}
}

func TestApplyJSONPath_KubectlCurlyBrace(t *testing.T) {
	data := map[string]any{"name": "Fluffy"}
	got, err := testApplyJSONPath(data, "{.name}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "Fluffy" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "Fluffy")
	}
}

func TestApplyJSONPath_DollarPrefix(t *testing.T) {
	data := map[string]any{"name": "Fluffy"}
	got, err := testApplyJSONPath(data, "$.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "Fluffy" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "Fluffy")
	}
}

func TestApplyJSONPath_WildcardArray(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"name": "alpha"},
			map[string]any{"name": "beta"},
			map[string]any{"name": "gamma"},
		},
	}
	got, err := testApplyJSONPath(data, "{.items[*].name}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// kubectl style: space-separated on a single line
	if strings.TrimSpace(got) != "alpha beta gamma" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "alpha beta gamma")
	}
}

func TestApplyJSONPath_DollarCurlyBrace(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"name": "x"},
			map[string]any{"name": "y"},
		},
	}
	got, err := testApplyJSONPath(data, "{$.items[*].name}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "x y" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "x y")
	}
}

func TestApplyJSONPath_NumericValues(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"count": 1},
			map[string]any{"count": 2},
		},
	}
	got, err := testApplyJSONPath(data, "{.items[*].count}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Numbers are JSON-marshalled
	if strings.TrimSpace(got) != "1 2" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "1 2")
	}
}

func TestApplyJSONPath_NilResult(t *testing.T) {
	data := map[string]any{"name": "Fluffy"}
	// .missing does not exist; gojq yields null which is nil
	got, err := testApplyJSONPath(data, ".missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output for nil result, got %q", got)
	}
}

func TestApplyJSONPath_ScalarString(t *testing.T) {
	data := map[string]any{"status": "running"}
	got, err := testApplyJSONPath(data, ".status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) != "running" {
		t.Errorf("got %q, want %q", strings.TrimSpace(got), "running")
	}
}

func TestParseFormatString_InlineTemplate(t *testing.T) {
	base, inline := parseFormatString("go-template={{.name}}")
	if base != "go-template" {
		t.Errorf("base: got %q, want %q", base, "go-template")
	}
	if inline != "{{.name}}" {
		t.Errorf("inline: got %q, want %q", inline, "{{.name}}")
	}
}

func TestParseFormatString_InlineJSONPath(t *testing.T) {
	base, inline := parseFormatString("jsonpath={.items[*].name}")
	if base != "jsonpath" {
		t.Errorf("base: got %q, want %q", base, "jsonpath")
	}
	if inline != "{.items[*].name}" {
		t.Errorf("inline: got %q, want %q", inline, "{.items[*].name}")
	}
}

func TestParseFormatString_NoEquals(t *testing.T) {
	base, inline := parseFormatString("json")
	if base != "json" {
		t.Errorf("base: got %q, want %q", base, "json")
	}
	if inline != "" {
		t.Errorf("inline: got %q, want %q", inline, "")
	}
}

func TestParseFormatString_TemplateWithEqualsInExpr(t *testing.T) {
	// Only the FIRST '=' is the separator; the rest are part of the expression.
	base, inline := parseFormatString("go-template={{if eq .x 1}}yes{{end}}")
	if base != "go-template" {
		t.Errorf("base: got %q, want %q", base, "go-template")
	}
	if inline != "{{if eq .x 1}}yes{{end}}" {
		t.Errorf("inline: got %q, want %q", inline, "{{if eq .x 1}}yes{{end}}")
	}
}

func TestParseFormatString_GoTemplateFile(t *testing.T) {
	base, inline := parseFormatString("go-template-file=./pod.tmpl")
	if base != "go-template-file" {
		t.Errorf("base: got %q, want %q", base, "go-template-file")
	}
	if inline != "./pod.tmpl" {
		t.Errorf("inline: got %q, want %q", inline, "./pod.tmpl")
	}
}

func TestApplyGoTemplate_FromFile(t *testing.T) {
	f, err := os.CreateTemp("", "cliford-tmpl-*.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("{{.name}}")
	f.Close()

	tmplBytes, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]any{"name": "Whiskers"}
	got, err := testApplyGoTemplate(data, string(tmplBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Whiskers" {
		t.Errorf("got %q, want %q", got, "Whiskers")
	}
}

func TestApplyJSONPath_InvalidExpression(t *testing.T) {
	data := map[string]any{"name": "Fluffy"}
	// A string that gojq cannot parse after our conversion
	_, err := testApplyJSONPath(data, "{invalid jq (((}")
	if err == nil {
		t.Fatal("expected error for invalid JSONPath expression, got nil")
	}
}
