---
feature: Session to Test
status: in-progress
doc_type: tech-spec
feature_id: feature-session-to-test
last_reviewed: 2026-09-05
---

# Tech Spec: Session to Test

## Data flow

```
page (src/lib/page/reproduction.ts)
  computeSelectors      -> selectors        (locator 1)
  computeAXLocator      -> ax               (locator 2)
  computeViewportLocator-> viewport         (locator 3)
        |
        v  postMessage kaboom_enhanced_action
background (src/background/message-routing/telemetry-handler.ts)
  environmentPinFor(tab) -> environment     (pin stamp, only when the tab is pinned)
        |
        v  HTTP, WireEnhancedAction
daemon (internal/capture/actionstore) -> internal/reproduction
  BuildLocators / SessionPin -> Playwright and kaboom-native artifacts
```

## Wire contract

`internal/types/wire_enhanced_action.go` is the source of truth; `src/types/wire/wire-enhanced-action.ts`
is generated from it by `scripts/build/generate-wire-types.js`. Added there:

- `WireAXLocator` — `ref`, `role`, `name`.
- `WireViewportLocator` — `x`, `y`, `width`, `height`, `frame_url`, `viewport_width`,
  `viewport_height`, `device_pixel_ratio`.
- `WireClockPin`, `WireGeoPin`, `WireViewportPin`, `WireEnvironmentPin` — the pin report, with
  `unpinned` naming every knob that was asked for and refused.
- Three fields on `WireEnhancedAction`: `ax`, `viewport`, `environment`.

`types.EnhancedAction` reuses those structs rather than redeclaring them. `actionstore.cloneAction`
deep-copies all three pointers: sharing them would let a later ingest rewrite evidence a caller
already holds, so a snapshot would disagree with itself.

## Emission

`internal/reproduction/reproduction_locators.go` owns:

- `FallbackOrder` — the single declaration of selector -> ax -> coordinate, rendered into both
  artifacts as `FallbackOrderLabel`.
- `BuildLocators(action) []Locator` — the ordered list, omitting strategies the recording never
  captured.
- `RenderLocatorHuman` / `RenderLocatorCode` — the two renderings the backends need. Code rendering
  keeps the prose alongside the expression, because the expression alone loses the AX ref and the
  viewport the point was measured in.
- `EnvironmentPinLines`, `SessionPin`, `WriteEnvironmentPin` — the pin report and the mid-session
  change detection.
- `LocatorCoverage` — per-strategy counts, surfaced in `Meta`.

Both backends call `WriteEnvironmentPin` for the header and `WriteFallbackLocators` per targeted
step. The primary locator is not repeated: it is already the executable step.

## Pinning

`src/background/dom/cdp/cdp-env-pin.ts` runs over the shared CDP lease. Each knob is a `PinStep` with
its own `try`/`catch`; a refusal appends to `unpinned` and emits a structured `debugLog`, and the
remaining steps still run.

`clock_epoch_ms` is divided by 1000 before it reaches `Emulation.setVirtualTimePolicy`, whose
`initialVirtualTime` is TimeSinceEpoch in seconds. Passing milliseconds would place the page roughly
55,000 years ahead and make every date on it read as invalid.

The seed is installed twice: `Page.addScriptToEvaluateOnNewDocument` for every document after this
one, and `Runtime.evaluate` for the document already open. A third `Runtime.evaluate` reads
`__KABOOM_RANDOM_SEED_ACTIVE__` back and throws when it is not `true`, so a page where early-patch
never ran reports the seed as unpinned.

`src/early-patch.ts` owns the generator: FNV-1a over `seed + ':' + lane` into four xorshift128 lanes,
with a zero lane replaced by the golden-ratio constant because a zero lane makes xorshift emit zeros
forever. `Math.random` and `crypto.getRandomValues` are replaced through the existing
`safeAssignGlobal`, and `__KABOOM_RANDOM_SEED_ACTIVE__` records whether both replacements landed.

## MCP surface

`interact(what:'pin_environment', environment:{...})` and `interact(what:'unpin_environment')`.
One object property rather than nine flat ones: the interact schema shares a single property map
across every action, and flat `width`/`height`/`latitude` names would collide with targeting and
geometry parameters. CLI flag: `--environment` (`FlagJSON`).

`parseEnvironmentPinSpec` drops values of the wrong type rather than coercing them: a
`viewport_width` of `"wide"` coerced to `NaN` would be sent to CDP, refused, and reported as a knob
the browser would not pin — blaming the browser for a caller's typo.

## Budgets

- `internal/reproduction`: 8 files before, 10 after (at the folder limit, not over).
- `src/background/dom/cdp`: 7 -> 8. `src/background/environment-transaction`: 5 -> 6.
- `internal/types` stays at 10 files: the new wire structs extend `wire_enhanced_action.go`.
- `early-patch.bundled.js`: 3.8 KB -> 4.6 KB against a 250 KB per-file cap.
