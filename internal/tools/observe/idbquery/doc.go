// Purpose: Package idbquery — reads IndexedDB databases, object stores and rows out of the tracked tab.
// Why: This is the only part of observe that runs generated JavaScript in the page instead of reading a
// server-side buffer, so it carries a different dependency (the extension execute round-trip) and a
// different risk (script-literal escaping). Isolating it keeps that surface small and reviewable.
// Docs: docs/features/feature/observe/index.md

/*
Package idbquery queries IndexedDB in the tracked tab by dispatching a generated
script through the extension's execute channel and normalizing the reply.

Key functions:
  - Listing: enumerates databases and their object stores.
  - Entries: reads rows from one object store, up to a limit.

Both return the decoded result map exactly as the page produced it, with the
fields observe's response builders depend on ("databases", "entries", "count",
"database", "store") backfilled when the page omits them.
*/
package idbquery
