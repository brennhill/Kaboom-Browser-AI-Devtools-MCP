// doc.go — Package pagestorage: the live-page storage and IndexedDB use case.
// Why: observe(what="storage"|"indexeddb") round-trip to the tracked tab; this package owns that workflow
// (tracking gate, extension queries, section filtering/summaries, IndexedDB script execution) behind
// narrow ports so MCP parsing and response assembly stay in the observe adapter. IndexedDB reads run
// generated JavaScript, so script-literal escaping lives here too (scripts.go) and stays reviewable.
// Docs: docs/features/feature/observe/index.md

/*
Package pagestorage reads localStorage, sessionStorage, cookies and IndexedDB
from the tracked tab through the browser-command owner (queries.QueryRunner).

It must not import internal/mcp, internal/capture/**, internal/tools/**,
net/http or cmd/** (enforced by scripts/contracts/layering as internal/app/**).
Failures are typed *Failure kinds; adapters map them to protocol errors.
*/
package pagestorage
