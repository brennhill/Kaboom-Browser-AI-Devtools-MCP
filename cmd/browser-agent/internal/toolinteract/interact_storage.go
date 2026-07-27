// Purpose: Handles granular localStorage, sessionStorage, and cookie mutation actions (set, delete, clear) via extension queries.
// Why: Enables agents to manipulate browser storage state without injecting arbitrary JavaScript.
// Docs: docs/features/feature/environment-manipulation/index.md
package toolinteract

import (
	"encoding/json"
	"fmt"
)

var validStorageTypes = map[string]string{
	"localStorage":   "localStorage",
	"sessionStorage": "sessionStorage",
}

// validateStorageType checks that storage_type is one of the valid storage types.
// Returns the JS expression (e.g. "localStorage") and true on success, or an error response and false on failure.
func validateStorageType(req JSONRPCRequest, storageType string) (string, JSONRPCResponse, bool) {
	storageExpr, ok := validStorageTypes[storageType]
	if !ok {
		return "", fail(req, ErrInvalidParam, "Invalid 'storage_type' value: "+storageType, "Use 'localStorage' or 'sessionStorage'", withParam("storage_type")), false
	}
	return storageExpr, JSONRPCResponse{}, true
}

func (h *InteractActionHandler) HandleSetStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string  `json:"storage_type"`
		Key         string  `json:"key"`
		Value       *string `json:"value"`
		TabID       int     `json:"tab_id,omitempty"`
		TimeoutMs   int     `json:"timeout_ms,omitempty"`
		World       string  `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateStorageType(req, params.StorageType)
	if !ok {
		return errResp
	}
	if resp, blocked := requireString(req, params.Key, "key", "Add the 'key' parameter and call again"); blocked {
		return resp
	}
	if params.Value == nil {
		return fail(req, ErrMissingParam, "Required parameter 'value' is missing for set_storage action", "Add the 'value' parameter and call again", withParam("value"))
	}

	script := fmt.Sprintf(`(() => { try { %s.setItem(%s, %s); return { ok: true, action: "set_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(*params.Value), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_set", params.TabID, params.TimeoutMs, params.World, script, "set_storage", "set_storage queued")
}

func (h *InteractActionHandler) HandleDeleteStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string `json:"storage_type"`
		Key         string `json:"key"`
		TabID       int    `json:"tab_id,omitempty"`
		TimeoutMs   int    `json:"timeout_ms,omitempty"`
		World       string `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateStorageType(req, params.StorageType)
	if !ok {
		return errResp
	}
	if resp, blocked := requireString(req, params.Key, "key", "Add the 'key' parameter and call again"); blocked {
		return resp
	}

	script := fmt.Sprintf(`(() => { try { %s.removeItem(%s); return { ok: true, action: "delete_storage", storage_type: %s, key: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.Key), jsQuote(params.StorageType), jsQuote(params.Key))
	return h.queueExecuteScript(req, args, "storage_del", params.TabID, params.TimeoutMs, params.World, script, "delete_storage", "delete_storage queued")
}

func (h *InteractActionHandler) HandleClearStorage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		StorageType string `json:"storage_type"`
		TabID       int    `json:"tab_id,omitempty"`
		TimeoutMs   int    `json:"timeout_ms,omitempty"`
		World       string `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	storageExpr, errResp, ok := validateStorageType(req, params.StorageType)
	if !ok {
		return errResp
	}

	script := fmt.Sprintf(`(() => { try { %s.clear(); return { ok: true, action: "clear_storage", storage_type: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		storageExpr, jsQuote(params.StorageType))
	return h.queueExecuteScript(req, args, "storage_clear", params.TabID, params.TimeoutMs, params.World, script, "clear_storage", "clear_storage queued")
}

func (h *InteractActionHandler) HandleSetCookie(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name      string  `json:"name"`
		Value     *string `json:"value"`
		Domain    string  `json:"domain,omitempty"`
		Path      string  `json:"path,omitempty"`
		TabID     int     `json:"tab_id,omitempty"`
		TimeoutMs int     `json:"timeout_ms,omitempty"`
		World     string  `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.Name, "name", "Add the 'name' parameter and call again"); blocked {
		return resp
	}
	if params.Value == nil {
		return fail(req, ErrMissingParam, "Required parameter 'value' is missing for set_cookie action", "Add the 'value' parameter and call again", withParam("value"))
	}

	cookie := params.Name + "=" + *params.Value
	if params.Path != "" {
		cookie += "; path=" + params.Path
	} else {
		cookie += "; path=/"
	}
	if params.Domain != "" {
		cookie += "; domain=" + params.Domain
	}

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "set_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_set", params.TabID, params.TimeoutMs, params.World, script, "set_cookie", "set_cookie queued")
}

func (h *InteractActionHandler) HandleDeleteCookie(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name      string `json:"name"`
		Domain    string `json:"domain,omitempty"`
		Path      string `json:"path,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if resp, blocked := requireString(req, params.Name, "name", "Add the 'name' parameter and call again"); blocked {
		return resp
	}

	cookie := params.Name + "=; expires=Thu, 01 Jan 1970 00:00:00 GMT"
	if params.Path != "" {
		cookie += "; path=" + params.Path
	} else {
		cookie += "; path=/"
	}
	if params.Domain != "" {
		cookie += "; domain=" + params.Domain
	}

	script := fmt.Sprintf(`(() => { try { document.cookie = %s; return { ok: true, action: "delete_cookie", name: %s }; } catch (e) { return { ok: false, error: String((e && e.message) || e) }; } })()`,
		jsQuote(cookie), jsQuote(params.Name))
	return h.queueExecuteScript(req, args, "cookie_del", params.TabID, params.TimeoutMs, params.World, script, "delete_cookie", "delete_cookie queued")
}

func (h *InteractActionHandler) queueExecuteScript(
	req JSONRPCRequest,
	waitArgs json.RawMessage,
	correlationPrefix string,
	tabID, timeoutMs int,
	world, script, reason, queuedMsg string,
) JSONRPCResponse {
	if world == "" {
		world = "auto"
	}
	if !validWorldValues[world] {
		return fail(req, ErrInvalidParam, "Invalid 'world' value: "+world, "Use 'auto' (default), 'main', or 'isolated'", withParam("world"))
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	return h.newCommand(reason).
		correlationPrefix(correlationPrefix).
		reason(reason).
		queryType("execute").
		buildParams(map[string]any{
			"script":     script,
			"timeout_ms": timeoutMs,
			"world":      world,
			"reason":     reason,
		}).
		tabID(tabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		cspGuard(world).
		queuedMessage(queuedMsg).
		execute(req, waitArgs)
}

func jsQuote(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
