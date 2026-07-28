// Purpose: Generates Playwright test scripts from captured errors, interactions, or regression baselines.
// Docs: docs/features/feature/test-generation/index.md

package testgen

import (
	"errors"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reproduction"
)

// DataProvider abstracts access to captured browser data for test generation.
type DataProvider interface {
	GetLogEntries() []map[string]any
	GetAllEnhancedActions() []types.EnhancedAction
	GetNetworkBodies() []types.NetworkBody
}

// ContextDispatch maps context values to their generator functions.
var ContextDispatch = map[string]func(dp DataProvider, params TestFromContextRequest) (*GeneratedTest, error){
	"error":       GenerateTestFromError,
	"interaction": GenerateTestFromInteraction,
	"regression":  GenerateTestFromRegression,
}

// TestGenErrorMapping maps error code substrings to structured error details.
type TestGenErrorMapping struct {
	Code    string
	Message string
	Retry   string
	Hint    string
}

// ErrorMappings provides error details for test generation failures.
var ErrorMappings = []TestGenErrorMapping{
	{
		Code:    ErrNoErrorContext,
		Message: "No console errors captured to generate test from",
		Retry:   "Trigger an error in the browser first, then retry",
		Hint:    "Use the observe tool to verify errors are being captured",
	},
	{
		Code:    ErrNoActionsCaptured,
		Message: "No user actions recorded in the session",
		Retry:   "Interact with the page first (click, type, navigate), then retry",
		Hint:    "Use the observe tool with what=actions to verify actions are being captured",
	},
	{
		Code:    ErrNoBaseline,
		Message: "No regression baseline available",
		Retry:   "Capture a baseline first by interacting with the page, then retry",
		Hint:    "The regression mode generates tests by comparing current state against a baseline",
	},
}

// GenerateTestFromError generates a test that reproduces a specific console error.
func GenerateTestFromError(dp DataProvider, req TestFromContextRequest) (*GeneratedTest, error) {
	targetError, errorID, errorTimestamp := FindTargetError(dp, req.ErrorID)
	if targetError == nil {
		return nil, errors.New(ErrNoErrorContext)
	}

	errorMessage, _ := targetError["message"].(string)

	relevantActions, err := GetActionsInTimeWindow(dp, errorTimestamp, 5000)
	if err != nil {
		return nil, err
	}

	script := reproduction.GeneratePlaywrightScript(relevantActions, reproduction.Params{
		ErrorMessage: errorMessage,
		BaseURL:      req.BaseURL,
	})
	assertionCount := strings.Count(script, "expect(") + 1
	filenameBase := errorMessage
	if req.TestName != "" {
		filenameBase = req.TestName
	}
	filename := GenerateTestFilename(filenameBase, req.Framework)
	selectors := ExtractSelectorsFromActions(relevantActions)

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: true,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(relevantActions) > 0,
		},
		Metadata: TestGenMetadata{
			SourceError: errorID,
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: []string{"console", "actions"},
		},
	}, nil
}

// GenerateTestFromInteraction generates a test from recorded user interactions.
func GenerateTestFromInteraction(dp DataProvider, req TestFromContextRequest) (*GeneratedTest, error) {
	allActions := dp.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	script := reproduction.GeneratePlaywrightScript(allActions, reproduction.Params{
		BaseURL: req.BaseURL,
	})
	assertionCount := strings.Count(script, "expect(")

	if req.IncludeMocks {
		assertionCount += CountNetworkAssertions(dp)
	}

	filenameBase := DeriveInteractionTestName(allActions)
	if req.TestName != "" {
		filenameBase = req.TestName
	}
	filename := GenerateTestFilename(filenameBase, req.Framework)
	selectors := ExtractSelectorsFromActions(allActions)

	contextUsed := []string{"actions"}
	if req.IncludeMocks {
		contextUsed = append(contextUsed, "network")
	}

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   filename,
		Content:    script,
		Selectors:  selectors,
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: false,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(allActions) > 0,
		},
		Metadata: TestGenMetadata{
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: contextUsed,
		},
	}, nil
}

// GenerateTestFromRegression generates a test that guards against regressions.
func GenerateTestFromRegression(dp DataProvider, req TestFromContextRequest) (*GeneratedTest, error) {
	allActions := dp.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	errorMessages := CollectErrorMessages(dp, 5)
	networkBodies := dp.GetNetworkBodies()

	assertions, assertionCount := BuildRegressionAssertions(errorMessages, networkBodies)

	script := reproduction.GeneratePlaywrightScript(allActions, reproduction.Params{
		BaseURL: req.BaseURL,
	})
	script = InsertAssertionsBeforeClose(script, assertions)

	return &GeneratedTest{
		Framework:  req.Framework,
		Filename:   GenerateTestFilename("regression-test", req.Framework),
		Content:    script,
		Selectors:  ExtractSelectorsFromActions(allActions),
		Assertions: assertionCount,
		Coverage: TestCoverage{
			ErrorReproduced: false,
			NetworkMocked:   req.IncludeMocks,
			StateCaptured:   len(allActions) > 0,
		},
		Metadata: TestGenMetadata{
			GeneratedAt: time.Now().Format(time.RFC3339),
			ContextUsed: []string{"actions", "console", "network", "performance"},
		},
	}, nil
}

// FindTargetError searches log entries for a matching error, returning the entry,
// its error_id, and its timestamp in milliseconds.
func FindTargetError(dp DataProvider, errorID string) (map[string]any, string, int64) {
	entries := dp.GetLogEntries()

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		if errorID != "" {
			entryID, _ := entry["error_id"].(string)
			if entryID != errorID {
				continue
			}
			tsStr, _ := entry["ts"].(string)
			return entry, entryID, NormalizeTimestamp(tsStr)
		}

		tsStr, _ := entry["ts"].(string)
		id, _ := entry["error_id"].(string)
		return entry, id, NormalizeTimestamp(tsStr)
	}

	return nil, "", 0
}

// GetActionsInTimeWindow returns actions within a time window around a center timestamp.
func GetActionsInTimeWindow(dp DataProvider, centerTimestamp int64, windowMs int64) ([]types.EnhancedAction, error) {
	allActions := dp.GetAllEnhancedActions()
	if len(allActions) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	var relevant []types.EnhancedAction
	for i := range allActions {
		action := &allActions[i]
		timeDiff := action.Timestamp - centerTimestamp
		if timeDiff >= -windowMs && timeDiff <= windowMs {
			relevant = append(relevant, *action)
		}
	}

	if len(relevant) == 0 {
		return nil, errors.New(ErrNoActionsCaptured)
	}

	return relevant, nil
}

// CountNetworkAssertions counts network bodies with status > 0.
func CountNetworkAssertions(dp DataProvider) int {
	networkBodies := dp.GetNetworkBodies()
	count := 0
	for _, nb := range networkBodies {
		if nb.Status > 0 {
			count++
		}
	}
	return count
}

// CollectErrorMessages extracts error-level log messages up to a limit.
func CollectErrorMessages(dp DataProvider, limit int) []string {
	entries := dp.GetLogEntries()

	var messages []string
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg != "" && len(messages) < limit {
			messages = append(messages, msg)
		}
	}
	return messages
}
