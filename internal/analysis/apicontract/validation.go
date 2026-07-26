// Purpose: Validates observed responses against learned API contract baselines.
// Why: Detects shape/type regressions and error spikes after baseline establishment.
// Docs: docs/features/feature/api-schema/index.md

package apicontract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Validate checks a network body against the learned schema and returns violations.
func (v *APIContractValidator) Validate(body capture.NetworkBody) []APIContractViolation {
	v.mu.Lock()
	defer v.mu.Unlock()

	endpoint := normalizeEndpoint(body.Method, body.URL)
	tracker := v.getOrCreateTracker(endpoint)
	v.recordObservation(tracker, body.Status)

	if body.Status >= 400 {
		return v.validateErrorResponse(tracker, body)
	}

	if tracker.SuccessCount < minCallsToEstablishShape {
		tracker.SuccessCount++
		v.learnShape(tracker, body)
		return nil
	}

	return v.validateShapeConsistency(tracker, endpoint, body)
}

// validateErrorResponse checks for error spikes in error responses.
func (v *APIContractValidator) validateErrorResponse(tracker *EndpointTracker, body capture.NetworkBody) []APIContractViolation {
	spike := v.detectErrorSpike(tracker, body)
	if spike == nil {
		return nil
	}
	v.addViolation(tracker, *spike)
	return []APIContractViolation{*spike}
}

// validateShapeConsistency compares a successful response shape against the established schema.
func (v *APIContractValidator) validateShapeConsistency(tracker *EndpointTracker, endpoint string, body capture.NetworkBody) []APIContractViolation {
	if body.ResponseBody == "" {
		return nil
	}
	if body.ContentType != "" && !strings.Contains(body.ContentType, "json") {
		return nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(body.ResponseBody), &parsed); err != nil {
		return nil
	}

	actualShape := v.extractShape(parsed, 0)
	shapeViolations := v.compareShapes(endpoint, tracker.EstablishedShape, actualShape, parsed)
	for _, violation := range shapeViolations {
		v.addViolation(tracker, violation)
	}
	if len(shapeViolations) == 0 {
		tracker.ConsistentCount++
	}
	return shapeViolations
}

func describeType(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v // Already a type string.
	case map[string]any:
		if _, hasArray := v["$array"]; hasArray {
			return "array"
		}
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func toStringMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = val
		case map[string]any:
			result[k] = toStringMap(val)
		default:
			result[k] = describeType(v)
		}
	}
	return result
}
