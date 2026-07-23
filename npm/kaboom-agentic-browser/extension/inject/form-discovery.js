/**
 * Purpose: Scans page forms to extract field metadata (name, type, required, validation) and optionally runs client-side validation.
 * Docs: docs/features/feature/form-filling/index.md
 */
/**
 * Find the label text for a form element.
 */
function findLabel(el) {
    // Check for explicit label via for attribute
    const id = el.getAttribute('id');
    if (id) {
        const label = document.querySelector(`label[for="${CSS.escape(id)}"]`);
        if (label)
            return (label.textContent || '').trim().slice(0, 100);
    }
    // Check for wrapping label
    const parentLabel = el.closest('label');
    if (parentLabel) {
        return (parentLabel.textContent || '').trim().slice(0, 100);
    }
    // Check aria-label
    const ariaLabel = el.getAttribute('aria-label');
    if (ariaLabel)
        return ariaLabel.trim();
    // Check placeholder
    const placeholder = el.getAttribute('placeholder');
    if (placeholder)
        return placeholder.trim();
    return '';
}
/**
 * Build a minimal CSS selector for a form element.
 */
function buildFieldSelector(el) {
    if (el.id)
        return `#${el.id}`;
    const name = el.getAttribute('name');
    if (name)
        return `${el.tagName.toLowerCase()}[name="${name}"]`;
    const tag = el.tagName.toLowerCase();
    const classes = Array.from(el.classList)
        .slice(0, 2)
        .map((c) => `.${c}`)
        .join('');
    return tag + classes;
}
/**
 * Build a minimal CSS selector for a form element.
 */
function buildFormSelector(form) {
    if (form.id)
        return `form#${form.id}`;
    const name = form.getAttribute('name');
    if (name)
        return `form[name="${name}"]`;
    const action = form.getAttribute('action');
    if (action)
        return `form[action="${action}"]`;
    return 'form';
}
/**
 * Extract validation constraints from a form element.
 */
function getValidationConstraints(el) {
    const constraints = {};
    if (el.required)
        constraints.required = true;
    const input = el;
    if (input.minLength > 0)
        constraints.min_length = input.minLength;
    if (input.maxLength > 0 && input.maxLength < 524288)
        constraints.max_length = input.maxLength;
    if (input.min)
        constraints.min = input.min;
    if (input.max)
        constraints.max = input.max;
    if (input.pattern)
        constraints.pattern = input.pattern;
    if (input.step)
        constraints.step = input.step;
    return constraints;
}
/**
 * Discover forms on the page.
 */
export function discoverForms(params) {
    const formSelector = params.selector || 'form';
    const forms = document.querySelectorAll(formSelector);
    const results = [];
    const MAX_FORMS = 20;
    const MAX_FIELDS = 50;
    for (let i = 0; i < forms.length && results.length < MAX_FORMS; i++) {
        const form = forms[i];
        // Skip if the element isn't actually a form when using a generic selector
        if (form.tagName !== 'FORM')
            continue;
        const fieldElements = form.querySelectorAll('input, select, textarea');
        const fields = [];
        for (let j = 0; j < fieldElements.length && fields.length < MAX_FIELDS; j++) {
            const field = fieldElements[j];
            const fieldType = field.getAttribute('type') || field.tagName.toLowerCase();
            // Skip hidden inputs
            if (fieldType === 'hidden')
                continue;
            const fieldInfo = {
                name: field.name || '',
                type: fieldType,
                required: field.required,
                value: field.value || '',
                label: findLabel(field),
                selector: buildFieldSelector(field),
                tag: field.tagName.toLowerCase(),
                validation_constraints: getValidationConstraints(field)
            };
            // Add options for select elements
            if (field.tagName === 'SELECT') {
                const select = field;
                fieldInfo.options = Array.from(select.options).map((opt) => ({
                    value: opt.value,
                    text: opt.text,
                    selected: opt.selected
                }));
            }
            // Add validation message in validate mode
            if (params.mode === 'validate') {
                field.checkValidity();
                if (field.validationMessage) {
                    fieldInfo.validation_message = field.validationMessage;
                }
            }
            fields.push(fieldInfo);
        }
        // Find submit button
        let submitButton = null;
        const submitEl = form.querySelector('button[type="submit"], input[type="submit"]') ||
            form.querySelector('button:not([type]), button[type="button"]');
        if (submitEl) {
            submitButton = {
                selector: buildFieldSelector(submitEl),
                text: (submitEl.textContent || submitEl.value || '').trim().slice(0, 100)
            };
        }
        const formInfo = {
            action: form.action || '',
            method: (form.method || 'GET').toUpperCase(),
            selector: buildFormSelector(form),
            id: form.id || '',
            name: form.name || '',
            fields,
            submit_button: submitButton
        };
        // Add validation results in validate mode
        if (params.mode === 'validate') {
            formInfo.valid = form.checkValidity();
            if (!formInfo.valid) {
                formInfo.validation_errors = fields
                    .filter((f) => f.validation_message)
                    .map((f) => ({
                    field: f.name || f.selector,
                    message: f.validation_message
                }));
            }
        }
        results.push(formInfo);
    }
    return results;
}
//# sourceMappingURL=form-discovery.js.map