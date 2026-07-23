// Purpose: Validates and marshals save_state/load_state parameters for browser state snapshot operations.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// State capture status values — always present in save_state response as "state_capture".
const (
	StateCaptureStatusCaptured              = "captured"
	StateCaptureStatusPilotDisabled         = "skipped_pilot_disabled"
	StateCaptureStatusExtensionDisconnected = "skipped_extension_disconnected"
	StateCaptureStatusTimeout               = "skipped_timeout"
	StateCaptureStatusError                 = "skipped_error"
)

// State restore status values — always present in load_state response as "state_restore".
const (
	StateRestoreStatusQueued          = "queued"
	StateRestoreStatusPilotDisabled   = "skipped_pilot_disabled"
	StateRestoreStatusExtensionDown   = "skipped_extension_disconnected"
	StateRestoreStatusNoData          = "skipped_no_state_data"
	StateRestoreStatusEnqueueRejected = "rejected_enqueue_failed"
)

// StateNamespace is the namespace key used for persisting saved states.
const StateNamespace = "saved_states"

// StateCaptureResult holds the outcome of a state capture attempt.
type StateCaptureResult struct {
	Status string         // one of StateCaptureStatus* constants
	Data   map[string]any // non-nil only when Status == "captured"
}

// StateDataFields lists the fields extracted from capture data into persisted state.
var StateDataFields = []string{"form_values", "scroll_position", "local_storage", "session_storage", "cookies"}

// StateCaptureScript is the JS executed in the browser to capture form values,
// scroll position, localStorage, sessionStorage, and cookies.
const StateCaptureScript = `(() => {
  const sensitiveKeyPattern = /(pass(word)?|token|secret|api[_-]?key|auth|cookie|session|bearer|credential|otp|ssn|credit|card|cvv|cvc)/i;
  const sensitiveAutocompletePattern = /(password|one-time-code|cc-|credit-card|csc|cvv)/i;
  const sensitiveValuePattern = /^(eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}|sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9_]{16,}|xox[baprs]-[A-Za-z0-9-]{10,})$/;

  function isSensitive(el, key, value) {
    const type = (el.type || '').toLowerCase();
    if (type === 'password' || type === 'hidden' || type === 'file') return true;

    const autocomplete = (el.getAttribute('autocomplete') || '').toLowerCase();
    if (sensitiveAutocompletePattern.test(autocomplete)) return true;

    const keyProbe = [key, el.name, el.id, el.getAttribute('aria-label'), el.getAttribute('placeholder')]
      .filter(Boolean)
      .join(' ');
    if (sensitiveKeyPattern.test(keyProbe)) return true;

    if (typeof value === 'string' && value.length >= 12 && sensitiveValuePattern.test(value.trim())) return true;
    return false;
  }

  const forms = {};
  document.querySelectorAll('input, textarea, select').forEach(el => {
    const key = el.id || el.name;
    if (!key) return;

    const type = (el.type || '').toLowerCase();
    const rawValue = (type === 'checkbox' || type === 'radio') ? !!el.checked : String(el.value ?? '');
    if (isSensitive(el, key, rawValue)) return;

    if (type === 'radio') {
      const groupName = el.name || key;
      if (el.checked) {
        forms['radio::' + groupName] = {
          kind: 'radio',
          name: groupName,
          value: String(el.value ?? '')
        };
      }
      return;
    }

    if (type === 'checkbox') {
      forms[key] = el.checked;
    } else {
      forms[key] = el.value;
    }
  });

  const ls = {};
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (!sensitiveKeyPattern.test(k)) {
        const v = localStorage.getItem(k);
        if (v !== null && !(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          ls[k] = v;
        }
      }
    }
  } catch(e) {}

  const ss = {};
  try {
    for (let i = 0; i < sessionStorage.length; i++) {
      const k = sessionStorage.key(i);
      if (!sensitiveKeyPattern.test(k)) {
        const v = sessionStorage.getItem(k);
        if (v !== null && !(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          ss[k] = v;
        }
      }
    }
  } catch(e) {}

  const cookies = {};
  try {
    document.cookie.split(';').forEach(c => {
      const [k, ...rest] = c.trim().split('=');
      if (k && !sensitiveKeyPattern.test(k)) {
        const v = rest.join('=');
        if (!(v.length >= 12 && sensitiveValuePattern.test(v.trim()))) {
          cookies[k] = v;
        }
      }
    });
  } catch(e) {}

  return {
    form_values: forms,
    scroll_position: { x: window.scrollX, y: window.scrollY },
    local_storage: ls,
    session_storage: ss,
    cookies: cookies
  };
})()`

// ParseCapturedStatePayload parses the raw JSON result from the state capture script.
// Returns the captured data map or an error if the payload is invalid or indicates failure.
func ParseCapturedStatePayload(raw json.RawMessage) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, errors.New("state_capture: received empty or null payload from browser. Ensure the page is loaded and accessible")
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}

	if successVal, hasSuccess := envelope["success"]; hasSuccess {
		success, _ := successVal.(bool)
		if !success {
			if msg, ok := envelope["message"].(string); ok && msg != "" {
				return nil, errors.New(msg)
			}
			if code, ok := envelope["error"].(string); ok && code != "" {
				return nil, errors.New(code)
			}
			return nil, errors.New("state_capture: execute_js failed with no error details. Check browser console for script errors")
		}

		if resultObj, ok := envelope["result"].(map[string]any); ok {
			return resultObj, nil
		}
		if _, ok := envelope["form_values"]; ok {
			return envelope, nil
		}
		return nil, errors.New("state_capture: execute_js succeeded but result payload is missing. The capture script may have returned an unexpected format")
	}

	// Direct result (no success envelope) — accept if any known field present
	for _, key := range []string{"form_values", "local_storage", "session_storage", "cookies"} {
		if _, ok := envelope[key]; ok {
			return envelope, nil
		}
	}
	return nil, errors.New("state_capture: payload missing expected fields (form_values, local_storage, session_storage, cookies). Verify the page has accessible state to capture")
}

// BuildStateRestoreScript builds a JS script that restores form values, scroll position,
// localStorage, sessionStorage, and cookies. All data is embedded as JSON literals.
func BuildStateRestoreScript(formValues, scrollPos, localStorage, sessionStorage, cookies map[string]any) string {
	if formValues == nil {
		formValues = map[string]any{}
	}
	if scrollPos == nil {
		scrollPos = map[string]any{}
	}
	if localStorage == nil {
		localStorage = map[string]any{}
	}
	if sessionStorage == nil {
		sessionStorage = map[string]any{}
	}
	if cookies == nil {
		cookies = map[string]any{}
	}
	formJSON, _ := json.Marshal(formValues)
	scrollJSON, _ := json.Marshal(scrollPos)
	lsJSON, _ := json.Marshal(localStorage)
	ssJSON, _ := json.Marshal(sessionStorage)
	cookiesJSON, _ := json.Marshal(cookies)

	return fmt.Sprintf(`(() => {
  const formValues = %s;
  const scrollPos = %s;
  const lsData = %s;
  const ssData = %s;
  const cookieData = %s;
  const escapeCSS = (value) => {
    if (typeof CSS !== 'undefined' && CSS && typeof CSS.escape === 'function') {
      return CSS.escape(String(value));
    }
    return String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  };

  Object.entries(formValues).forEach(([key, val]) => {
    if (val && typeof val === 'object' && val.kind === 'radio' && val.name) {
      const escapedName = escapeCSS(val.name);
      const escapedValue = escapeCSS(val.value ?? '');
      const radio = document.querySelector('input[type="radio"][name="' + escapedName + '"][value="' + escapedValue + '"]');
      if (radio) {
        radio.checked = true;
      }
      return;
    }

    const escapedKey = escapeCSS(key);
    const el = document.getElementById(key) || document.querySelector('[name="' + escapedKey + '"]');
    if (!el) return;
    if (el.type === 'checkbox' || el.type === 'radio') {
      el.checked = !!val;
    } else {
      el.value = String(val);
      el.dispatchEvent(new Event('input', {bubbles: true}));
    }
  });

  try { Object.entries(lsData).forEach(([k, v]) => { localStorage.setItem(k, v); }); } catch(e) {}
  try { Object.entries(ssData).forEach(([k, v]) => { sessionStorage.setItem(k, v); }); } catch(e) {}
  try { Object.entries(cookieData).forEach(([k, v]) => { document.cookie = k + '=' + v; }); } catch(e) {}

  if (scrollPos && scrollPos.x !== undefined) {
    window.scrollTo(scrollPos.x, scrollPos.y);
  }
  return {
    restored_forms: Object.keys(formValues).length,
    restored_local_storage: Object.keys(lsData).length,
    restored_session_storage: Object.keys(ssData).length,
    restored_cookies: Object.keys(cookieData).length
  };
})()`, string(formJSON), string(scrollJSON), string(lsJSON), string(ssJSON), string(cookiesJSON))
}
