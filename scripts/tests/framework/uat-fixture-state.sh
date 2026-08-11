#!/bin/bash
# uat-fixture-state.sh — What each armed fixture is known to contain.
#
# One source of truth for fixture expectations. Categories assert against these
# values instead of copying literals, so changing a fixture updates one file
# rather than silently invalidating assertions scattered across categories.
#
# The arming scripts here are the *only* place that decides what state a fixture
# holds; framework.sh's arm_fixture runs them. Counts below are exact because
# arm_fixture clears the capture buffers first — without that they would be
# cumulative and only "at least one" would be assertable.
#
# Docs: docs/features/feature/self-testing/index.md

# ── telemetry.html ─────────────────────────────────────────
#
# Fires a known mix of console noise and real errors. The mix is the point: the
# spam is the negative control. A log filter that returns everything passes an
# errors-only assertion, and only fails when something proves the warnings and
# info lines were excluded.
#
# fireConsoleSpam(6) -> 6 console entries cycling log/warn/info, each matching
#                       '[HMR] update #N — module chunk reloaded'
# fireConsoleErrors(3) -> 3 console.error entries matching 'SMOKE_ERROR_<ts>_<i>'
# throwTypeError()     -> console.error 'TypeError: ...' (null property read)
# throwReferenceError()-> console.error 'ReferenceError: ...' from a nested stack
UAT_ARM_TELEMETRY='fireConsoleSpam(6); fireConsoleErrors(3); throwTypeError(); throwReferenceError(); "armed"'

# MEASURED against a live daemon + extension on 2026-08-11, not assumed.
# Arming produces exactly 5 captured log entries, all at level=error:
#   SMOKE_ERROR_<ts>_0, _1, _2, then 'TypeError:', then 'ReferenceError:'.
UAT_TELEMETRY_ERROR_COUNT=5

# The six fireConsoleSpam entries are captured as ZERO, and that is correct
# behaviour rather than a capture gap: they match the builtin_hmr_console noise
# rule (internal/noise/noise_builtin.go:156, MessageRegex ^\[(vite|HMR|webpack|next)\]).
#
# That makes the spam a stronger negative control than a plain level filter. A
# log pipeline that returned everything would show 11 entries here; one that
# only implemented min_level would still show the log/warn/info spam when asked
# for all levels. Asserting 5-and-only-5 proves noise suppression is actually
# running, which nothing in the suite currently verifies.
UAT_TELEMETRY_SPAM_CAPTURED_COUNT=0
UAT_TELEMETRY_SPAM_MARKER='module chunk reloaded'   # must NEVER appear in observe(logs)

# Substrings guaranteed present in the error stream after arming.
UAT_TELEMETRY_ERROR_MARKER='SMOKE_ERROR_'
UAT_TELEMETRY_TYPEERROR_MARKER='TypeError'
UAT_TELEMETRY_REFERROR_MARKER='ReferenceError'

# ── network state ──────────────────────────────────────────
#
# Assert on distinct URLs and statuses, NOT on counts, until kaboom-b8xy is
# fixed: the inject-side poller re-sends the page's entire resource-timing
# table on every poll (src/inject/message-handlers.ts:456 calls
# getNetworkWaterfall({}) with no `since`), and the store appends without
# dedup, so the waterfall grows ~8 entries every 3 seconds with no new traffic.
# Measured 32 entries for 8 distinct (url, start_time) pairs.
UAT_ARM_TELEMETRY_NETWORK='fetch404(); fetch500(); "armed"'
UAT_NETWORK_404_PATH='/tests/404'
UAT_NETWORK_500_PATH='/tests/500'
UAT_NETWORK_ABSENT_PATH='/tests/never-requested-by-any-fixture'

# ── performance.html ───────────────────────────────────────
#
# runLongTask blocks the main thread synchronously, so the task is real rather
# than simulated; triggerCLS expands a box with transition:none so the shift is
# counted rather than animated away.
UAT_ARM_PERFORMANCE='runLongTask(250); triggerCLS(); "armed"'
UAT_PERFORMANCE_LONG_TASK_MS=250
UAT_PERFORMANCE_CLS_ELEMENT='cls-box'

# ── a11y.html ──────────────────────────────────────────────
#
# Violations are static in the markup, so no arming is needed. These are the
# rules an accessibility audit must report; anything else it reports on this
# page is a false positive worth investigating.
UAT_A11Y_EXPECTED_RULES='label|image-alt|color-contrast|button-name'
UAT_A11Y_VIOLATION_ELEMENT='bad-input-name'

# ── interact.html ──────────────────────────────────────────
#
# The DOM anchors connected categories assert against.
UAT_INTERACT_FORM_SELECTOR='#smoke-form-dom'
UAT_INTERACT_ABSENT_SELECTOR='#definitely-not-present-in-the-fixture'
UAT_INTERACT_RESULT_SELECTOR='#sf-result'
