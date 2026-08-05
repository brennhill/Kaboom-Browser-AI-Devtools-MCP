// automation.go — Automation playbook content.
// Why: Keeps capability-specific playbooks modular and easier to update.

package resources

func playbookSetAutomation() map[string]string {
	return map[string]string{
		"automation/quick": `# Playbook: Browser Automation (Quick)

Use when you need to interact with any web page: navigate, fill forms, click buttons, post content, or complete multi-step workflows.

## Preconditions

- Extension connected and tracked tab confirmed.

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
3. {"tool":"observe","arguments":{"what":"screenshot"}}
4. {"tool":"interact","arguments":{"what":"click","selector":"<button-or-link>"}}
5. {"tool":"interact","arguments":{"what":"type","selector":"<input-or-textarea>","text":"<content>"}}
6. {"tool":"observe","arguments":{"what":"screenshot"}}

## Tips

- Always take a screenshot after navigation to understand the page layout.
- Take a screenshot before irreversible actions (submit, post, delete) to verify state.
- Use text=<visible text> selectors when CSS selectors are unknown.
- Use interact(what:"list_interactive") to discover clickable elements on the page.
- For rich text editors, type will handle content insertion automatically.
`,
		"automation/full": `# Playbook: Browser Automation (Full)

Use for complex multi-step browser workflows: form filling, multi-page navigation, content posting, or any task requiring sequential browser interactions.

## Preconditions

- Extension connected
- Correct tracked tab

## Steps

1. Verify connection:
   {"tool":"configure","arguments":{"what":"health"}}
2. Navigate to target:
   {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
3. Screenshot to understand layout:
   {"tool":"observe","arguments":{"what":"screenshot"}}
4. Discover interactive elements (if selectors unknown):
   {"tool":"interact","arguments":{"what":"list_interactive","scope_selector":"<container>"}}
5. Perform actions (click, type, select, etc.):
   {"tool":"interact","arguments":{"what":"click","selector":"<element>"}}
   {"tool":"interact","arguments":{"what":"type","selector":"<input>","text":"<content>"}}
6. Verify result with screenshot:
   {"tool":"observe","arguments":{"what":"screenshot"}}
7. Continue or submit:
   {"tool":"interact","arguments":{"what":"click","selector":"<submit-button>"}}

## Example Workflows

### Fill and submit a form
  navigate → screenshot → type fields → click submit → screenshot

### Post content on a website
  navigate → click "new post" → type content → screenshot to verify → click post

### Multi-page checkout
  navigate → fill form → click next → fill form → screenshot → click submit

## Failure Modes

- element_not_found: use list_interactive to discover elements, retry with element_id
- ambiguous_target: narrow with scope_selector or scope_rect
- stale_element_id: refresh list_interactive, reacquire element_id
- blocked_by_overlay: run interact({what:"dismiss_top_overlay"}) then retry
- page_changed_unexpectedly: take screenshot, reassess

## Tips

- Screenshot before and after critical actions
- Use observe(what:"page") to confirm current URL
- interact navigate and refresh auto-include performance metrics
- For file uploads use interact(what:"upload")
`,
	}
}
