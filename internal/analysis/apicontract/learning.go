// Purpose: Learns endpoint response shape baselines from observed network bodies.
// Why: Establishes stable contract expectations before validation checks run.
// Docs: docs/features/feature/api-schema/index.md

package apicontract

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Learn records a network body observation for contract tracking.
func (v *APIContractValidator) Learn(body capture.NetworkBody) {
	v.mu.Lock()
	defer v.mu.Unlock()

	endpoint := normalizeEndpoint(body.Method, body.URL)

	if _, exists := v.trackers[endpoint]; !exists && len(v.trackers) >= maxContractEndpoints {
		return
	}

	tracker := v.getOrCreateTracker(endpoint)
	v.recordObservation(tracker, body.Status)

	if body.Status >= 200 && body.Status < 300 {
		tracker.SuccessCount++
		v.learnShape(tracker, body)
	}
}

func (v *APIContractValidator) getOrCreateTracker(endpoint string) *EndpointTracker {
	tracker, exists := v.trackers[endpoint]
	if !exists {
		tracker = &EndpointTracker{
			Endpoint:      endpoint,
			FieldPresence: make(map[string]int),
			FieldTypes:    make(map[string]string),
			StatusHistory: make([]int, 0),
			Violations:    make([]APIContractViolation, 0),
		}
		v.trackers[endpoint] = tracker
	}
	return tracker
}

func (v *APIContractValidator) learnShape(tracker *EndpointTracker, body capture.NetworkBody) {
	if body.ResponseBody == "" {
		return
	}
	if body.ContentType != "" && !strings.Contains(body.ContentType, "json") {
		return
	}

	var parsed any
	if err := json.Unmarshal([]byte(body.ResponseBody), &parsed); err != nil {
		return
	}

	shape := v.extractShape(parsed, 0)

	if tracker.EstablishedShape == nil {
		tracker.EstablishedShape = shape
		tracker.ConsistentCount++ // First response is by definition consistent.
	} else {
		shapeViolations := v.compareShapes(tracker.Endpoint, tracker.EstablishedShape, shape, parsed)
		if len(shapeViolations) == 0 {
			tracker.ConsistentCount++
		}
		tracker.EstablishedShape = v.mergeShapes(tracker.EstablishedShape, shape)
	}

	if objShape, ok := shape.(map[string]any); ok {
		for field, fieldType := range objShape {
			tracker.FieldPresence[field]++
			if typeStr, ok := fieldType.(string); ok {
				tracker.FieldTypes[field] = typeStr
			}
		}
	}
}

// extractShape extracts a type-only shape from a JSON value.
func (v *APIContractValidator) extractShape(value any, depth int) any {
	if depth > maxShapeComparisonDepth {
		return "object" // Truncate deep nesting.
	}

	switch val := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		if len(val) > 0 {
			elemShape := v.extractShape(val[0], depth+1)
			return map[string]any{"$array": elemShape}
		}
		return "array"
	case map[string]any:
		shape := make(map[string]any)
		for k, fieldVal := range val {
			shape[k] = v.extractShape(fieldVal, depth+1)
		}
		return shape
	default:
		return "unknown"
	}
}

// mergeShapes combines two shapes, keeping all observed fields.
func (v *APIContractValidator) mergeShapes(existing, incoming any) any {
	existingMap, existingOK := existing.(map[string]any)
	incomingMap, incomingOK := incoming.(map[string]any)

	if !existingOK || !incomingOK {
		// If types differ, prefer the existing established shape.
		return existing
	}

	merged := make(map[string]any)
	for k, val := range existingMap {
		merged[k] = val
	}
	for k, val := range incomingMap {
		if existingVal, exists := merged[k]; !exists {
			merged[k] = val
		} else {
			merged[k] = v.mergeShapes(existingVal, val)
		}
	}
	return merged
}

// recordObservation updates tracker counters and status history for a new observation.
func (v *APIContractValidator) recordObservation(tracker *EndpointTracker, status int) {
	tracker.CallCount++
	now := time.Now()
	if tracker.FirstCalled.IsZero() {
		tracker.FirstCalled = now
	}
	tracker.LastCalled = now

	tracker.StatusHistory = append(tracker.StatusHistory, status)
	if len(tracker.StatusHistory) > maxStatusHistory {
		newHistory := make([]int, len(tracker.StatusHistory)-1)
		copy(newHistory, tracker.StatusHistory[1:])
		tracker.StatusHistory = newHistory
	}
}
