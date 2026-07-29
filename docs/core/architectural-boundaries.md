# Architectural Boundaries

Kaboom treats physical size limits as a floor, not proof of modularity.
`make check-structure` enforces the dependency graph and public surface in
addition to the 800-line and 10-file limits.

## Dependency direction

- `lib` is shared and cannot depend on feature entry points.
- `background`, `content`, `inject`, `offscreen`, and `popup` are sibling
  runtime contexts. They cannot import one another.
- Cross-context communication uses the canonical runtime-message and wire
  contracts, while shared computation moves completely into `lib`.

The executable policy lives in `.architecture-boundaries.json`. A forbidden
direction must be removed through a complete caller migration; compatibility
facades and pass-through re-exports are not exceptions.

## Public surfaces

Authored TypeScript files default to at most 24 exported declarations. The
configuration lists narrowly budgeted source-of-truth exceptions:

- extension constants and storage keys;
- cross-context runtime messages;
- shared utility types;
- the terminal state/timing contract;
- the ratcheted background state surface.

Each exception includes a reason and a maximum. It may shrink without updating
the policy, but it cannot grow silently.

## Duplicate code

Changes to `src/background` or `src/popup` must retain zero non-trivial clones
at the repository's standard 8-line/60-token threshold. This check is part of
`make check-structure`, alongside circular dependency reporting, dormant-test
detection, file length, and folder size.
