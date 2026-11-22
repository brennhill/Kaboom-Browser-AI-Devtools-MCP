// annotation_hints.go — Builds LLM guidance hints for annotation responses.
// Why: Isolates presentation/hint logic from annotation handler flow control.
// Docs: docs/features/feature/annotated-screenshots/index.md

package toolanalyze

// BuildSessionHints returns LLM guidance hints for annotation session responses.
func BuildSessionHints(screenshotPath string) map[string]any {
	hints := map[string]any{
		"checklist": []string{
			"Present annotations as a numbered checklist with suggested priority.",
			"For each annotation, call analyze({what:'annotation_detail', correlation_id:'...'}) for DOM/style context.",
			"If css_framework is detected, use framework-idiomatic code in fixes.",
			"Check correlated_errors — errors near the annotation timestamp may explain visual issues.",
			"After fixes, screenshot each page to compare against the baseline screenshot.",
		},
	}
	if screenshotPath != "" {
		hints["screenshot_baseline"] = "A pre-alteration screenshot was captured at " + screenshotPath + ". Compare after changes."
	}
	return hints
}

// BuildDetailHints returns context-aware LLM hints for annotation detail responses.
// Returns nil if no hints apply (no framework, no a11y flags, no correlated errors).
func BuildDetailHints(cssFramework string, jsFramework string, a11yFlags []string, hasCorrelatedErrors bool) map[string]any {
	hints := make(map[string]any)

	if hint, ok := cssFrameworkHint(cssFramework); ok {
		hints["design_system"] = hint
	}

	if hint, ok := jsFrameworkHint(jsFramework); ok {
		hints["runtime_framework"] = hint
	}

	if len(a11yFlags) > 0 {
		hints["accessibility"] = "Accessibility issues detected. Address a11y_flags before visual changes — screen reader compatibility and contrast ratios affect all users."
	}

	if hasCorrelatedErrors {
		hints["error_context"] = "Console errors occurred near this annotation's timestamp. The visual issue may be caused by a JavaScript error — check correlated_errors first."
	}

	if len(hints) == 0 {
		return nil
	}
	return hints
}

func cssFrameworkHint(framework string) (string, bool) {
	if framework == "" {
		return "", false
	}
	switch framework {
	case "tailwind":
		return "This element uses Tailwind CSS. Prefer utility classes (e.g., bg-blue-500, p-4, text-sm) over custom CSS.", true
	case "bootstrap":
		return "This element uses Bootstrap. Use Bootstrap component classes (e.g., btn-primary, form-control) and grid system.", true
	case "css-modules":
		return "This element uses CSS Modules. Styles are scoped — modify the corresponding .module.css file.", true
	case "styled-components":
		return "This element uses styled-components/Emotion. Modify the component's styled template literal.", true
	default:
		return "CSS framework detected: " + framework + ". Use framework-idiomatic patterns.", true
	}
}

func jsFrameworkHint(framework string) (string, bool) {
	if framework == "" {
		return "", false
	}
	switch framework {
	case "react":
		return "Runtime framework appears to be React. Prefer component-level fixes over direct DOM mutation.", true
	case "vue":
		return "Runtime framework appears to be Vue. Keep template bindings and reactive state in sync with style/layout changes.", true
	case "angular":
		return "Runtime framework appears to be Angular. Prefer template/component stylesheet updates instead of manual DOM patches.", true
	case "svelte":
		return "Runtime framework appears to be Svelte. Apply fixes in component markup/style so compiled DOM stays consistent.", true
	default:
		return "Runtime framework detected: " + framework + ". Prefer framework-native component changes.", true
	}
}
