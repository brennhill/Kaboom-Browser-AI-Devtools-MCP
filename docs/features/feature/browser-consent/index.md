---
doc_type: feature_index
feature_id: feature-browser-consent
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-03
code_paths:
  - internal/browserconsent/policy.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/interactdispatch/handler.go
  - cmd/browser-agent/tools_configure_consent.go
  - cmd/browser-agent/tools_interact_dispatch.go
  - internal/schema/configure/properties_core.go
  - internal/tools/configure/capabilities/modespecs_configure.go
  - cmd/browser-agent/internal/cli/parser/generate_configure.go
test_paths:
  - internal/browserconsent/policy_test.go
  - cmd/browser-agent/internal/toolguard/consent_guard_test.go
  - cmd/browser-agent/tools_configure_consent_test.go
---

# Browser driving consent

Per-origin consent for **driving** the browser, as distinct from **observing** it.

## Why this exists

Kaboom holds `<all_urls>` host permissions and the `debugger` permission. Since
`kaboom-05ue.1`, input runs over a persistent CDP session producing `isTrusted: true`
events. Anything that reaches the interact tool can therefore click and type as the user on
every origin the browser is signed into.

The pre-existing domain machinery does not cover this:

| Mechanism | Governs |
| --- | --- |
| `src/options.ts` allowlist/blocklist | which domains are **observed** |
| `src/lib/tabs/cloaked-domains.ts` | two domains where kaboom **disables itself** |
| this feature | which origins may be **driven** |

Both comparable products gate driving. Codex keeps an origins allowlist in
`~/.codex/browser/config.toml` plus a narrower per-session list; Claude in Chrome prompts
per domain and checks on every tool call.

## Design

**Fail closed.** Gating is defined by an explicit *read-only* set (`readOnlyActions`), not by
a list of mutating actions. `isMutationAction` in `toolinteract` is a hand-maintained
allowlist of mutating actions — correct for deciding when to capture evidence, wrong as a
security boundary, because an action added later would default to "not mutating" and skip
the gate. Here, an unrecognized action is gated until someone decides otherwise.

**Enforced on the Go side, before dispatch.** The check runs in
`interactdispatch.preDispatch`, the single choke point every interact action passes through,
so a refused action has no side effects and no second call path can route around it.

**The target is the origin being acted upon.** An action carrying an explicit `url` is
checked against *that* origin — for `navigate` the destination is what will be driven, and
checking the current page would gate the origin being left rather than the one being entered.
Everything else is checked against the tracked tab's URL.

**An unresolvable target is refused.** A gated action whose origin cannot be determined
(`about:blank`, `chrome://`, empty) is denied rather than defaulted, because that is the case
where proceeding is least safe.

**Origins are exact.** Consent for `https://example.com` does not extend to
`https://evil.example.com`, `http://example.com`, or `https://example.com:8443`.

**Loopback is allowed by default**, so local development is not gated into uselessness. The
default is revocable with `deny_localhost`.

**Refusals are not retryable.** Retrying without a grant produces the same refusal; marking
it retryable is how a bounded failure turns into a burned retry budget.

## Privacy

`OriginOf` reduces a URL to `scheme://host[:port]`, dropping path, query and fragment, so a
consent entry or a refusal message can never carry a token or an email address into a stored
list, a response, or a log (rules 7 and 13). Tests assert this directly.

## Usage

```bash
configure(mode='consent', action='list')
configure(mode='consent', action='allow',         origin='https://example.com')
configure(mode='consent', action='allow_session', origin='https://staging.example.com')
configure(mode='consent', action='revoke',        origin='https://example.com')
configure(mode='consent', action='clear_session')
configure(mode='consent', action='deny_localhost')
```

A refusal names the origin and the exact command that grants it.

## Known limitation

`batch` is gated as a whole against the tracked tab's origin; its individual steps are not
re-checked, so a batch may navigate to a second origin and act there under the first
origin's grant. Tracked as `kaboom-05ue.9`.

## Related

- `kaboom-05ue.2` — this feature
- `kaboom-05ue.1` — the persistent CDP session that made the gate necessary
- `kaboom-05ue.9` — the batch sub-step gap above
- `kaboom-x0li.3` — content provenance, which informs how much to trust a consented origin
