// Purpose: Re-exports test generation aliases, constants, and helper bindings.
// Why: Keeps compatibility surfaces centralized so handler files stay focused on request flow.
// Docs: docs/features/feature/test-generation/index.md

package testgenhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testgen"
)

type TestFromContextRequest = testgen.TestFromContextRequest
type GeneratedTest = testgen.GeneratedTest
type TestHealRequest = testgen.TestHealRequest
type HealedSelector = testgen.HealedSelector
type HealResult = testgen.HealResult
type HealSummary = testgen.HealSummary
type BatchHealResult = testgen.BatchHealResult
type TestClassifyRequest = testgen.TestClassifyRequest
type TestFailure = testgen.TestFailure
type FailureClassification = testgen.FailureClassification
type SuggestedFix = testgen.SuggestedFix
type BatchClassifyResult = testgen.BatchClassifyResult

const (
	ErrNoErrorContext          = testgen.ErrNoErrorContext
	ErrNoActionsCaptured       = testgen.ErrNoActionsCaptured
	ErrTestFileNotFound        = testgen.ErrTestFileNotFound
	ErrClassificationUncertain = testgen.ErrClassificationUncertain
	ErrBatchTooLarge           = testgen.ErrBatchTooLarge
)

const maxFailuresPerBatch = testgen.MaxFailuresPerBatch

var (
	deriveInteractionTestName   = testgen.DeriveInteractionTestName
	buildRegressionAssertions   = testgen.BuildRegressionAssertions
	insertAssertionsBeforeClose = testgen.InsertAssertionsBeforeClose
	formatHealSummary           = testgen.FormatHealSummary
)
