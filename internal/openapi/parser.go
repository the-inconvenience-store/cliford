// Package openapi handles parsing OpenAPI specifications and building
// the Operation Registry that drives all code generation.
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/the-inconvenience-store/cliford/pkg/registry"
)

// Parser reads an OpenAPI specification and produces a Registry.
type Parser struct {
	specPath string
}

// NewParser creates a parser for the given spec file path.
func NewParser(specPath string) *Parser {
	return &Parser{specPath: specPath}
}

// Parse loads, validates, and converts the OpenAPI spec into a Registry.
func (p *Parser) Parse(ctx context.Context) (*registry.Registry, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(p.specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec %s: %w", p.specPath, err)
	}

	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("OpenAPI spec validation failed: %w", err)
	}

	reg := &registry.Registry{
		Title:           doc.Info.Title,
		Description:     doc.Info.Description,
		Version:         doc.Info.Version,
		Servers:         parseServers(doc.Servers),
		SecuritySchemes: parseSecuritySchemes(doc.Components),
		TagGroups:       make(map[string][]registry.OperationMeta),
	}

	operations, err := parseOperations(doc)
	if err != nil {
		return nil, err
	}

	if len(operations) == 0 {
		return nil, fmt.Errorf("OpenAPI spec contains no operations; at least one operation is required")
	}

	reg.Operations = operations

	for i := range reg.Operations {
		op := &reg.Operations[i]
		for _, tag := range op.Tags {
			reg.TagGroups[tag] = append(reg.TagGroups[tag], *op)
		}
	}

	return reg, nil
}

func parseServers(servers openapi3.Servers) []registry.ServerConfig {
	var result []registry.ServerConfig
	for _, s := range servers {
		sc := registry.ServerConfig{
			URL:         s.URL,
			Description: s.Description,
			Variables:   make(map[string]registry.ServerVariable),
		}
		for name, v := range s.Variables {
			sv := registry.ServerVariable{
				Default:     v.Default,
				Description: v.Description,
			}
			sv.Enum = append(sv.Enum, v.Enum...)
			sc.Variables[name] = sv
		}
		result = append(result, sc)
	}
	return result
}

func parseSecuritySchemes(components *openapi3.Components) map[string]registry.SecurityScheme {
	schemes := make(map[string]registry.SecurityScheme)
	if components == nil || components.SecuritySchemes == nil {
		return schemes
	}

	for name, ref := range components.SecuritySchemes {
		if ref.Value == nil {
			continue
		}
		ss := ref.Value
		scheme := registry.SecurityScheme{
			Name:         name,
			Type:         registry.SecuritySchemeType(ss.Type),
			Scheme:       ss.Scheme,
			BearerFormat: ss.BearerFormat,
		}

		if ss.Type == "apiKey" {
			scheme.In = registry.ParamLocation(ss.In)
		}

		if ss.Type == "openIdConnect" {
			scheme.OpenIDConnectURL = ss.OpenIdConnectUrl
		}

		if ss.Flows != nil {
			scheme.Flows = &registry.OAuthFlows{}
			if f := ss.Flows.AuthorizationCode; f != nil {
				scheme.Flows.AuthorizationCode = &registry.OAuthFlow{
					AuthorizationURL: f.AuthorizationURL,
					TokenURL:         f.TokenURL,
					RefreshURL:       f.RefreshURL,
					Scopes:           f.Scopes,
				}
			}
			if f := ss.Flows.ClientCredentials; f != nil {
				scheme.Flows.ClientCredentials = &registry.OAuthFlow{
					TokenURL: f.TokenURL,
					Scopes:   f.Scopes,
				}
			}
		}

		schemes[name] = scheme
	}

	return schemes
}

func parseOperations(doc *openapi3.T) ([]registry.OperationMeta, error) {
	var ops []registry.OperationMeta

	// Collect path+method pairs and sort for deterministic output
	type pathMethod struct {
		path   string
		method string
		op     *openapi3.Operation
		params openapi3.Parameters
	}
	var pairs []pathMethod

	for path, item := range doc.Paths.Map() {
		pathParams := item.Parameters
		for method, op := range map[string]*openapi3.Operation{
			http.MethodGet:    item.Get,
			http.MethodPost:   item.Post,
			http.MethodPut:    item.Put,
			http.MethodPatch:  item.Patch,
			http.MethodDelete: item.Delete,
		} {
			if op == nil {
				continue
			}
			allParams := make(openapi3.Parameters, 0, len(pathParams)+len(op.Parameters))
			allParams = append(allParams, pathParams...)
			allParams = append(allParams, op.Parameters...)
			pairs = append(pairs, pathMethod{path: path, method: method, op: op, params: allParams})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].path != pairs[j].path {
			return pairs[i].path < pairs[j].path
		}
		return pairs[i].method < pairs[j].method
	})

	for _, pm := range pairs {
		op := pm.op
		meta := registry.OperationMeta{
			OperationID: op.OperationID,
			Method:      pm.method,
			Path:        pm.path,
			Summary:     op.Summary,
			Description: op.Description,
			Tags:        op.Tags,
			Deprecated:  op.Deprecated,
			Security:    parseOperationSecurity(op, doc),
		}

		if meta.OperationID == "" {
			meta.OperationID = deriveOperationID(pm.method, pm.path)
		}

		meta.SDKMethodName = toExportedName(meta.OperationID)

		for _, pRef := range pm.params {
			if pRef.Value == nil {
				continue
			}
			meta.Parameters = append(meta.Parameters, parseParameter(pRef.Value))
		}

		if op.RequestBody != nil && op.RequestBody.Value != nil {
			meta.RequestBody = parseRequestBody(op.RequestBody.Value)
		}

		meta.Responses = parseResponses(op.Responses)
		meta.TUIDisplay = inferDisplayMode(meta)

		// Extract x-cliford-* extensions (overrides inferred values)
		ExtractExtensions(pm.op, &meta)

		ops = append(ops, meta)
	}

	return ops, nil
}

func parseParameter(p *openapi3.Parameter) registry.ParamMeta {
	pm := registry.ParamMeta{
		Name:        p.Name,
		In:          registry.ParamLocation(p.In),
		Description: p.Description,
		Required:    p.Required,
		FlagName:    toKebabCase(p.Name),
	}

	if p.Schema != nil && p.Schema.Value != nil {
		pm.Schema = convertSchema(p.Schema.Value)
		if p.Schema.Value.Default != nil {
			pm.Default = p.Schema.Value.Default
		}
		for _, e := range p.Schema.Value.Enum {
			pm.Enum = append(pm.Enum, e)
		}
	}

	// Extract example: parameter-level first, then named examples, then schema-level.
	switch {
	case p.Example != nil:
		pm.Example = exampleToString(p.Example)
	case len(p.Examples) > 0:
		keys := make([]string, 0, len(p.Examples))
		for k := range p.Examples {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if ref := p.Examples[keys[0]]; ref != nil && ref.Value != nil {
			pm.Example = exampleToString(ref.Value.Value)
		}
	case p.Schema != nil && p.Schema.Value != nil && p.Schema.Value.Example != nil:
		pm.Example = exampleToString(p.Schema.Value.Example)
	}

	return pm
}

func parseRequestBody(rb *openapi3.RequestBody) *registry.RequestBodyMeta {
	// Sort content-type keys for deterministic selection.
	contentTypes := make([]string, 0, len(rb.Content))
	for ct := range rb.Content {
		contentTypes = append(contentTypes, ct)
	}
	sort.Strings(contentTypes)

	for _, contentType := range contentTypes {
		mediaType := rb.Content[contentType]
		if mediaType == nil || mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}
		meta := &registry.RequestBodyMeta{
			ContentType: contentType,
			Schema:      convertSchema(mediaType.Schema.Value),
			Required:    rb.Required,
			Description: rb.Description,
		}
		// Extract example: media-type level first, then named examples.
		switch {
		case mediaType.Example != nil:
			meta.Example = exampleToString(mediaType.Example)
		case len(mediaType.Examples) > 0:
			keys := make([]string, 0, len(mediaType.Examples))
			for k := range mediaType.Examples {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if ref := mediaType.Examples[keys[0]]; ref != nil && ref.Value != nil {
				meta.Example = exampleToString(ref.Value.Value)
			}
		}
		return meta
	}
	return nil
}

func parseResponses(responses *openapi3.Responses) map[int]registry.ResponseMeta {
	result := make(map[int]registry.ResponseMeta)
	if responses == nil {
		return result
	}

	statusCodes := map[string]int{
		"200": 200, "201": 201, "204": 204,
		"400": 400, "401": 401, "403": 403, "404": 404, "422": 422,
		"500": 500, "502": 502, "503": 503,
	}

	for code, ref := range responses.Map() {
		if ref.Value == nil {
			continue
		}
		sc, ok := statusCodes[code]
		if !ok {
			continue
		}

		rm := registry.ResponseMeta{
			StatusCode:  sc,
			Description: *ref.Value.Description,
		}

		for ct, mt := range ref.Value.Content {
			rm.ContentType = ct
			if mt.Schema != nil && mt.Schema.Value != nil {
				s := convertSchema(mt.Schema.Value)
				rm.Schema = &s
			}
			break // Take the first content type
		}

		result[sc] = rm
	}

	return result
}

func parseOperationSecurity(op *openapi3.Operation, doc *openapi3.T) []registry.SecurityRequirement {
	secReqs := op.Security
	if secReqs == nil {
		secReqs = &doc.Security
	}
	if secReqs == nil {
		return nil
	}

	var result []registry.SecurityRequirement
	for _, req := range *secReqs {
		for name, scopes := range req {
			result = append(result, registry.SecurityRequirement{
				SchemeName: name,
				Scopes:     scopes,
			})
		}
	}
	return result
}

func convertSchema(s *openapi3.Schema) registry.SchemaMeta {
	typeStr := ""
	if ts := s.Type.Slice(); len(ts) > 0 {
		typeStr = ts[0]
	}

	sm := registry.SchemaMeta{
		Type:        typeStr,
		Format:      s.Format,
		Nullable:    s.Nullable,
		Required:    s.Required,
		Description: s.Description,
	}

	// Check for x-cliford-display extension
	if ext, ok := s.Extensions["x-cliford-display"]; ok {
		if v, ok := ext.(bool); ok && v {
			sm.Display = true
		}
	}

	if len(s.Enum) > 0 {
		sm.Enum = make([]any, len(s.Enum))
		copy(sm.Enum, s.Enum)
	}

	if s.Items != nil && s.Items.Value != nil {
		items := convertSchema(s.Items.Value)
		sm.Items = &items
	}

	if len(s.Properties) > 0 {
		sm.Properties = make(map[string]registry.SchemaMeta)
		for name, prop := range s.Properties {
			if prop.Value != nil {
				sm.Properties[name] = convertSchema(prop.Value)
			}
		}
	}

	return sm
}

func inferDisplayMode(op registry.OperationMeta) registry.DisplayMode {
	switch {
	case op.Method == http.MethodGet && op.RequestBody == nil:
		resp, ok := op.Responses[200]
		if ok && resp.Schema != nil && resp.Schema.Type == "array" {
			return registry.DisplayTable
		}
		return registry.DisplayDetail
	case op.Method == http.MethodDelete:
		return registry.DisplayDetail
	default:
		return registry.DisplayForm
	}
}

// deriveOperationID creates an operation ID from method + path when not specified.
func deriveOperationID(method, path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var filtered []string
	for _, p := range parts {
		if strings.HasPrefix(p, "{") {
			continue
		}
		filtered = append(filtered, p)
	}
	return strings.ToLower(method) + toExportedName(strings.Join(filtered, "-"))
}

// toKebabCase converts camelCase or PascalCase to kebab-case.
func toKebabCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r + 32) // toLower
		} else if r == '_' || r == ' ' {
			result.WriteByte('-')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// exampleToString converts an OpenAPI example value (any) to a printable string.
// String values pass through directly; all other types are JSON-marshalled.
// Returns an empty string if v is nil or marshalling fails.
func exampleToString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// toExportedName converts an operation ID to an exported Go function name.
func toExportedName(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	if result.Len() == 0 {
		return s
	}
	return result.String()
}
