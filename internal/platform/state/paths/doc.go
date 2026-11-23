// Purpose: Package state — resolves runtime filesystem paths for state, logs, PID files, and recordings.
// Why: Ensures all runtime artifacts use a consistent, configurable directory policy.
// Docs: docs/features/feature/project-isolation/index.md

/*
Package state resolves filesystem paths for Kaboom runtime artifacts.

Resolution order for the root directory:
 1. KABOOM_STATE_DIR environment variable (if set).
 2. XDG_STATE_HOME/kaboom (if XDG_STATE_HOME is set).
 3. ~/.kaboom (cross-platform dotfolder fallback).

Key functions:
  - RootDir: returns the runtime state root directory.
  - LogDir: returns the directory for log files.
  - PIDFile: returns the path to the daemon PID file.
  - RecordingDir: returns the directory for recording storage.
*/
package state
