---
feature: drag-drop-automation
doc_type: qa-plan
feature_id: feature-drag-drop-automation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Drag & Drop Automation

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Event sequence generation
- [ ] Test HTML5 drag event sequence creation
- [ ] Test mouse event sequence creation
- [ ] Test DataTransfer object construction
- [ ] Test coordinate calculation for element centers

**Integration tests:** Full drag-drop flow
- [ ] Test sortable list item reorder
- [ ] Test Trello-style card move between columns
- [ ] Test file drop on upload zone
- [ ] Test drag in canvas/visual editor (coordinate-based)

**Edge case tests:** Error handling
- [ ] Test element not found (source or target)
- [ ] Test element not draggable (force drag anyway)
- [ ] Test drop rejected by handler
- [ ] Test timeout on slow drop handler

### Security/Compliance Testing

**Permission tests:** Verify only authorized access
- [ ] Test drag-drop requires AI Web Pilot toggle enabled

---

## Human UAT Walkthrough

### Scenario 1: Sortable List Reorder (Happy Path)
1. Setup:
   - Open page with sortable list (e.g., jQuery UI Sortable demo)
   - Enable AI Web Pilot toggle
2. Steps:
   - [ ] Observe DOM to identify list items
   - [ ] Call `interact({what: "drag_drop", source: "li:nth-child(1)", target: "li:nth-child(3)", position: "after"})`
   - [ ] Wait for async result
   - [ ] Verify first item moved to third position
3. Expected Result: List item reordered
4. Verification: Observe DOM to confirm new order

### Scenario 2: Trello Card Move Between Columns
1. Setup:
   - Open Trello board (or similar kanban)
   - Identify card in "To Do" column, target "Done" column
2. Steps:
   - [ ] Call `drag_drop` from card to Done column
   - [ ] Verify card appears in Done column
   - [ ] Check network for PATCH/PUT request updating card status
3. Expected Result: Card moved, backend updated
4. Verification: Network shows API call, card persists in new column on refresh

### Scenario 3: File Drop Simulation
1. Setup:
   - Open page with file upload drop zone
2. Steps:
   - [ ] Call `drag_drop` with `{target: "#dropzone", data_transfer: {files: [{name: "test.pdf", type: "application/pdf"}]}}`
   - [ ] Verify drop zone handler triggered
   - [ ] Check if file name appears in UI
3. Expected Result: Drop zone recognizes file drop
4. Verification: UI shows "test.pdf ready to upload"

### Scenario 4: Canvas Drag (Coordinate-Based)
1. Setup:
   - Open diagram editor or visual canvas
2. Steps:
   - [ ] Call `drag_drop` with `{source: "#shape-1", target_x: 200, target_y: 150}`
   - [ ] Verify shape moved to coordinates (200, 150)
3. Expected Result: Shape repositioned on canvas
4. Verification: Observe DOM or call execute_js to get element position

### Scenario 5: AI Web Pilot Toggle OFF (Error Path)
1. Setup:
   - Disable AI Web Pilot toggle
2. Steps:
   - [ ] Attempt drag_drop
3. Expected Result: Error "ai_web_pilot_disabled"
4. Verification: No elements moved

---

## Regression Testing

- Test existing interact actions still work
- Test execute_js doesn't conflict with drag-drop events
- Test form filling after drag-drop

---

## Performance/Load Testing

- Test drag-drop with 100 intermediate mousemove events (< 500ms)
- Test complex drop handler (should complete within the async timeout window)
