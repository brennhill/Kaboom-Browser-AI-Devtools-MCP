#!/bin/bash
# cat-33-expectations.sh — What each mode's response must actually contain.
#
# cat-33 invokes every mode in the live schema, but for most of them the only
# thing it could check was "the response was not an MCP error". A mode that
# returns an empty body, a wrong payload, or the same request duplicated twelve
# times satisfies that — which is how the network waterfall inflated on every
# read for as long as it did while its test passed (kaboom-8cyd).
#
# This table gives a mode a *content* expectation: an extended regex the
# response text must match. Patterns name a field the handler can only produce
# by doing its work, so an empty success is a failure.
#
# Every pattern here was captured from a live daemon and extension rather than
# read off a struct — several handlers return a different shape than their Go
# type suggests, because the response is assembled downstream.
#
# Modes with no entry fall through to `reachability_only`, are counted, and are
# held under a baseline that may only shrink (see UAT_REACHABILITY_BASELINE).
#
# Docs: docs/features/feature/self-testing/index.md

# Returns an ERE the response content must match, or `reachability_only`.
action_content_expectation() {
    case "$1/$2" in
        # ── daemon state: no browser required ──────────────
        configure/describe_capabilities) echo '"params"' ;;
        configure/tutorial) echo '"next_steps"' ;;
        configure/list_sequences) echo '"sequences"' ;;
        configure/security_mode) echo '"security_mode"' ;;
        configure/action_jitter) echo '"action_jitter_ms"' ;;
        configure/telemetry) echo '"telemetry_mode"' ;;
        configure/load) echo '"project_id"|"session_count"' ;;
        configure/health) echo '"extension_connected"' ;;
        configure/doctor) echo '"checks"|"diagnostics"|"status"' ;;
        observe/inbox) echo '"events"' ;;
        generate/test_classify) echo '"classification"' ;;

        # KNOWN-WEAK. Browser-mediated modes return an async lifecycle envelope
        # (queued/status/result/correlation_id) with the real payload nested
        # under .result, so matching "result" proves the query was queued and
        # nothing about what came back. Replace once the payload shape is
        # declared — see kaboom-jp5i.
        analyze/feature_gates) echo '"result"' ;;

        # ── DOM: asserted against interact.html ────────────
        interact/query) echo '"exists"' ;;
        interact/get_value) echo '"value"' ;;
        interact/get_text) echo '"text"|"success"' ;;
        interact/list_interactive) echo '"candidate_count"' ;;
        interact/get_readable) echo '"word_count"' ;;
        interact/get_markdown) echo '"markdown"|"content"' ;;
        interact/wait_for_stable) echo '"mutations_observed"' ;;
        interact/batch) echo '"steps"|"results"' ;;
        analyze/forms) echo '"forms"' ;;
        analyze/page_structure) echo '"frameworks"' ;;
        analyze/computed_styles) echo '"elements"' ;;
        analyze/dom) echo '"elements"|"nodes"|"html"' ;;

        # ── capture buffers: the payload must be the documented
        #    collection, not a bare success envelope ────────
        observe/logs) echo '"logs"' ;;
        observe/errors) echo '"errors"' ;;
        observe/network_waterfall) echo '"entries"' ;;
        observe/tabs) echo '"tabs"' ;;
        observe/page) echo '"url"' ;;
        observe/recordings) echo '"recordings"|"active_recording_id"' ;;
        observe/storage) echo '"items"|"keys"|"storage_type"' ;;
        analyze/accessibility) echo '"violations"' ;;

        # ── daemon-owned state: captured from a live daemon on 2026-09-05 by
        #    replaying cat-33's own arguments against a daemon with no
        #    extension attached, so each pattern is a field only the daemon can
        #    produce and none of them depends on a browser being present.
        #
        #    These are SHAPE assertions: they prove the handler emitted its
        #    documented collection rather than an error or a bare success
        #    envelope. They do not prove the collection has the right contents —
        #    that is what the human rig asks a person (scripts/uat/human).
        analyze/annotations) echo '"annotations"' ;;
        analyze/draw_history) echo '"sessions"' ;;
        analyze/error_clusters) echo '"clusters"' ;;
        analyze/navigation_patterns) echo '"entries"' ;;
        analyze/security_audit) echo '"findings"' ;;
        analyze/third_party_audit) echo '"third_parties"' ;;
        analyze/verification) echo '"contract"' ;;
        analyze/visual_baselines) echo '"keys"|"namespace"' ;;
        configure/diff_sessions) echo '"snapshots"' ;;
        configure/event_recording_start) echo '"recording_id"' ;;
        configure/network_recording) echo '"active"' ;;
        configure/qa_fixture) echo '"valid"' ;;
        configure/report_issue) echo '"formatted_body"' ;;
        configure/streaming) echo '"config"' ;;
        configure/test_boundary_start) echo '"test_id"' ;;
        generate/csp) echo '"policy"' ;;
        generate/har) echo '"log"' ;;
        generate/pr_summary) echo '"stats"' ;;
        generate/reproduction) echo '"script"' ;;
        generate/test) echo '"action_count"|"script"' ;;
        interact/list_states) echo '"states"' ;;
        observe/actions) echo '"entries"' ;;
        observe/error_bundles) echo '"bundles"' ;;
        observe/failed_commands) echo '"commands"' ;;
        observe/history) echo '"entries"' ;;
        observe/network_bodies) echo '"entries"' ;;
        observe/pending_commands) echo '"completed"' ;;
        observe/pilot) echo '"extension_connected"' ;;
        observe/saved_videos) echo '"recordings"' ;;
        observe/summarized_logs) echo '"groups"' ;;
        observe/timeline) echo '"entries"' ;;
        observe/transients) echo '"entries"' ;;
        observe/vitals) echo '"metrics"' ;;
        observe/websocket_events) echo '"entries"' ;;
        observe/websocket_status) echo '"connections"' ;;

        *) echo "reachability_only" ;;
    esac
}

# How many modes are still allowed to pass on reachability alone.
#
# This is a ratchet, not a target. cat-33 fails if the count exceeds it, so a
# newly added mode cannot quietly join the untested majority, and every mode
# that gains a real expectation must be paid for by lowering this number.
UAT_REACHABILITY_BASELINE="${UAT_REACHABILITY_BASELINE:-107}"
