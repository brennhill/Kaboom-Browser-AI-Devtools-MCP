// registry.go — Observe mode metadata (server-side modes, value aliases).
// Why: Keeps mode definitions discoverable in one place and exportable for the dispatch wiring.

package toolobserve

// ServerSideObserveModes lists modes that don't depend on live extension data.
var ServerSideObserveModes = map[string]bool{
	"command_result":    true,
	"pending_commands":  true,
	"failed_commands":   true,
	"saved_videos":      true,
	"recordings":        true,
	"recording_actions": true,
	"playback_results":  true,
	"log_diff_report":   true,
	"pilot":             true,
	"history":           true,
	"inbox":             true,
	"annotations":       true,
	"annotation_detail": true,
	"draw_history":      true,
	"draw_session":      true,
}
