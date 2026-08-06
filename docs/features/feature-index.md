---
status: active
scope: feature/index
ai-priority: high
tags: [feature-index, status, navigation, canonical]
relates-to: [../README.md, feature/]
last-verified: 2026-03-05
canonical: true
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Feature Index

> **For LLM agents:** This is the canonical index of all Kaboom features. Use the `status` column to determine what is implemented vs. planned. Use the `tool` and `mode` columns to find the correct MCP interface.

## Status Legend

| Status | Meaning |
|--------|---------|
| **shipped** | Implemented, tested, available in current release |
| **in-progress** | Under active development |
| **proposed** | Specified but not yet implemented |
| **deprecated** | Scheduled for removal |

## Feature Table

| Feature | Status | Tool | Mode/Action | Version | Docs |
|---------|--------|------|-------------|---------|------|
| AI Capture Control | shipped | configure | capture | 5.0.0 | [feature/ai-capture-control](feature/ai-capture-control/) |
| AI Web Pilot | shipped | interact | highlight, save_state, load_state, execute_js, navigate | 5.0.0 | [feature/ai-web-pilot](feature/ai-web-pilot/) |
| Accessibility Audit | shipped | analyze, generate | accessibility, sarif | 5.0.0 | [feature/sarif-export](feature/sarif-export/) |
| Agentic CI/CD | proposed | observe, interact | (multi-mode) | — | [feature/agentic-cicd](feature/agentic-cicd/) |
| Agentic E2E Repair | proposed | observe, generate | (multi-mode) | — | [feature/agentic-e2e-repair](feature/agentic-e2e-repair/) |
| API Key Auth | shipped | configure | (request validation) | 5.0.0 | [feature/api-key-auth](feature/api-key-auth/) |
| API Schema Inference | deprecated | — | — | 5.0.0 | [feature/api-schema](feature/api-schema/) |
| Behavioral Baselines | in-progress | analyze | performance | — | [feature/behavioral-baselines](feature/behavioral-baselines/) |
| Binary Format Detection | shipped | observe | network_bodies | 5.0.0 | [feature/binary-format-detection](feature/binary-format-detection/) |
| Budget Thresholds | in-progress | configure | health | — | [feature/budget-thresholds](feature/budget-thresholds/) |
| Causal Diffing | deprecated | — | — | — | [feature/causal-diffing](feature/causal-diffing/) |
| Compressed Diffs | deprecated | — | — | 5.0.0 | [feature/compressed-diffs](feature/compressed-diffs/) |
| Config Profiles | proposed | configure | (settings) | — | [feature/config-profiles](feature/config-profiles/) |
| Context Streaming | proposed | configure | streaming | — | [feature/context-streaming](feature/context-streaming/) |
| Deployment Watchdog | proposed | analyze | performance | — | [feature/deployment-watchdog](feature/deployment-watchdog/) |
| DOM Fingerprinting | in-progress | interact, observe | dom_fingerprint | — | [feature/dom-fingerprinting](feature/dom-fingerprinting/) |
| Dynamic Exposure | proposed | configure | (feature flags) | — | [feature/dynamic-exposure](feature/dynamic-exposure/) |
| Enterprise Audit | shipped | analyze | security_audit | 5.0.0 | [feature/enterprise-audit](feature/enterprise-audit/) |
| Error Clustering | shipped | analyze | error_clusters | 5.0.0 | [feature/error-clustering](feature/error-clustering/) |
| Framework Selector Resilience | shipped | analyze, interact | page_structure, navigate/click/type/list_interactive/wait_for | 0.7.12 | [feature/framework-selector-resilience](feature/framework-selector-resilience/) |
| Kaboom CI | proposed | observe, generate | CI integration | — | [feature/kaboom-ci](feature/kaboom-ci/) |
| HAR Export | deprecated | — | — | 5.0.0 | [feature/har-export](feature/har-export/) |
| In-Browser Agent Panel | proposed | observe, analyze, interact, configure | agent_panel, agent_watch (proposed) | — | [feature/in-browser-agent-panel](feature/in-browser-agent-panel/) |
| Interception Deferral | in-progress | observe, configure | (network buffering) | — | [feature/interception-deferral](feature/interception-deferral/) |
| Memory Enforcement | shipped | configure | health | 5.0.0 | [feature/memory-enforcement](feature/memory-enforcement/) |
| Noise Filtering | shipped | configure | noise_rule | 5.0.0 | [feature/noise-filtering](feature/noise-filtering/) |
| Performance Budget | shipped | configure, analyze | health, performance | 5.0.0 | [feature/performance-budget](feature/performance-budget/) |
| Persistent Memory | shipped | configure | store, load | 5.0.0 | [feature/persistent-memory](feature/persistent-memory/) |
| Push Alerts | shipped | observe | (alert system) | 5.0.0 | [feature/push-alerts](feature/push-alerts/) |
| Push Regression | shipped | analyze | performance | 5.0.0 | [feature/push-regression](feature/push-regression/) |
| Query DOM | shipped | analyze | dom | 5.0.0 | [feature/query-dom](feature/query-dom/) |
| Rate Limiting | shipped | configure | (throttling) | 5.0.0 | [feature/rate-limiting](feature/rate-limiting/) |
| Redaction Patterns | shipped | configure | (data masking) | 5.0.0 | [feature/redaction-patterns](feature/redaction-patterns/) |
| Reproduction Enhancements | shipped | generate | reproduction, test_from_context, test_heal | 5.0.0 | [feature/reproduction-enhancements](feature/reproduction-enhancements/) |
| SARIF Export | shipped | generate | sarif | 5.0.0 | [feature/sarif-export](feature/sarif-export/) |
| Security Hardening | shipped | configure | (security config) | 5.0.0 | [feature/security-hardening](feature/security-hardening/) |
| Self-Healing Tests | proposed | observe, generate | (test auto-repair) | — | [feature/self-healing-tests](feature/self-healing-tests/) |
| Self-Testing | in-progress | interact, generate | execute_js, test_from_context | — | [feature/self-testing](feature/self-testing/) |
| SPA Route Measurement | in-progress | analyze, observe | performance, timeline | — | [feature/spa-route-measurement](feature/spa-route-measurement/) |
| Temporal Graph | shipped | analyze | history | 5.0.0 | [feature/temporal-graph](feature/temporal-graph/) |
| Test Harness V2 | proposed | — | — | — | [feature-test-harness.md](feature-test-harness.md) |
| TTL Retention | shipped | configure | (data TTL) | 5.0.0 | [feature/ttl-retention](feature/ttl-retention/) |
| Web Vitals | shipped | observe | vitals | 5.0.0 | [feature/web-vitals](feature/web-vitals/) |
| Workflow Integration | proposed | observe, generate | (CI integration) | — | [feature/workflow-integration](feature/workflow-integration/) |

## Summary

| Status | Count |
|--------|-------|
| Shipped | 24 |
| In-Progress | 6 |
| Proposed | 10 |
| Deprecated | 4 |
| **Total** | **44** |

## MCP Tool Distribution

| Tool | Shipped Modes | Description |
|------|--------------|-------------|
| **observe** | errors, logs, extension_logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, timeline, error_bundles, screenshot, command_result, pending_commands, failed_commands, saved_videos, recordings, recording_actions, playback_results, log_diff_report | Read captured browser state |
| **analyze** | dom, performance, accessibility, error_clusters, history, security_audit, third_party_audit, link_health, link_validation, page_summary, annotations, annotation_detail, api_validation, draw_history, draw_session | Active analysis and audits |
| **generate** | reproduction, test, pr_summary, har, csp, sri, sarif, visual_test, annotation_report, annotation_issues, test_from_context, test_heal, test_classify | Produce artifacts from captured data |
| **configure** | store, load, noise_rule, clear, health, streaming, test_boundary_start, test_boundary_end, recording_start, recording_stop, playback, log_diff, telemetry, diff_sessions, audit_log | Session settings and utilities |
| **interact** | highlight, subtitle, save_state, load_state, list_states, delete_state, execute_js, navigate, refresh, back, forward, new_tab, screenshot, click, type, select, check, get_text, get_value, get_attribute, set_attribute, focus, scroll_to, wait_for, key_press, paste, list_interactive, screen_recording_start, screen_recording_stop, upload, draw_mode_start | Browser control and automation |
