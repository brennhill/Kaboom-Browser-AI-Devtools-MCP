// Purpose: Defines evidence mode types and command-level evidence state structures.
// Why: Separates evidence type definitions from behavior to keep evidence files focused.

package interacthandler

type evidenceMode string

const (
	evidenceModeOff        evidenceMode = "off"
	evidenceModeOnMutation evidenceMode = "on_mutation"
	evidenceModeAlways     evidenceMode = "always"
)

const (
	evidenceRetryEnv       = "KABOOM_EVIDENCE_RETRY_COUNT"
	evidenceMaxCapturesEnv = "KABOOM_EVIDENCE_MAX_CAPTURES_PER_COMMAND"
)

type evidenceShot struct {
	Path     string
	Filename string
	Error    string
	Attempts int
}

type commandEvidenceState struct {
	mode          evidenceMode
	action        string
	shouldCapture bool
	maxCaptures   int
	clientID      string
	skipped       string

	before evidenceShot
	after  evidenceShot

	finalized bool
	cached    map[string]any
}

// evidenceCaptureFn is declared in interact_evidence_capture.go
