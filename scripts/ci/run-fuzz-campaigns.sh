#!/bin/bash
# run-fuzz-campaigns.sh — Owns deterministic seed replay and bounded nightly Go fuzz campaigns.
set -euo pipefail

MODE="${1:---smoke}"
FUZZ_TIME="${KABOOM_FUZZ_TIME:-30s}"
ARTIFACT_DIR="artifacts/fuzz"
mkdir -p "$ARTIFACT_DIR"

TARGETS=(
  "./internal/qafixture|FuzzParseFixture"
  "./internal/qafixture|FuzzRegistryGenerationTransitions"
  "./internal/statediag|FuzzCollectorLifecycleTransitions"
  "./internal/capture|FuzzSyncRequestCanonicalRoundTrip"
  "./internal/redaction|FuzzRedactJSON"
  "./internal/security/scan|FuzzSecurityPatterns"
)

case "$MODE" in
  --smoke) CAMPAIGN_KIND="seed_replay" ;;
  --nightly) CAMPAIGN_KIND="bounded_mutation" ;;
  *) echo "usage: $0 --smoke|--nightly" >&2; exit 2 ;;
esac

{
  echo "campaign_kind=$CAMPAIGN_KIND"
  echo "fuzz_time=$FUZZ_TIME"
  echo "go_version=$(go version)"
  echo "commit=$(git rev-parse HEAD 2>/dev/null || echo unknown)"
  printf 'targets=%s\n' "${TARGETS[*]}"
} > "$ARTIFACT_DIR/manifest.txt"

for entry in "${TARGETS[@]}"; do
  package="${entry%%|*}"
  target="${entry##*|}"
  log="$ARTIFACT_DIR/$target.log"
  if [ "$MODE" = "--smoke" ]; then
    go test "$package" -run "^${target}$" -count=1 2>&1 | tee "$log"
  else
    go test "$package" -run "^${target}$" -fuzz "^${target}$" -fuzztime "$FUZZ_TIME" 2>&1 | tee "$log"
  fi
done
