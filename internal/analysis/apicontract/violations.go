// Purpose: Detects API contract violations from status/shape transitions.
// Why: Isolates violation semantics from report assembly and filtering.
// Docs: docs/features/feature/api-schema/index.md

package apicontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// detectErrorSpike checks if there's a pattern of success followed by errors.
func (v *APIContractValidator) detectErrorSpike(tracker *EndpointTracker, body capture.NetworkBody) *APIContractViolation {
	history := tracker.StatusHistory
	if len(history) < 3 {
		return nil
	}

	recentErrors := 0
	earlierSuccesses := 0

	for i := len(history) - 1; i >= 0; i-- {
		if history[i] >= 400 {
			recentErrors++
		} else {
			break
		}
	}

	for i := len(history) - 1 - recentErrors; i >= 0 && i >= len(history)-10; i-- {
		if history[i] >= 200 && history[i] < 300 {
			earlierSuccesses++
		}
	}

	if body.Status >= 500 && earlierSuccesses >= 2 && recentErrors >= 2 {
		var errorBody map[string]any
		_ = json.Unmarshal([]byte(body.ResponseBody), &errorBody)

		return &APIContractViolation{
			Endpoint:      tracker.Endpoint,
			Type:          "error_spike",
			Description:   fmt.Sprintf("Endpoint returned success %d times, then started returning errors", earlierSuccesses),
			StatusHistory: history,
			LastErrorBody: errorBody,
		}
	}

	return nil
}

// compareShapes compares expected vs actual shape and returns violations.
func (v *APIContractValidator) compareShapes(endpoint string, expected, actual, actualData any) []APIContractViolation {
	expectedMap, expectedOK := expected.(map[string]any)
	actualMap, actualOK := actual.(map[string]any)

	if !expectedOK || !actualOK {
		return detectTopLevelTypeChange(endpoint, expected, actual)
	}

	var violations []APIContractViolation
	violations = append(violations, detectMissingFields(endpoint, expectedMap, actualMap)...)
	violations = append(violations, detectNewFields(endpoint, expectedMap, actualMap)...)
	violations = append(violations, detectFieldTypeChanges(endpoint, expectedMap, actualMap, actualData)...)
	return violations
}

// detectTopLevelTypeChange reports a violation if the top-level response type changed.
func detectTopLevelTypeChange(endpoint string, expected, actual any) []APIContractViolation {
	if fmt.Sprintf("%T", expected) == fmt.Sprintf("%T", actual) {
		return nil
	}
	return []APIContractViolation{{
		Endpoint:     endpoint,
		Type:         "type_change",
		Description:  "Response type changed",
		ExpectedType: describeType(expected),
		ActualType:   describeType(actual),
	}}
}

// detectMissingFields returns a shape_change violation if fields from the expected shape are missing.
func detectMissingFields(endpoint string, expectedMap, actualMap map[string]any) []APIContractViolation {
	var missing []string
	for field := range expectedMap {
		if field == "$array" {
			continue
		}
		if _, found := actualMap[field]; !found {
			missing = append(missing, field)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return []APIContractViolation{{
		Endpoint:      endpoint,
		Type:          "shape_change",
		Description:   fmt.Sprintf("Field(s) missing from response: %s", strings.Join(missing, ", ")),
		MissingFields: missing,
		ExpectedShape: toStringMap(expectedMap),
		ActualShape:   toStringMap(actualMap),
	}}
}

// detectNewFields returns a new_field violation if unexpected fields appeared.
func detectNewFields(endpoint string, expectedMap, actualMap map[string]any) []APIContractViolation {
	var newFields []string
	for field := range actualMap {
		if field == "$array" {
			continue
		}
		if _, found := expectedMap[field]; !found {
			newFields = append(newFields, field)
		}
	}
	if len(newFields) == 0 {
		return nil
	}
	sort.Strings(newFields)
	return []APIContractViolation{{
		Endpoint:    endpoint,
		Type:        "new_field",
		Description: fmt.Sprintf("New field(s) appeared in response: %s", strings.Join(newFields, ", ")),
		NewFields:   newFields,
	}}
}

// detectFieldTypeChanges returns violations for fields whose types changed or became null.
func detectFieldTypeChanges(endpoint string, expectedMap, actualMap map[string]any, actualData any) []APIContractViolation {
	actualDataMap, _ := actualData.(map[string]any)
	var violations []APIContractViolation
	for field, expectedType := range expectedMap {
		actualType, found := actualMap[field]
		if !found {
			continue
		}
		expectedStr := describeType(expectedType)
		actualStr := describeType(actualType)

		if violation := classifyFieldTypeChange(endpoint, field, expectedStr, actualStr, actualDataMap); violation != nil {
			violations = append(violations, *violation)
		}
	}
	return violations
}

// classifyFieldTypeChange returns a violation if a field's type changed, or nil.
func classifyFieldTypeChange(endpoint, field, expectedType, actualType string, actualDataMap map[string]any) *APIContractViolation {
	if expectedType == actualType {
		return nil
	}
	if actualType == "null" && expectedType != "null" {
		return &APIContractViolation{
			Endpoint:     endpoint,
			Type:         "null_field",
			Description:  fmt.Sprintf("Field '%s' became null (was %s)", field, expectedType),
			Field:        field,
			ExpectedType: expectedType,
			ActualType:   "null",
		}
	}
	if actualType != "null" {
		var sampleValue any
		if actualDataMap != nil {
			sampleValue = actualDataMap[field]
		}
		return &APIContractViolation{
			Endpoint:     endpoint,
			Type:         "type_change",
			Description:  fmt.Sprintf("Field '%s' changed type from %s to %s", field, expectedType, actualType),
			Field:        field,
			ExpectedType: expectedType,
			ActualType:   actualType,
			SampleValue:  sampleValue,
		}
	}
	return nil
}

func (v *APIContractValidator) addViolation(tracker *EndpointTracker, violation APIContractViolation) {
	now := time.Now().Format(time.RFC3339)
	violation.ViolationType = violation.Type
	violation.Severity = violationSeverity(violation.Type)
	violation.AffectedCallCount = 1

	for i := range tracker.Violations {
		if tracker.Violations[i].Type == violation.Type && tracker.Violations[i].Endpoint == violation.Endpoint {
			tracker.Violations[i].AffectedCallCount++
			tracker.Violations[i].LastSeenAt = now
			return
		}
	}

	violation.FirstSeenAt = now
	violation.LastSeenAt = now

	if len(tracker.Violations) >= maxViolationsPerEndpoint {
		newViolations := make([]APIContractViolation, len(tracker.Violations)-1)
		copy(newViolations, tracker.Violations[1:])
		tracker.Violations = newViolations
	}
	tracker.Violations = append(tracker.Violations, violation)
}

// violationSeverity maps violation types to severity levels.
func violationSeverity(violationType string) string {
	switch violationType {
	case "error_spike":
		return "critical"
	case "shape_change":
		return "high"
	case "type_change":
		return "high"
	case "null_field":
		return "medium"
	case "new_field":
		return "low"
	default:
		return "medium"
	}
}
