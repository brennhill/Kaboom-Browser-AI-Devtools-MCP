// Purpose: Filters and sorts log entries by level, source, URL, and time range for observe responses.
// Docs: docs/features/feature/observe/index.md

package core

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strconv"
	"strings"
)

// LogLevelRank returns the severity rank of a log level (higher = more severe).
func LogLevelRank(level string) int {
	switch level {
	case "debug":
		return 0
	case "log":
		return 1
	case "info":
		return 2
	case "warn":
		return 3
	case "error":
		return 4
	default:
		return -1
	}
}

// ContainsIgnoreCase reports whether s contains substr (case-insensitive).
func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ============================================
// Network body filtering
// ============================================

type jsonPathToken struct {
	key     string
	index   int
	isIndex bool
}

// ApplyNetworkBodyFilter filters a network body by JSON path extraction.
func ApplyNetworkBodyFilter(body types.NetworkBody, bodyPath string) (types.NetworkBody, bool, error) {
	if bodyPath == "" {
		return body, true, nil
	}
	if body.ResponseBody == "" {
		return body, false, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(body.ResponseBody), &decoded); err != nil {
		return body, false, nil
	}

	value, ok, err := extractJSONPath(decoded, bodyPath)
	if err != nil {
		return body, false, err
	}
	if !ok {
		return body, false, nil
	}
	return encodeFilteredNetworkBody(body, value)
}

func encodeFilteredNetworkBody(body types.NetworkBody, value any) (types.NetworkBody, bool, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return body, false, fmt.Errorf("failed to encode filtered value: %w", err)
	}
	filtered := body
	filtered.ResponseBody = string(raw)
	filtered.ResponseTruncated = false
	filtered.BinaryFormat = ""
	filtered.FormatConfidence = 0
	return filtered, true, nil
}

func extractJSONPath(root any, path string) (any, bool, error) {
	tokens, err := parseJSONPath(path)
	if err != nil {
		return nil, false, err
	}

	current := root
	for _, token := range tokens {
		if token.isIndex {
			items, ok := current.([]any)
			if !ok || token.index < 0 || token.index >= len(items) {
				return nil, false, nil
			}
			current = items[token.index]
			continue
		}

		object, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		next, ok := object[token.key]
		if !ok {
			return nil, false, nil
		}
		current = next
	}

	return current, true, nil
}

func parseJSONPath(path string) ([]jsonPathToken, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("body_path_filter: path argument cannot be empty. Provide a dot-delimited path like 'data.items'")
	}

	trimmed = trimJSONPathRoot(trimmed)
	if trimmed == "" {
		return []jsonPathToken{}, nil
	}

	tokens := make([]jsonPathToken, 0, 6)
	for i := 0; i < len(trimmed); {
		var err error
		switch trimmed[i] {
		case '.':
			i++
			if i >= len(trimmed) || trimmed[i] == '.' || trimmed[i] == '[' {
				return nil, fmt.Errorf("invalid empty segment after '.'")
			}
		case '[':
			var token jsonPathToken
			token, i, err = parseIndexSegment(trimmed, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
		default:
			var token jsonPathToken
			token, i, err = parseKeySegment(trimmed, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
		}
	}

	return tokens, nil
}

func trimJSONPathRoot(trimmed string) string {
	if strings.HasPrefix(trimmed, "$.") {
		return trimmed[2:]
	}
	if strings.HasPrefix(trimmed, "$") {
		return trimmed[1:]
	}
	return trimmed
}

func parseIndexSegment(trimmed string, start int) (jsonPathToken, int, error) {
	endOffset := strings.IndexByte(trimmed[start:], ']')
	if endOffset < 0 {
		return jsonPathToken{}, start, fmt.Errorf("missing closing ']' in path")
	}
	end := start + endOffset
	inner := strings.TrimSpace(trimmed[start+1 : end])
	if inner == "" {
		return jsonPathToken{}, end + 1, fmt.Errorf("empty [] segment in path")
	}

	if isQuotedSegment(inner) {
		key := inner[1 : len(inner)-1]
		if key == "" {
			return jsonPathToken{}, end + 1, fmt.Errorf("empty key segment in path")
		}
		return jsonPathToken{key: key}, end + 1, nil
	}

	index, err := strconv.Atoi(inner)
	if err != nil {
		return jsonPathToken{}, end + 1, fmt.Errorf("invalid array index %q: %w", inner, err)
	}
	if index < 0 {
		return jsonPathToken{}, end + 1, fmt.Errorf("invalid array index %q", inner)
	}
	return jsonPathToken{index: index, isIndex: true}, end + 1, nil
}

func isQuotedSegment(inner string) bool {
	return (inner[0] == '\'' && inner[len(inner)-1] == '\'') || (inner[0] == '"' && inner[len(inner)-1] == '"')
}

func parseKeySegment(trimmed string, start int) (jsonPathToken, int, error) {
	i := start
	for i < len(trimmed) && trimmed[i] != '.' && trimmed[i] != '[' {
		i++
	}
	key := strings.TrimSpace(trimmed[start:i])
	if key == "" {
		return jsonPathToken{}, i, fmt.Errorf("invalid key segment in path")
	}
	return jsonPathToken{key: key}, i, nil
}
