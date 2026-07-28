---
feature: dialog-handling
doc_type: qa-plan
feature_id: feature-dialog-handling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Dialog Handling

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Dialog interception
- [ ] Test alert() interception and capture
- [ ] Test confirm() interception (accept/dismiss)
- [ ] Test prompt() interception with text input
- [ ] Test beforeunload interception
- [ ] Test dialog buffer storage and retrieval

**Integration tests:** Full dialog handling flow
- [ ] Test alert triggered by button click, agent acknowledges
- [ ] Test confirm dialog, agent accepts
- [ ] Test confirm dialog, agent dismisses
- [ ] Test prompt dialog, agent provides input
- [ ] Test multiple dialogs queued, agent handles sequentially

**Edge case tests:** Error handling
- [ ] Test dialog shown before extension loads (should show native)
- [ ] Test rapid succession dialogs (queue correctly)
- [ ] Test timeout on unhandled dialog (auto-dismiss after 10s)
- [ ] Test AI Web Pilot toggle OFF (dialog handling fails)

### Security/Compliance Testing

**Data leak tests:** Verify no sensitive data exposed
- [ ] Test dialog messages with sensitive data (passwords) are redacted

**Permission tests:** Verify only authorized access
- [ ] Test dialog handling requires AI Web Pilot toggle enabled

---

## Human UAT Walkthrough

### Scenario 1: Alert Dialog (Happy Path)
1. Setup:
   - Open test page with button that triggers `alert("Test alert")`
   - Enable AI Web Pilot toggle
2. Steps:
   - [ ] Click button to trigger alert
   - [ ] Call `observe({what: "dialogs"})`
   - [ ] Verify dialog captured with type="alert", message="Test alert"
   - [ ] Call `interact({what: "handle_dialog", response: "accept"})`
   - [ ] Verify alert dismissed, workflow continues
3. Expected Result: Alert is captured and dismissed programmatically
4. Verification: No alert visible in browser after handling

### Scenario 2: Confirm Dialog (Accept)
1. Setup:
   - Test page with confirm dialog: `if (confirm("Delete?")) { deleteItem(); }`
2. Steps:
   - [ ] Trigger confirm dialog
   - [ ] Observe dialog
   - [ ] Handle with `{response: "accept"}`
   - [ ] Verify deleteItem() was called (check network for DELETE request)
3. Expected Result: Confirm accepted, deletion proceeds
4. Verification: Network shows DELETE request

### Scenario 3: Confirm Dialog (Dismiss)
1. Setup:
   - Same test page with confirm dialog
2. Steps:
   - [ ] Trigger confirm dialog
   - [ ] Handle with `{response: "dismiss"}`
   - [ ] Verify deleteItem() was NOT called
3. Expected Result: Confirm dismissed, no deletion
4. Verification: No DELETE request in network

### Scenario 4: Prompt Dialog with Input
1. Setup:
   - Test page: `var name = prompt("Enter name:", "Guest"); console.log("Name: " + name);`
2. Steps:
   - [ ] Trigger prompt
   - [ ] Observe dialog (shows default_value="Guest")
   - [ ] Handle with `{response: "accept", text: "John Doe"}`
   - [ ] Verify console log shows "Name: John Doe"
3. Expected Result: Prompt accepted with custom input
4. Verification: Console log via observe({what: "logs"}) shows correct name

### Scenario 5: Beforeunload Dialog
1. Setup:
   - Test page with: `window.addEventListener('beforeunload', (e) => { e.preventDefault(); e.returnValue = ''; });`
2. Steps:
   - [ ] Navigate away from page (e.g., `interact({what: "navigate", url: "https://example.com"})`)
   - [ ] Observe beforeunload dialog captured
   - [ ] Handle with `{response: "accept"}` to allow navigation
   - [ ] Verify navigation completes
3. Expected Result: Navigation proceeds after handling beforeunload
4. Verification: New page loaded

---

## Regression Testing

- Test existing interact actions still work
- Test observe tool still captures logs/network correctly
- Test AI Web Pilot toggle gates all interact actions

---

## Performance/Load Testing

- Test 10 rapid dialogs queued correctly
- Measure overhead of dialog interception (<0.01ms per call)
