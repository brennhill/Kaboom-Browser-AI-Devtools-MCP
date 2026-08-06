---
doc_type: checklist
status: active
last_reviewed: 2026-03-05
owners: []
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Safe Edit Checklist

Use this checklist when requesting or implementing changes.

## 1) Edit Contract (required)

- Goal:
- Allowed scope (files/directories):
- No-touch zones:
- Behavior that must not change:
- Interface compatibility requirements:
- Data/security constraints:
- Performance/reliability constraints:

## 2) Canonical Docs Reviewed (required)

- [ ] [system-overview.md](../platform/system-overview.md)
- [ ] [module-map.md](module-map.md)
- [ ] [invariants.md](invariants.md)
- [ ] [interfaces.md](interfaces.md)
- [ ] Relevant ADR(s) in this folder

## 3) Validation Gates (pick all applicable)

- [ ] `go test ./cmd/browser-agent/...`
- [ ] `go test ./internal/...`
- [ ] `make test`
- [ ] `make check-wire-drift`
- [ ] `./scripts/validate-architecture.sh`
- [ ] Feature-specific command(s): `...`

## 4) Stop-and-Ask Triggers

Stop and ask before proceeding if any of these occur:

- A requirement conflicts with an invariant or ADR.
- The smallest safe change still touches a no-touch zone.
- A public interface change appears unavoidable.
- Tests are missing for a high-risk path.

## 5) Definition of Done

- [ ] Constraints above are still true after the change.
- [ ] Relevant tests and checks pass.
- [ ] Architecture docs and/or ADRs updated if behavior changed.
- [ ] PR/commit note includes risk and rollback plan.
