// demos.go — Stores lightweight demo scripts exposed via kaboom://demo/* resources.
// Why: Keeps demo payloads modular and separate from primary capability playbooks.

package resources

func demoScripts() map[string]string {
	return map[string]string{
		"ws": `# Demo: WebSocket Debugging

Goal: show mismatched message format and where to fix it.

Steps:
1. {"tool":"observe","arguments":{"what":"websocket_status"}}
2. {"tool":"observe","arguments":{"what":"websocket_events","limit":20}}
3. {"tool":"analyze","arguments":{"what":"api_validation","operation":"analyze","ignore_endpoints":["/socket"]}}

Expected:
- Connection OK, but message schema warnings
- Identify client-side parsing path for fix
`,
		"annotations": `# Demo: Usability Annotations

Goal: highlight a layout issue and collect feedback.

Steps:
1. {"tool":"interact","arguments":{"what":"draw_mode_start","annot_session":"demo-ux"}}
2. Ask user to annotate oversized image and desired size.
3. {"tool":"analyze","arguments":{"what":"annotations","annot_session":"demo-ux","wait":true}}

Expected:
- Annotation list with coordinates and notes
`,
		"recording": `# Demo: Flow Recording

Goal: show record → action → stop workflow.

Steps:
1. {"tool":"configure","arguments":{"what":"event_recording_start","name":"demo-flow"}}
2. {"tool":"interact","arguments":{"what":"navigate","url":"http://localhost:xxxx"}}
3. {"tool":"configure","arguments":{"what":"event_recording_stop","recording_id":"..."}}

Expected:
- Saved recording ID and playback instructions
`,
		"dependencies": `# Demo: Dependency Vetting

Goal: identify unexpected third-party origins.

Steps:
1. {"tool":"analyze","arguments":{"what":"third_party_audit","first_party_origins":["http://localhost:xxxx"]}}
2. {"tool":"observe","arguments":{"what":"network_waterfall","limit":50}}

Expected:
- Highlight unexpected origins for review
`,
	}
}
