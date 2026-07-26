// Purpose: Generates minimal OpenAPI YAML from inferred endpoint schema.
// Why: Separates export formatting concerns from schema accumulation logic.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ============================================
// OpenAPI Stub Generation
// ============================================

// BuildOpenAPIStub generates minimal OpenAPI 3.0 YAML from inferred schema.
func (s *SchemaStore) BuildOpenAPIStub(filter SchemaFilter) string {
	schema := s.BuildSchema(filter)

	var b strings.Builder
	b.WriteString("openapi: \"3.0.0\"\ninfo:\n  title: \"Inferred API\"\n  version: \"1.0.0\"\n  description: \"Auto-inferred from observed network traffic\"\npaths:\n")

	pathMethods := groupEndpointsByPath(schema.Endpoints)
	paths := util.SortedMapKeys(pathMethods)

	for _, path := range paths {
		b.WriteString("  " + path + ":\n")
		methods := pathMethods[path]
		sort.Slice(methods, func(i, j int) bool { return methods[i].Method < methods[j].Method })
		for i := range methods {
			writeEndpointYAML(&b, &methods[i])
		}
	}
	return b.String()
}

// groupEndpointsByPath groups endpoints by their path pattern.
func groupEndpointsByPath(endpoints []EndpointSchema) map[string][]EndpointSchema {
	pathMethods := make(map[string][]EndpointSchema)
	for i := range endpoints {
		ep := &endpoints[i]
		pathMethods[ep.PathPattern] = append(pathMethods[ep.PathPattern], *ep)
	}
	return pathMethods
}

// writeEndpointYAML writes a single endpoint's YAML to the builder.
func writeEndpointYAML(b *strings.Builder, ep *EndpointSchema) {
	method := strings.ToLower(ep.Method)
	b.WriteString("    " + method + ":\n")
	b.WriteString("      summary: \"" + ep.Method + " " + ep.PathPattern + "\"\n")
	b.WriteString("      responses:\n")
	writeResponseShapes(b, ep)
	writeRequestBody(b, ep)
	writeParameters(b, ep)
}

// writeResponseShapes writes response shape YAML for an endpoint.
func writeResponseShapes(b *strings.Builder, ep *EndpointSchema) {
	if len(ep.ResponseShapes) == 0 {
		b.WriteString("        \"200\":\n          description: \"OK\"\n")
		return
	}
	for status, shape := range ep.ResponseShapes {
		b.WriteString("        \"" + intToString(status) + "\":\n          description: \"Response\"\n")
		if len(shape.Fields) > 0 {
			b.WriteString("          content:\n            application/json:\n              schema:\n                type: object\n                properties:\n")
			for fieldName, fs := range shape.Fields {
				b.WriteString("                  " + fieldName + ":\n                    type: " + mapToOpenAPIType(fs.Type) + "\n")
			}
		}
	}
}

// writeRequestBody writes request body YAML if the endpoint has one.
func writeRequestBody(b *strings.Builder, ep *EndpointSchema) {
	if ep.RequestShape == nil || len(ep.RequestShape.Fields) == 0 {
		return
	}
	b.WriteString("      requestBody:\n        content:\n          application/json:\n            schema:\n              type: object\n              properties:\n")
	for fieldName, fs := range ep.RequestShape.Fields {
		b.WriteString("                " + fieldName + ":\n                  type: " + mapToOpenAPIType(fs.Type) + "\n")
	}
}

// writeParameters writes path and query parameter YAML.
func writeParameters(b *strings.Builder, ep *EndpointSchema) {
	if len(ep.PathParams) == 0 && len(ep.QueryParams) == 0 {
		return
	}
	b.WriteString("      parameters:\n")
	for _, pp := range ep.PathParams {
		b.WriteString("        - name: " + pp.Name + "\n          in: path\n          required: true\n          schema:\n            type: " + mapToOpenAPIType(pp.Type) + "\n")
	}
	for _, qp := range ep.QueryParams {
		b.WriteString("        - name: " + qp.Name + "\n          in: query\n")
		if qp.Required {
			b.WriteString("          required: true\n")
		}
		b.WriteString("          schema:\n            type: " + mapToOpenAPIType(qp.Type) + "\n")
	}
}

func mapToOpenAPIType(t string) string {
	switch t {
	case "integer":
		return "integer"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	case "uuid":
		return "string"
	default:
		return "string"
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if neg {
		result = "-" + result
	}
	return result
}
