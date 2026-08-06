#!/bin/bash
# 04-network-websocket.sh — 4.1-4.2: WebSocket capture, network waterfall.
set -eo pipefail

begin_category "4" "Network & WebSocket" "2"

# ── Test 4.1: Real WebSocket traffic ─────────────────────
begin_test "4.1" "[BROWSER] WebSocket capture on a real WS-heavy page" \
    "Navigate to a live trading page (Binance BTC/USDT), verify WS connections in observe" \
    "Tests: real WebSocket interception > extension > daemon > MCP observe"

run_test_4_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"what":"navigate","url":"https://www.binance.com/en/trade/BTC_USDT","reason":"Load WebSocket-heavy page"}' 20

    sleep 5

    local response
    response=$(call_tool "observe" '{"what":"websocket_status"}')
    local content_text
    content_text=$(extract_content_text "$response")

    local active_count
    active_count=$(echo "$content_text" | grep -oE '"active_count":[0-9]+' | head -1 | grep -oE '[0-9]+' || true)

    if [ -n "$active_count" ] && [ "$active_count" -gt 0 ] 2>/dev/null; then
        echo "  [active connections: $active_count]"
        local json_part
        json_part=$(echo "$content_text" | sed -n '/{/,$ p' | tr '\n' ' ')
        echo "$json_part" | jq -r '.connections[:3][] | "    \(.state): \(.url[:80])\n      incoming: \(.message_rate.incoming.total // 0) msgs (\(.message_rate.incoming.per_second // 0)/s)"' 2>/dev/null || true
        pass "WebSocket capture: $active_count active connection(s) on Binance trading page."
    else
        local events_response
        events_response=$(call_tool "observe" '{"what":"websocket_events","last_n":5}')
        local events_text
        events_text=$(extract_content_text "$events_response")

        if echo "$events_text" | grep -qi "binance\|stream\|ws"; then
            pass "WebSocket events captured from Binance (connections may have closed)."
        else
            fail "No WebSocket connections captured. Early-patch should intercept WS before page scripts."
        fi
    fi
}
run_test_4_1

# ── Test 4.2: Network waterfall has real data ───────────
begin_test "4.2" "[BROWSER] Network waterfall has real resource timing" \
    "observe(network_waterfall) should return entries with real URLs and timing" \
    "Tests on-demand extension query: MCP > daemon > extension > performance API"

run_test_4_2() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed resource timing buffer","script":"fetch(window.location.href).then(function(r){return r.ok?\"fetched\":\"error\"}).catch(function(){return \"error\"})"}'
    sleep 0.5

    local response
    response=$(call_tool "observe" '{"what":"network_waterfall"}')

    if check_bridge_timeout "$response"; then
        skip "Bridge timeout on network_waterfall (extension query took >4s)."
        return
    fi

    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(network_waterfall)."
        return
    fi

    if echo "$content_text" | grep -qE 'https?://'; then
        local entry_count
        entry_count=$(echo "$content_text" | grep -oE '"count":[0-9]+' | head -1 | grep -oE '[0-9]+' || true)

        echo "  [sample entry]"
        local json_part_waterfall
        json_part_waterfall=$(echo "$content_text" | sed -n '/{/,$ p' | tr '\n' ' ')
        echo "$json_part_waterfall" | jq -r '.entries[0] | "    url:               \(.url[:80] // "")\n    initiator_type:    \(.initiator_type // "")\n    duration_ms:       \(.duration_ms // 0)\n    start_time:        \(.start_time // 0)\n    transfer_size:     \(.transfer_size // 0)\n    decoded_body_size: \(.decoded_body_size // 0)\n    page_url:          \(.page_url[:80] // "")"' 2>/dev/null || true

        local has_initiator has_start_time has_transfer
        has_initiator=$(echo "$content_text" | grep -oE '"initiator_type":"[a-z]' | head -1 || true)
        has_start_time=$(echo "$content_text" | grep -oE '"start_time":[1-9]' | head -1 || true)
        has_transfer=$(echo "$content_text" | grep -oE '"transfer_size":[1-9]' | head -1 || true)

        if [ -n "$has_initiator" ] && [ -n "$has_start_time" ] && [ -n "$has_transfer" ]; then
            pass "Real waterfall data: ${entry_count:-some} entries with URLs, timing, initiator types, and transfer sizes."
        else
            fail "Waterfall entries have URLs but missing fields: initiator_type=$([ -n "$has_initiator" ] && echo 'ok' || echo 'MISSING'), start_time=$([ -n "$has_start_time" ] && echo 'ok' || echo 'MISSING'), transfer_size=$([ -n "$has_transfer" ] && echo 'ok' || echo 'MISSING')."
        fi
    else
        fail "No real URLs in waterfall. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_4_2
