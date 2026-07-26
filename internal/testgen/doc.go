// Purpose: Package testgen — test generation from captured browser state and test failure classification.
// Why: Accelerates regression coverage by turning observed failures into repeatable tests.
// Docs: docs/features/feature/test-generation/index.md

/*
Package testgen provides test generation from captured browser state and test
failure classification.

Selector healing (the generate {type: "test_heal"} action) lives in the
self-contained subpackage internal/testgen/heal; it shares no code with this
package.

Key types:
  - GeneratedTest: output of test generation with framework, content, selectors, and coverage metadata.
  - FailureClassification: categorized test failure with confidence, evidence, and suggested fix.
  - DataProvider: interface abstracting access to captured logs, actions, and network bodies.

Key functions:
  - GenerateTestFromError: generates a test reproducing a specific console error.
  - GenerateTestFromInteraction: generates a test from recorded user interactions.
  - ClassifyFailure: categorizes a test failure (selector_broken, timing_flaky, real_bug, etc.).
*/
package testgen
