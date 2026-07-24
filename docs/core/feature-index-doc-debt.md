# Feature-index doc debt — dead cross-references

Tracked follow-up from the 2026-07-24 feature-index hygiene sweep. These `code_paths`/`test_paths`
entries in feature `index.md` files point at files that no longer exist — mostly a removed
`pypi/` package and code renamed (not 1:1) during the `cmd/browser-agent` -> `internal/*` refactor.
The sweep auto-repaired 44 uniquely-resolvable relocations; these 52 need feature-owner
judgment (repoint or remove). They do not fail the docs gate (which checks bundle completeness,
not path validity), so they are recorded here rather than silently carried under a fresh review date.


## enhanced-cli-config (16)

- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/__init__.py`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/config.py`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/PKG-INFO`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/entry_points.txt`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/requires.txt`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/top_level.txt`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser.egg-info/SOURCES.txt`
- code_paths: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py`
- test_paths: `pypi/kaboom-agentic-browser/tests/test_branding.py`
- test_paths: `pypi/kaboom-agentic-browser/tests/test_config.py`
- test_paths: `pypi/kaboom-agentic-browser/tests/test_install.py`
- test_paths: `pypi/kaboom-agentic-browser/tests/test_uninstall.py`
- test_paths: `pypi/kaboom-agentic-browser/tests/test_skills.py`

## self-testing (7)

- test_paths: `scripts/tests/cat-17-generation-logic.sh`
- test_paths: `scripts/tests/cat-17-healing-logic.sh`
- test_paths: `scripts/tests/cat-18-recording-logic.sh`
- test_paths: `scripts/tests/cat-18-playback-logic.sh`
- test_paths: `scripts/tests/cat-19-extended.sh`
- test_paths: `scripts/tests/cat-20-security.sh`
- test_paths: `scripts/tests/cat-20-filtering-logic.sh`

## cursor-pagination (5)

- code_paths: `internal/pagination/pagination_actions.go`
- code_paths: `internal/pagination/pagination_websocket.go`
- code_paths: `internal/pagination/serialization.go`
- test_paths: `internal/pagination/pagination_actions_test.go`
- test_paths: `internal/pagination/pagination_websocket_test.go`

## observe (5)

- code_paths: `cmd/browser-agent/tools_observe_response.go`
- code_paths: `cmd/browser-agent/tools_observe_analysis.go`
- code_paths: `cmd/browser-agent/tools_observe_bundling.go`
- code_paths: `cmd/browser-agent/observe_filtering.go`
- code_paths: `internal/capture/queries.go`

## pagination (4)

- code_paths: `internal/pagination/pagination_actions.go`
- code_paths: `internal/pagination/pagination_websocket.go`
- test_paths: `internal/pagination/pagination_actions_test.go`
- test_paths: `internal/pagination/pagination_websocket_test.go`

## interact-explore (3)

- test_paths: `cmd/browser-agent/tools_interact_command_builder_test.go`
- test_paths: `cmd/browser-agent/tools_interact_upload_test.go`
- test_paths: `cmd/browser-agent/tools_interact_retry_contract_test.go`

## terminal (3)

- code_paths: `cmd/browser-agent/terminal_handlers.go`
- code_paths: `cmd/browser-agent/terminal_server.go`
- test_paths: `cmd/browser-agent/terminal_handlers_test.go`

## request-session-correlation (2)

- code_paths: `internal/session/verify_actions.go`
- test_paths: `internal/session/verify_test.go`

## analyze-tool (1)

- code_paths: `cmd/browser-agent/tools_security_audit.go`

## annotated-screenshots (1)

- code_paths: `cmd/browser-agent/tools_generate_annotations_visual.go`

## backend-log-streaming (1)

- code_paths: `internal/capture/queries.go`

## cold-start-queuing (1)

- code_paths: `cmd/browser-agent/tools_async_helpers.go`

## file-upload (1)

- test_paths: `cmd/browser-agent/tools_interact_upload_test.go`

## push-alerts (1)

- test_paths: `cmd/browser-agent/alerts_unit_test.go`

## query-dom (1)

- test_paths: `cmd/browser-agent/tools_analyze_route_test.go`
