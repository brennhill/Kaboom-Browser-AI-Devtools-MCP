/**
 * Purpose: Scans page forms to extract field metadata (name, type, required, validation) and optionally runs client-side validation.
 * Docs: docs/features/feature/form-filling/index.md
 */

// form-discovery.ts — Form discovery and validation handler for inject context.
// Scans forms on the page, extracts field metadata, and optionally validates.

interface FormDiscoveryParams {
  selector?: string
  mode?: 'discover' | 'validate'
}

interface FormFieldInfo {
  name: string
  type: string
  required: boolean
  value: string
  label: string
  selector: string
  tag: string
  validation_constraints: Record<string, string | number | boolean>
  options?: Array<{ value: string; text: string; selected: boolean }>
  validation_message?: string
}

interface FormInfo {
  action: string
  method: string
  selector: string
  id: string
  name: string
  fields: FormFieldInfo[]
  submit_button: { selector: string; text: string } | null
  valid?: boolean
  validation_errors?: Array<{ field: string; message: string }>
}

/**
 * Find the label text for a form element.
 */
function findLabel(el: HTMLElement): string {
  // Check for explicit label via for attribute
  const id = el.getAttribute('id')
  if (id) {
    const label = document.querySelector(`label[for="${CSS.escape(id)}"]`)
    if (label) return (label.textContent || '').trim().slice(0, 100)
  }

  // Check for wrapping label
  const parentLabel = el.closest('label')
  if (parentLabel) {
    return (parentLabel.textContent || '').trim().slice(0, 100)
  }

  // Check aria-label
  const ariaLabel = el.getAttribute('aria-label')
  if (ariaLabel) return ariaLabel.trim()

  // Check placeholder
  const placeholder = el.getAttribute('placeholder')
  if (placeholder) return placeholder.trim()

  return ''
}

/**
 * Build a minimal CSS selector for a form element.
 */
function buildFieldSelector(el: Element): string {
  if (el.id) return `#${el.id}`
  const name = el.getAttribute('name')
  if (name) return `${el.tagName.toLowerCase()}[name="${name}"]`
  const tag = el.tagName.toLowerCase()
  const classes = Array.from(el.classList)
    .slice(0, 2)
    .map((c) => `.${c}`)
    .join('')
  return tag + classes
}

/**
 * Build a minimal CSS selector for a form element.
 */
function buildFormSelector(form: HTMLFormElement): string {
  if (form.id) return `form#${form.id}`
  const name = form.getAttribute('name')
  if (name) return `form[name="${name}"]`
  const action = form.getAttribute('action')
  if (action) return `form[action="${action}"]`
  return 'form'
}

/**
 * Extract validation constraints from a form element.
 */
function getValidationConstraints(
  el: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
): Record<string, string | number | boolean> {
  const constraints: Record<string, string | number | boolean> = {}
  if (el.required) constraints.required = true

  const input = el as HTMLInputElement
  if (input.minLength > 0) constraints.min_length = input.minLength
  if (input.maxLength > 0 && input.maxLength < 524288) constraints.max_length = input.maxLength
  if (input.min) constraints.min = input.min
  if (input.max) constraints.max = input.max
  if (input.pattern) constraints.pattern = input.pattern
  if (input.step) constraints.step = input.step

  return constraints
}

function collectFormField(
  field: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement,
  mode: 'discover' | 'validate' | undefined
): FormFieldInfo {
  const fieldType = field.getAttribute('type') || field.tagName.toLowerCase()

  const fieldInfo: FormFieldInfo = {
    name: field.name || '',
    type: fieldType,
    required: field.required,
    value: field.value || '',
    label: findLabel(field),
    selector: buildFieldSelector(field),
    tag: field.tagName.toLowerCase(),
    validation_constraints: getValidationConstraints(field)
  }

  // Add options for select elements
  if (field.tagName === 'SELECT') {
    const select = field as HTMLSelectElement
    fieldInfo.options = Array.from(select.options).map((opt) => ({
      value: opt.value,
      text: opt.text,
      selected: opt.selected
    }))
  }

  // Add validation message in validate mode
  if (mode === 'validate') {
    field.checkValidity()
    if (field.validationMessage) {
      fieldInfo.validation_message = field.validationMessage
    }
  }

  return fieldInfo
}

function collectFormFields(
  form: HTMLFormElement,
  mode: 'discover' | 'validate' | undefined,
  maxFields: number
): FormFieldInfo[] {
  const fieldElements = form.querySelectorAll('input, select, textarea')
  const fields: FormFieldInfo[] = []

  for (let j = 0; j < fieldElements.length && fields.length < maxFields; j++) {
    const field = fieldElements[j]! as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
    // Skip hidden inputs
    if ((field.getAttribute('type') || field.tagName.toLowerCase()) === 'hidden') continue
    fields.push(collectFormField(field, mode))
  }

  return fields
}

function findSubmitButton(form: HTMLFormElement): { selector: string; text: string } | null {
  const submitEl =
    form.querySelector('button[type="submit"], input[type="submit"]') ||
    form.querySelector('button:not([type]), button[type="button"]')
  if (!submitEl) return null
  return {
    selector: buildFieldSelector(submitEl),
    text: (submitEl.textContent || (submitEl as HTMLInputElement).value || '').trim().slice(0, 100)
  }
}

function buildFormInfo(
  form: HTMLFormElement,
  fields: FormFieldInfo[],
  submitButton: { selector: string; text: string } | null
): FormInfo {
  return {
    action: form.action || '',
    method: (form.method || 'GET').toUpperCase(),
    selector: buildFormSelector(form),
    id: form.id || '',
    name: form.name || '',
    fields,
    submit_button: submitButton
  }
}

function applyFormValidation(form: HTMLFormElement, formInfo: FormInfo, fields: FormFieldInfo[]): void {
  formInfo.valid = form.checkValidity()
  if (!formInfo.valid) {
    formInfo.validation_errors = fields
      .filter((f) => f.validation_message)
      .map((f) => ({
        field: f.name || f.selector,
        message: f.validation_message!
      }))
  }
}

/**
 * Discover forms on the page.
 */
export function discoverForms(params: FormDiscoveryParams): FormInfo[] {
  const formSelector = params.selector || 'form'
  const forms = document.querySelectorAll(formSelector)
  const results: FormInfo[] = []
  const MAX_FORMS = 20
  const MAX_FIELDS = 50

  for (let i = 0; i < forms.length && results.length < MAX_FORMS; i++) {
    const form = forms[i]! as HTMLFormElement

    // Skip if the element isn't actually a form when using a generic selector
    if (form.tagName !== 'FORM') continue

    const fields = collectFormFields(form, params.mode, MAX_FIELDS)
    const submitButton = findSubmitButton(form)
    const formInfo = buildFormInfo(form, fields, submitButton)

    // Add validation results in validate mode
    if (params.mode === 'validate') {
      applyFormValidation(form, formInfo, fields)
    }

    results.push(formInfo)
  }

  return results
}
