# Eval Rig & Hooks — Open Questions

Captured during implementation. Review and resolve before merging.

## Eval Rig

1. **Tier 2 codebase pinning**: The spec says pin chi@v5.1.0 and hono@v4.0.0. Should we vendor these into testdata or clone on-the-fly during CI? Vendoring is faster/deterministic but bloats the repo. Current decision: defer Tier 2 to a follow-up; Tier 1 (unit evals with synthetic fixtures) is built now.

2. **Tier 3 live metrics**: The spec describes writing to `~/.kaboom/sessions/<id>/metrics.jsonl`. Should this be opt-in via `.kaboom.json` (`"eval_metrics": true`) or always-on? Current decision: not implemented yet — needs the daemon integration for aggregation.

3. **Golden file format**: The spec shows `.golden` files alongside `.json` fixtures. Current implementation uses `expect` fields inside the JSON fixture itself (contains/not_contains/has_output). Golden files would be a separate comparison mode for exact output matching. Current decision: JSON-inline expectations are sufficient for now.

4. **Performance budget CI enforcement**: Spec says "CI fails if any hook exceeds its budget on 3 consecutive runs." How do we track consecutive runs across CI invocations? Current decision: enforce per-run with generous budgets; consecutive-run tracking deferred.

## Session Tracking

5. **Session cleanup timing**: Spec says clean sessions older than 4 hours. Is this aggressive enough? Long coding sessions could exceed 4 hours. Current decision: 8 hours, configurable later.

6. **Session ID for Codex**: Spec mentions `CODEX_SESSION_ID` env var but Codex's hook protocol is undocumented. Current decision: check for the env var but treat as best-effort.

## Blast Radius

7. **Import graph cache invalidation**: When should the cached graph be rebuilt? On every edit? On file creation/deletion only? Current decision: rebuild on edit if the edited file is not in the graph, otherwise reuse cache. Full rebuild if cache is older than 5 minutes.

8. **Cross-module imports in Go**: Should we resolve module paths (e.g., `github.com/foo/bar/pkg`) to local paths? Current decision: yes, using go.mod module path prefix stripping.

9. **Python import resolution**: Python imports are notoriously complex (relative imports, __init__.py, sys.path). How deep do we go? Current decision: handle `from X import Y` and `import X` with simple path mapping. Skip dynamic imports.

## Decision Guard

10. **Decision file location**: Spec says `.kaboom/decisions.json`. Should we also check project root `decisions.json`? Current decision: only `.kaboom/decisions.json` for now.

11. **Regex safety**: User-provided regex patterns in decisions.json could be pathological (ReDoS). Should we add a timeout or complexity limit? Current decision: use regexp.Compile (not MustCompile) and skip invalid patterns with a warning in output.

12. **Decision expiry**: Spec mentions optional expiry dates. How does the AI know a decision expired? Current decision: skip expired decisions silently during matching.

## Multi-Agent Protocol

13. **Gemini CLI testing**: We can't easily test Gemini CLI integration without Gemini CLI installed. Current decision: unit tests mock env vars; manual testing deferred.

14. **Codex TOML config**: Codex uses TOML for configuration, not JSON. The installer handles this but the configure tool doesn't write Codex config yet. Current decision: deferred to a follow-up.
