// Purpose: Package interact — implementation helpers for the interact MCP tool's selectors, state, and workflows.
// Why: Centralizes selector/workflow logic so browser actions remain repeatable and debuggable.
// Docs: docs/features/feature/interact-explore/index.md

/*
Package interact provides the implementation for the interact MCP tool, which
performs browser actions like click, type, navigate, and state management.

Key types:
  - WorkflowStep: records a single step's outcome within a workflow trace.

Key functions:
  - ParseSelectorForReproduction: converts semantic selectors (text=, role=) into structured maps.
  - ValidateStateSaveParams: validates save_state parameters.
  - ValidateStateLoadParams: validates load_state parameters.
  - BuildWorkflowTraceEnvelope: creates normalized stage-level trace metadata.
  - WorkflowResult: wraps workflow responses with summary and trace metadata.
*/
package interact
