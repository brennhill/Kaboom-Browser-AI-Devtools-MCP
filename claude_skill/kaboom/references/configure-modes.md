# Configure Tool — Mode Reference

29 modes. Universal params on every call: `what` (required), `telemetry_mode` (off|auto|full), `tab_id` (number).

---

## store
Persist and retrieve session data.
**Params:** store_action (save|load|list|delete|stats), namespace (string, default: session), key (string), data (object), value (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"store","store_action":"save","namespace":"session","key":"my_key","data":"my_value"}'
```

## load
Load persisted data.
**Params:** namespace (string), key (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"load","namespace":"session","key":"my_key"}'
```

## clear
Clear buffers or session data.
**Params:** buffer (network|websocket|actions|logs|inbox|all)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"clear","buffer":"all"}'
```

## health
System health status.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"health"}'
```

## tutorial
Interactive tutorial.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"tutorial"}'
```

## examples
Usage examples.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"examples"}'
```

## noise_rule
Manage event noise filtering.
**Params:** noise_action (add|remove|list|reset|auto_detect), rules (array), classification (string), message_regex (string), source_regex (string), url_regex (string), status_min (int), status_max (int), level (string), rule_id (string), pattern (string), category (console|network|websocket, default: console), reason (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"noise_rule","noise_action":"add","category":"console","message_regex":".*favicon.*","reason":"ignore favicon noise"}'
```

## streaming
Push notification config.
**Params:** streaming_action (enable|disable|status), events (array: errors|network_errors|performance|user_frustration|security|regression|anomaly|ci|all), throttle_seconds (int, 1-60), severity_min (info|warning|error)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"streaming","streaming_action":"enable","events":["errors","network_errors"],"throttle_seconds":5}'
```

## test_boundary_start
Mark test start boundary.
**Params:** test_id (string), label (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"test_boundary_start","test_id":"t1","label":"login flow test"}'
```

## test_boundary_end
Mark test end boundary.
**Params:** test_id (string), label (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"test_boundary_end","test_id":"t1","label":"login flow test"}'
```

## event_recording_start
Begin event recording session.
**Params:** name (string), sensitive_data_enabled (bool)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"event_recording_start","name":"checkout_flow","sensitive_data_enabled":false}'
```

## event_recording_stop
End event recording session.
**Params:** recording_id (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"event_recording_stop","recording_id":"rec_abc123"}'
```

## playback
Replay recorded events.
**Params:** recording_id (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"playback","recording_id":"rec_abc123"}'
```

## log_diff
Compare logs between recordings.
**Params:** original_id (string), replay_id (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"log_diff","original_id":"rec_abc123","replay_id":"rec_def456"}'
```

## telemetry
Configure telemetry metadata.
**Params:** telemetry_mode (off|auto|full)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"telemetry","telemetry_mode":"auto"}'
```

## describe_capabilities
List available modes and parameters.
**Params:** tool (string, optional filter), mode (string, optional filter)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"describe_capabilities","tool":"configure"}'
```

## diff_sessions
Capture and compare session snapshots.
**Params:** verif_session_action (capture|compare|list|delete), name (string), compare_a (string), compare_b (string), url (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"diff_sessions","verif_session_action":"capture","name":"before_deploy","url":"https://example.com"}'
```

## audit_log
Analyze tool call history.
**Params:** operation (analyze|report|clear), audit_session_id (string), tool_name (string), since (ISO 8601), limit (number, default 100, max 1000)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"audit_log","operation":"report","limit":50}'
```

## restart
Restart server/extension.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"restart"}'
```

## save_sequence
Save interaction sequence.
**Params:** name (string), steps (array), description (string), tags (array)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"save_sequence","name":"login_flow","steps":[{"tool":"interact","args":{"what":"click","selector":"#login"}}],"description":"Automated login","tags":["auth"]}'
```

## get_sequence
Retrieve saved sequence.
**Params:** name (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"get_sequence","name":"login_flow"}'
```

## list_sequences
List all sequences.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"list_sequences"}'
```

## delete_sequence
Delete saved sequence.
**Params:** name (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"delete_sequence","name":"login_flow"}'
```

## replay_sequence
Replay interaction sequence.
**Params:** name (string), override_steps (array), step_timeout_ms (number, default 10000), continue_on_error (bool, default true), stop_after_step (number)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"replay_sequence","name":"login_flow","step_timeout_ms":15000,"continue_on_error":true}'
```

## doctor
Diagnostics and troubleshooting.
**Params:** none
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"doctor"}'
```

## security_mode
Toggle security proxy mode.
**Params:** mode (normal|insecure_proxy), confirm (bool, required true for insecure_proxy)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"security_mode","mode":"insecure_proxy","confirm":true}'
```

## network_recording
Record HTTP/WebSocket traffic.
**Params:** operation (start|stop|status), method (string), domain (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"network_recording","operation":"start","domain":"api.example.com"}'
```

## action_jitter
Add random delays to actions.
**Params:** action_jitter_ms (number, 0 to disable)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"action_jitter","action_jitter_ms":200}'
```

## report_issue
Create and submit issue reports.
**Params:** operation (list_templates|preview|submit), template (string), title (string), user_context (string)
**Example:**
```bash
bash scripts/kaboom-call.sh configure '{"what":"report_issue","operation":"preview","template":"bug","title":"Click fails on modal","user_context":"Happens after popup opens"}'
```
