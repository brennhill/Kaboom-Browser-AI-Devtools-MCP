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
scripts/tests/transcripts/record-connected-transcripts.sh            # every category
scripts/tests/transcripts/record-connected-transcripts.sh --category 36
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

## Recording against a browser this repo controls

Recording needs an extension whose command contract matches the daemon. Relying
on whichever extension a machine happens to have loaded makes that a coin flip:
on 2026-09-05 every connected category failed with `command_contract_mismatch`
because the browser held an older build than the tree.

    KABOOM_UAT_LAUNCH_BROWSER=1 scripts/tests/transcripts/record-connected-transcripts.sh

starts a browser with `extension/` loaded from this tree and a throwaway profile,
so the extension under test is by construction the one just compiled. Set
`KABOOM_UAT_CHROME` to name the binary and `KABOOM_UAT_CHROME_HEADLESS=1` on a
runner with no display.

**It needs Chrome for Testing, not stable Chrome.** Chrome 137 removed
`--load-extension` from stable builds, and a stable Chrome given that switch
starts normally while loading nothing at all — measured on 152.0.7977.76: zero
requests to the daemon port in 120 seconds, no service worker target, no
extension recorded in the profile, with a real page open and with
`--disable-features=DisableLoadExtensionCommandLineSwitch`. That is
indistinguishable from a browser that attached and went quiet, so discovery
refuses stable Chrome by name rather than handing back a browser that will time
out for an invisible reason. Install one with
`npx @puppeteer/browsers install chrome@stable`.

**One browser at a time.** The daemon has a single extension slot — whichever
browser checked in last owns `extension_connected` and `command_contract_id`. If
another Chrome with the extension is polling the same port, the launcher refuses
to start rather than racing it. Close it, or point it at a different server URL
in the extension's options, and re-run.

### What the launched browser costs

A cold profile takes about 80 seconds to its first `/sync` check-in — the
service worker starts on an event and Chrome clamps the reconnect alarm well
above the 5 seconds the extension asks for. `uat_launch_extension_browser`
therefore waits 180 seconds by default, and a category that starts issuing
commands the instant readiness returns can still outrun a worker that has gone
idle between polls.

The browser is given its own port, and the extension it loads is a copy of
`extension/` with the compiled `DEFAULT_SERVER_URL` repointed at it. That is
what keeps the developer's own Chrome — which polls 7890 — out of the run.
Measured end to end: the staged copy checks in on 7899 carrying this tree's
command contract, and an `interact new_tab` through it returns a real tab id.
