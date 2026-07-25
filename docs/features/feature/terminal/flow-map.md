---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
canonical_flow_map: ../../../architecture/flow-maps/terminal-side-panel-host.md
---

# Terminal Flow Map Pointer

Canonical flow maps:

- [Terminal Side Panel Host and Launcher Coordination](../../../architecture/flow-maps/terminal-side-panel-host.md)
- [Terminal Server Isolation](../../../architecture/flow-maps/terminal-server-isolation.md)
- [DRY Test Helpers and Daemon Header Consolidation](../../../architecture/flow-maps/dry-test-helper-and-daemon-header-consolidation.md)

Latest sync update (2026-03-28): the terminal side panel is now specified as a tab-group-backed Kaboom workspace; power closes the panel and session, minimize hides the panel but preserves the session.

Round-3 connection hardening (2026-07-25): reconnect/replay/subscribe path made
robust — ErrFanoutFull no longer misreported as `exited`; scrollback-replay writes
are now write-deadline bound; scrollback snapshot + fanout subscribe are atomic
(`Relay.SubscribeWithHistory`); input typed during replay is buffered FIFO; reconnect
recovery is bounded on both the parent (`sidepanel.ts`) and the iframe
(`terminal.html`); self-heal uses an atomic `Map.ReplaceRelay`; init uses a unique
fanout sub id; the daemon takeover probe skips retries on connection-refused; and a
stuck-writer `WriteBuffer.Close` timeout is now logged instead of leaking silently.
See the `*_test.go` and `terminal-html-reconnect.test.js` guards listed in
`index.md`.
