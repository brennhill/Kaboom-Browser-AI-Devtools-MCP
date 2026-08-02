// properties_fixture.go — Defines the strict versioned QA fixture schema.

package configure

func fixtureProperties() map[string]any {
	return map[string]any{
		"fixture_action": map[string]any{
			"type":        "string",
			"description": "Validate, apply, inspect, or restore a QA fixture transaction. Failed applies are rolled back.",
			"enum":        []string{"validate", "apply", "status", "restore"},
		},
		"fixture":        qaFixtureSchema(),
		"transaction_id": map[string]any{"type": "string", "description": "Opaque transaction handle returned by fixture_action=apply."},
	}
}

func qaFixtureSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          "Versioned, declarative browser QA environment. Values remain local and are never echoed in responses or diagnostics.",
		"additionalProperties": false,
		"required":             []string{"version"},
		"properties": map[string]any{
			"version":          map[string]any{"type": "integer", "enum": []int{1}},
			"target":           closedObject(map[string]any{"url": map[string]any{"type": "string"}}),
			"viewport":         closedObject(map[string]any{"width": boundedInteger(320, 7680), "height": boundedInteger(240, 4320)}),
			"locale":           map[string]any{"type": "string"},
			"permissions":      stringArray([]string{"camera", "clipboard_read", "clipboard_write", "geolocation", "microphone", "notifications"}),
			"network":          closedObject(map[string]any{"profile": enumString([]string{"online", "offline", "slow_3g", "fast_3g"})}),
			"cookies":          map[string]any{"type": "array", "maxItems": 100, "items": cookieSchema()},
			"local_storage":    stringMapSchema(),
			"session_storage":  stringMapSchema(),
			"feature_flags":    map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "boolean"}, "maxProperties": 100},
			"seed_data":        map[string]any{"type": "object", "maxProperties": 100},
			"user_state":       enumString([]string{"fresh", "returning"}),
			"auth_role":        map[string]any{"type": "string", "maxLength": 64},
			"setup_timeout_ms": boundedInteger(100, 30000),
		},
	}
}

func cookieSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"name", "value"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1}, "value": map[string]any{"type": "string"},
			"domain": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"},
			"secure": map[string]any{"type": "boolean"}, "http_only": map[string]any{"type": "boolean"},
			"same_site": enumString([]string{"strict", "lax", "none"}),
		},
	}
}

func closedObject(properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
}

func boundedInteger(minimum, maximum int) map[string]any {
	return map[string]any{"type": "integer", "minimum": minimum, "maximum": maximum}
}

func enumString(values []string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func stringArray(values []string) map[string]any {
	return map[string]any{"type": "array", "uniqueItems": true, "items": enumString(values)}
}

func stringMapSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "maxProperties": 100}
}
