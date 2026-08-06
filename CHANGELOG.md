# Changelog

All notable changes to Dev Console will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [6.0.0] - 2026-02-09

### Added

#### Features

- **Link Health Analyzer** — New `analyze({what: 'link_health'})` tool to check all links on current page for issues
  - Concurrent checking (20 workers) for performance
  - Categorizes links: ok (2xx), redirect (3xx), requires_auth (401/403), broken (4xx/5xx), timeout, **cors_blocked**
  - Browser-based validation using extension security context
  - **CORS detection** — Explicitly flags CORS-blocked links (status 0) instead of false negatives
  - Async operation with correlation ID tracking
  - 19 comprehensive UAT tests covering edge cases and concurrency

- **Link Validation Tool** — New `analyze({what: 'link_validation'})` tool for server-side verification
  - Validates CORS-blocked links that browser extension cannot check
  - Concurrent HTTP HEAD/GET requests (worker pool, max 100)
  - Categorizes results: ok (2xx), redirect (3xx), requires_auth (401/403), broken (4xx/5xx)
  - No external dependencies (native Go http package)
  - Fallback for links flagged as `cors_blocked` by browser analyzer

#### Architecture

- **New analyze() tool** — Unified interface for active analysis operations
  - Migrated `dom`, `api_validation` from configure()
  - Migrated `performance`, `accessibility`, `error_clusters`, `history`, `security_audit`, `third_party_audit`, `security_diff` from observe()
  - Clean semantic separation: analyze (active work) vs observe (passive reading)

### Changed

- **Tool reorganization** — Clear taxonomy with 5 MCP tools:
  - `observe()` — Passive data reading from buffers
  - `analyze()` — Active analysis and inspection (NEW)
  - `configure()` — Settings and session control
  - `interact()` — Browser manipulation
  - `generate()` — Artifact creation

### Fixed

- **MCP compliance** — isError field now always present in tool responses (spec requirement)
- **Extension version format** — Updated to valid semver (6.0.0)

## [5.3.0] - 2026-01-31

### Added

#### Features & Infrastructure
- **Flow Recording storage infrastructure** — Modular storage system for recording playback (Module 2, Phase 1)
- **Playback engine tests** — Comprehensive test suite for playback functionality (Module 3, Phase 1)
- **Extended test coverage** — Converted 40+ placeholder/skip tests to assertions across modules:
  - WebSocket streaming module tests (Module 1)
  - Log diffing module tests (Module 4)
  - Extension integration module tests (Module 5)
  - Recording storage quota tests

#### Documentation & Process
- **Complete filename standardization** — All 335 documentation files standardized to `lowercase-with-hyphens` format (product-spec.md, tech-spec.md, qa-plan.md across all 71 features)
- **Documentation automation scripts** — Feature navigation generator (`generate-feature-navigation.py`) and lint checker (`lint-documentation.py`) with link verification and frontmatter validation
- **Mandatory documentation workflow** — `documentation-maintenance.md` with quality gates, commit checklists, and enforcement rules
- **Comprehensive documentation guides** — `docs/features/README.md` (LLM-optimized feature guide), `docs/cross-reference.md` (dependency mapping), `docs/core/architecture/codebase-canon-v5.3.md` (baseline reference)
- **YAML frontmatter metadata** — All feature docs include status, scope, ai-priority, tags, relates-to, and last-verified fields for AI discoverability
- **On-demand context loading strategy** — `context-on-demand.md` with task-based documentation loading to minimize startup overhead
- **Optimized startup context** — Reduced initial documentation load to ~5K tokens

### Changed

#### Development Workflow
- **Process enforcement** — Documentation updates now mandatory before commit; lint checker blocks broken links and stale metadata
- **Feature documentation structure** — Unified naming and metadata across all 71 features with status tracking
- **Navigation automation** — Feature index auto-generated from folder structure and frontmatter
- **Reference authority** — v5.3 established as codebase baseline with recovery guide for version verification

#### Testing
- **Test execution converted from skips to assertions** — 40+ tests now actively validate behavior instead of being skipped or marked as placeholders
- **Quota enforcement** — Recording storage quota tests verify disk/memory constraints

### Technical Details

**Code Changes:**
- Flow recording storage infrastructure with multi-format support
- Playback engine with state machine validation
- 40+ test assertions added/converted across all modules

**Documentation Changes:**
- 372 files updated for reference consistency after 335 file renames
- Zero breaking changes — All documentation functionality preserved during standardization
- Quality gates enforced — Pre-commit lint checking prevents documentation rot
- AI-optimized navigation — Multiple discovery paths for LLM agents (quick-reference, feature-navigation, cross-reference, context-on-demand)

**Documentation Cleanup:**
- Removed 2 broken stub files (`product-spec.md`, `tech-spec.md` in core/)
- Consolidated duplicate reviews (SARIF, behavioral-baselines, interception-deferral)
- Archived 11 stale tab-tracking documents with recovery guide
- Added comprehensive metadata to 130+ feature docs

---

## [5.2.5] - 2026-01-30

### Fixed
- **Accessibility audit runtime error** (HIGH SEVERITY): Fixed `observe({what: "accessibility"})` failing with "chrome.runtime.getURL is not a function" by pre-injecting axe-core from content script context
- **Parameter validation warnings** (HIGH SEVERITY): Removed false "unknown parameter" warnings for all documented sub-handler parameters (limit, selector, test_name, etc.)

### Changed
- Removed routing-level parameter validation that incorrectly flagged documented parameters
- Updated `loadAxeCore()` to wait for pre-injected axe-core instead of attempting injection from page context

### Technical Details
- Extension: Pre-inject axe-core from content script (has chrome.runtime API access)
- Server: Remove `unmarshalWithWarnings()` from routing functions (toolObserve, toolConfigure, toolGenerate, toolInteract)
- Tests: Removed 2 tests that validated broken routing-level parameter warnings
- See bug-fixes-summary.md for complete technical details

## [5.1.0] - 2026-01-28

### Security

- **Single-tab tracking isolation** — Extension now only captures telemetry from the explicitly tracked tab. Previously, data from ALL browser tabs was captured regardless of tracking state. This was a critical privacy vulnerability.

### Added

- **"Track This Tab" button** — Replaces "Track This Page". Attaches to a single browser tab for telemetry capture. One-click enable/disable.
- **"No Tracking" mode** — When no tab is tracked, LLM receives clear warning with actionable instructions.
- **Status ping endpoint** (`/api/extension-status`) — Extension pings server every 30s with tracking state.
- **Chrome internal page blocking** — Cannot track `chrome://`, `about:`, or other internal pages.
- **Browser restart handling** — Tracking state cleared automatically on browser restart.
- **Tab ID attachment** — All forwarded messages include `tabId` for data attribution.
- **40 new tab-filtering tests** — Comprehensive test coverage for isolation, filtering, and edge cases.

### Changed

- **validate_api parameter** — Renamed from conflicting `action`/`api_action` to `operation` for LLM discoverability.
- **Network schema improvements** — Unit suffixes (`durationMs`, `transferSizeBytes`), `compressionRatio`, `capturedAt` timestamps, helpful `limitations` array.

### Infrastructure
- **PyPI distribution** — `pip install kaboom-agentic-browser` now available alongside NPM
- **Repository renamed** to `Kaboom-Browser-AI-Devtools-MCP` for SEO discoverability
- **Documentation overhaul** — Consolidated tests into `tests/`, assets into `docs/assets/`, ADRs into `docs/adrs/`, added templates, feature index, and master doc navigation

### Known Issues

See [KNOWN-ISSUES.md](known-issues.md) for issues targeted for v5.2.

## [5.0.0] - 2026-01-26

### Added
- **4-tool MCP architecture** — `observe`, `generate`, `configure`, `interact` (replaced discrete tools)
- **AI Web Pilot** — Browser automation via MCP (navigate, click, type, execute JS, save/load state)
- **Async command execution** — Background commands between MCP server and browser extension
- **Enterprise audit trail** — Tamper-evident logging with SARIF export
- **API schema inference** — Auto-discover OpenAPI schemas from captured network traffic
- **Web Vitals monitoring** — LCP, CLS, INP, FCP with automatic regression detection
- **Error clustering** — Automatic grouping with noise filtering
- **Session checkpoints** — Named save points with diff support
- **HAR export** — Standard HTTP Archive from captured network data
- **CSP and SRI generation** — Security policy generation from audit data
- **Temporal graph** — Cross-session history tracking
- **Rate limiting and memory enforcement** — Resource management with configurable TTLs
- **Binary format detection** — Smart handling of non-text network bodies
- **NPM distribution** — `npx kaboom-agentic-browser` with platform-specific binaries

### Changed
- Migrated from multiple discrete tools to 4 composite tools
- Restructured MCP tool descriptions for LLM optimization
- Go server rewrite (zero dependencies, single binary)

## [3.0.0] - 2026-01-23

### Added

- **Initial release** — Kaboom v3.0.0 foundation
- **Browser Extension (MV3)**
  - Console capture (log, warn, error, info, debug)
  - Network error capture (4xx, 5xx responses)
  - Exception capture (onerror, unhandled rejections)
  - Configurable capture levels
  - Domain filtering
  - Connection status badge
- **Go Server**
  - Zero-dependency server
  - Health check endpoint
  - CORS support for browser extension
- **Landing Page**
  - Quick start instructions
  - Feature overview
  - Privacy information

---

## [2.0.0] - 2026-01-18

### Architecture & Planning Phase

Exploration and planning phase focused on architecting the plugin/server split and determining MVP scope.

### Added

- **Architecture decisions**
  - Separated browser extension (MV3) from backend server
  - Chose Go for zero-dependency server
  - Defined MCP (Model Context Protocol) as communication layer
  - Established data flow: extension → server → MCP stdio

- **MVP direction planning**
  - Core telemetry capabilities (console, network, exceptions)
  - Basic storage and session management
  - Initial Landing Page design
  - Security model (localhost-only, ephemeral sessions)

- **Early implementation**
  - Browser extension scaffold (MV3 foundation)
  - Go server foundation
  - MCP integration approach

### Notes

This was the architecture exploration phase that bridged the initial plugin-only POC (v1.0.0) with the full MVP implementation (v3.0.0). Established the core design patterns and constraints that shaped production development.

---

## [1.0.0] - 2026-01-16

### Added

- **Initial proof-of-concept** — Browser plugin generation prototype
- **Experimental features**
  - Basic browser extension scaffold generation
  - Development foundations for MCP integration
  - Early architecture exploration

### Notes

This was an initial POC under a different project name, exploring browser plugin generation capabilities. Evolved into Kaboom v3.0.0 with full MCP implementation and production-ready architecture.
