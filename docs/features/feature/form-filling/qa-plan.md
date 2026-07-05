---
feature: form-filling
doc_type: qa-plan
feature_id: feature-form-filling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Form Filling

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Field type detection and value setting
- [ ] Test text input field filling
- [ ] Test email input with validation
- [ ] Test checkbox checking/unchecking
- [ ] Test radio button selection
- [ ] Test select dropdown value change
- [ ] Test textarea filling
- [ ] Test number input with min/max validation
- [ ] Test date input
- [ ] Test multi-select handling

**Integration tests:** Full form filling flow
- [ ] Test simple login form (2 fields)
- [ ] Test multi-field registration form (8+ fields)
- [ ] Test form with conditional fields (field appears based on selection)
- [ ] Test form with HTML5 validation (required, pattern, email)
- [ ] Test form submission after filling

**Edge case tests:** Error handling
- [ ] Test selector not found (field doesn't exist)
- [ ] Test readonly field (should skip)
- [ ] Test disabled field (should skip)
- [ ] Test hidden field (should attempt fill but may fail)
- [ ] Test multiple elements match selector (should fill first)
- [ ] Test timeout on slow form (10s limit)

### Security/Compliance Testing

**Data leak tests:** Verify no sensitive data exposed
- [ ] Test password field values not logged to console
- [ ] Test form data not persisted in memory buffers

**Permission tests:** Verify only authorized access
- [ ] Test form filling fails when AI Web Pilot toggle is OFF
- [ ] Test cross-origin iframe field access properly blocked

---

## Human UAT Walkthrough

### Scenario 1: Simple Login Form (Happy Path)
1. Setup:
   - Open <https://example.com/login> (or local test page with login form)
   - Start Kaboom, enable AI Web Pilot toggle in extension
   - Observe DOM to identify form fields
2. Steps:
   - [ ] Call `interact({action: "fill_form", fields: [{selector: "#username", value: "testuser"}, {selector: "#password", value: "testpass123"}]})`
   - [ ] Wait for async completion (poll `observe({what: "command_result"})`)
   - [ ] Visually verify username and password fields are filled in browser
   - [ ] Call `interact({action: "execute_js", code: "document.querySelector('form').submit()"})` to submit
3. Expected Result: Form submits with correct credentials, login succeeds (or shows "invalid credentials" from backend)
4. Verification: Check network traffic via `observe({what: "network_waterfall"})` to confirm POST to /login endpoint

### Scenario 2: Multi-Step Registration Form with Validation
1. Setup:
   - Open <https://example.com/register> (or local test page with complex form)
   - Form has: first name, last name, email (validated), password (min length), confirm password, country (select), agree to terms (checkbox)
2. Steps:
   - [ ] Call `interact({action: "fill_form", fields: [{selector: "#firstName", value: "John"}, {selector: "#lastName", value: "Doe"}, {selector: "#email", value: "invalid-email"}, {selector: "#password", value: "abc"}, {selector: "#confirmPassword", value: "abc"}, {selector: "#country", value: "US"}, {selector: "#terms", value: true}]})`
   - [ ] Wait for result
   - [ ] Verify result includes validation errors for email (invalid format) and password (too short)
   - [ ] Retry with valid data: `fill_form` with `email: "john@example.com"` and `password: "SecurePass123"`
   - [ ] Verify all fields filled correctly
3. Expected Result: First attempt returns partial status with validation errors. Second attempt succeeds with all fields filled.
4. Verification: Inspect form in browser, confirm validation messages appeared/disappeared

### Scenario 3: Conditional Field Form
1. Setup:
   - Open form with conditional logic (e.g., "I have a referral code" checkbox reveals text input)
   - Default state: referral input is hidden
2. Steps:
   - [ ] Call `fill_form` with `[{selector: "#hasReferral", value: true}, {selector: "#referralCode", value: "REF123"}]`
   - [ ] Verify checkbox is checked first (triggering display of referral input)
   - [ ] Verify referral code is filled (field should now be visible)
3. Expected Result: Checkbox enables conditional field, then code is filled
4. Verification: Visually confirm referral input is visible and filled

### Scenario 4: AI Web Pilot Toggle OFF (Error Path)
1. Setup:
   - Open any form page
   - Disable AI Web Pilot toggle in extension settings
2. Steps:
   - [ ] Call `interact({action: "fill_form", fields: [{selector: "#username", value: "test"}]})`
   - [ ] Wait for async result
3. Expected Result: Returns `{error: "ai_web_pilot_disabled", message: "Interactive features require AI Web Pilot toggle to be enabled"}`
4. Verification: Confirm form is NOT filled, error message is clear

### Scenario 5: Readonly Field Handling
1. Setup:
   - Open form with readonly field (e.g., pre-filled email from profile)
2. Steps:
   - [ ] Call `fill_form` with `[{selector: "#email", value: "newemail@example.com"}, {selector: "#name", value: "John"}]`
   - [ ] Wait for result
3. Expected Result: Result shows `{selector: "#email", status: "skipped", reason: "readonly"}` and `{selector: "#name", status: "filled"}`
4. Verification: Confirm readonly field unchanged, other field filled

---

## Regression Testing

### What existing features might break?
- `interact({action: "execute_js"})` — ensure form filling doesn't interfere with general JS execution
- Async command architecture — verify correlation_id handling still works correctly
- AI Web Pilot toggle — confirm toggle still gates all interact actions

### How to verify:
- Run existing `interact` tool tests
- Test execute_js with simple alert() to confirm still works
- Test toggle OFF/ON states for all interact actions

---

## Performance/Load Testing

### Performance requirements:
- Simple form (5 fields): < 500ms
- Complex form (20 fields): < 2s (within decision point)
- Extremely large form (100 fields): < 10s (within total timeout)

### Load test:
- Fill form with 100 fields
- Measure time from fill_form call to result posted
- Verify partial results returned if timeout occurs
