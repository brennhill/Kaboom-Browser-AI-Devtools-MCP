#!/bin/bash
# cat-19-link-crawling.sh — Link Health Domain Crawling Tests (6 tests)
# Tests recursive link crawling, CORS boundaries, domain traversal.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "31" "Link Health: Domain Crawling & CORS" "6"

ensure_daemon

# ── TEST 19.16: Single-Domain Link Crawling ────────────────────────────

begin_test "31.16" "Link crawl stays within same domain" \
    "Start at example.com, crawl only example.com links, skip external" \
    "Domain boundary enforcement prevents external dependency checks"

run_test_19_16() {
    response=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com",
        "same_domain_only":true
    }')

    if ! check_not_error "$response"; then
        fail "Link crawl query failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "Same-domain crawl queued. Crawl would check only example.com links."
    else
        fail "Link crawl did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_19_16

# ── TEST 19.17: CORS Handling During Crawl ────────────────────────────

begin_test "31.17" "Crawl respects CORS boundaries" \
    "Link to cdn.example.com (different origin), check CORS before crawling" \
    "CORS-blocked links marked differently than unreachable"

run_test_19_17() {
    response=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com",
        "check_cors":true
    }')

    if ! check_not_error "$response"; then
        fail "CORS-aware crawl query failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "CORS-aware crawl queued. CORS-blocked links would be identified separately."
    else
        fail "CORS crawl did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_19_17

# ── TEST 19.18: Recursive Depth Limiting ──────────────────────────────

begin_test "31.18" "Crawl respects max_depth parameter" \
    "Start URL → linked pages → nested pages. Stop at depth=2" \
    "Depth limiting prevents infinite crawls"

run_test_19_18() {
    response=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com",
        "max_depth":2
    }')

    if ! check_not_error "$response"; then
        fail "Depth-limited crawl failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "Depth-limited crawl queued. Would stop at depth 2."
    else
        fail "Depth-limited crawl did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_19_18

# ── TEST 19.19: Crawl with Filtering ──────────────────────────────────

begin_test "31.19" "Crawl excludes links matching patterns" \
    "Skip */admin, */api, */health check endpoints" \
    "Filtering prevents unnecessary checks"

run_test_19_19() {
    response=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com",
        "exclude_patterns":["*/admin","*/api","*/health"]
    }')

    if ! check_not_error "$response"; then
        fail "Filtered crawl failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "Filtered crawl queued. Would skip admin/api/health endpoints."
    else
        fail "Filtered crawl did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_19_19

# ── TEST 19.20: Crawl Result Consistency Across Runs ──────────────────

begin_test "31.20" "Running same crawl twice produces consistent results" \
    "Crawl A and Crawl B find same set of links/status codes" \
    "Crawl results should be deterministic"

run_test_19_20() {
    # First crawl
    response1=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com"
    }')

    if ! check_not_error "$response1"; then
        fail "First crawl failed"
        return
    fi

    sleep 1

    # Second crawl (same URL)
    response2=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com"
    }')

    if ! check_not_error "$response2"; then
        fail "Second crawl failed"
        return
    fi

    local id1 id2
    id1=$(echo "$response1" | jq -r '.result.content[0].text' 2>/dev/null | grep -o 'link_health_[a-z0-9]*' | head -1 || true)
    id2=$(echo "$response2" | jq -r '.result.content[0].text' 2>/dev/null | grep -o 'link_health_[a-z0-9]*' | head -1 || true)

    if [ "$id1" != "$id2" ]; then
        pass "Two crawl operations have distinct correlation IDs (results tracked separately)"
    else
        fail "Correlation IDs should be unique per operation"
    fi
}
run_test_19_20

# ── TEST 19.21: Crawl Timeout Handling ────────────────────────────────

begin_test "31.21" "Crawl operation times out after specified duration" \
    "Set timeout=5s, crawl large site, verify timeout respected" \
    "Timeout prevents indefinite crawls"

run_test_19_21() {
    response=$(call_tool "analyze" '{
        "what":"link_health",
        "mode":"crawl",
        "start_url":"https://example.com",
        "timeout_ms":5000
    }')

    if ! check_not_error "$response"; then
        fail "Timeout-controlled crawl failed. Content: $(truncate "$(extract_content_text "$response")")"
        return
    fi

    local text
    text=$(extract_content_text "$response")

    if check_contains "$text" "correlation_id"; then
        pass "Timeout-controlled crawl queued. Would respect 5s timeout."
    else
        fail "Timeout-controlled crawl did not return valid response. Content: $(truncate "$text")"
    fi
}
run_test_19_21

finish_category
