# Connected UAT transcripts

Recorded daemon↔extension command exchanges, one file per connected category
(`cat-<id>.jsonl`), plus an optional shared `connected.jsonl` fallback.

They exist so the connected categories — the only tests that prove a browser
feature still works — can run headless. Without them the "Connected Suite
(Replay)" CI job annotates the build as **NOT VERIFIED** rather than passing
quietly, because a green job with no fixtures reads as coverage that does not
exist.

## Recording

Needs Chrome open with the Kaboom extension connected. Once per meaningful
change to what the extension returns:

```bash
scripts/tests/record-connected-transcripts.sh            # every category
scripts/tests/record-connected-transcripts.sh --category 36
```

The script refuses to run when this directory has uncommitted changes: a
partial re-record is hard to tell from a real diff.

## Replaying

```bash
KABOOM_UAT_REPLAY=scripts/tests/transcripts \
  scripts/uat/runners/test-all-tools-comprehensive.sh --suite connected
```

## Reviewing a diff

A changed transcript means the browser now answers differently. That is either
the feature change you just made — in which case the diff is the evidence — or
drift nobody intended. Read it; do not re-record to make a failure go away.

A command with no recorded answer is replayed as an **error**, never as an empty
success, so a stale transcript fails loudly instead of turning every category
green against a fixture that answers nothing.

The `Connected Canary` workflow re-records on a browser-equipped self-hosted
runner and fails when the live command set differs from what is committed here.
It runs when something that can change the exchange changes — the extension, the
daemon's command dispatch, the wire contracts, these fixtures, or the record and
replay machinery — not on a schedule. A transcript goes stale because code
changed, so a clock-driven run on an unchanged tree proves nothing.
