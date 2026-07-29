#!/bin/bash
# uat-artifacts.sh — Emit canonical JSON and JUnit reports from UAT category records.

uat_emit_artifacts() {
    local category_records="$1"
    local json_path="$2"
    local junit_path="$3"
    local suite="$4"
    local elapsed="$5"
    local restoration="$6"
    local readiness="$7"

    mkdir -p "$(dirname "$json_path")" "$(dirname "$junit_path")"
    jq -s \
        --arg suite "$suite" \
        --argjson elapsed_seconds "$elapsed" \
        --arg restoration_status "$restoration" \
        --arg connected_readiness "$readiness" \
        '{
          schema_version: 1,
          suite: $suite,
          elapsed_seconds: $elapsed_seconds,
          prerequisites: {
            connected_readiness: $connected_readiness
          },
          restoration: {
            status: $restoration_status
          },
          categories: .,
          totals: {
            pass: (map(.pass) | add // 0),
            fail: (map(.fail) | add // 0),
            skip: (map(.skip) | add // 0),
            tests: (map(.total) | add // 0),
            aggregation_errors: (map(select(.result_status != "complete")) | length)
          }
        }' "$category_records" > "$json_path"

    jq -r '
      def xml:
        tostring
        | gsub("&"; "&amp;")
        | gsub("<"; "&lt;")
        | gsub(">"; "&gt;")
        | gsub("\""; "&quot;")
        | gsub("'"'"'"; "&apos;");
      . as $root |
      "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
      "<testsuites name=\"Kaboom UAT\" tests=\"\($root.totals.tests)\" failures=\"\($root.totals.fail + $root.totals.aggregation_errors)\" skipped=\"\($root.totals.skip)\" time=\"\($root.elapsed_seconds)\">\n" +
      (
        $root.categories | map(
          "  <testsuite name=\"\(.name | xml)\" tests=\"\(.total)\" failures=\"\(.fail + (if .result_status == "complete" then 0 else 1 end))\" skipped=\"\(.skip)\" time=\"\(.elapsed_seconds)\">\n" +
          (if .result_status == "complete" then "" else "    <testcase name=\"category result\"><failure message=\"\(.result_status | xml)\"/></testcase>\n" end) +
          (.skip_reasons | map("    <testcase name=\"\(. | xml)\"><skipped message=\"\(. | xml)\"/></testcase>\n") | join("")) +
          "  </testsuite>\n"
        ) | join("")
      ) +
      "</testsuites>\n"
    ' "$json_path" > "$junit_path"
}
