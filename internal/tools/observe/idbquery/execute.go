// Purpose: Executes IndexedDB listing and entry queries via the extension execute channel.
// Why: Separates query dispatch and reply normalization from the script templates in scripts.go.
// Docs: docs/features/feature/observe/index.md

package idbquery

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// queryTimeout bounds a single IndexedDB round-trip through the extension.
const queryTimeout = 10 * time.Second

// Listing enumerates IndexedDB databases in the tracked tab, each with its object stores,
// sorted by database name. A page without IndexedDB reports supported=false rather than erroring.
func Listing(cap *capture.Store) (map[string]any, error) {
	data, err := executeScript(cap, listingScript, "observe_storage_indexeddb", queryTimeout)
	if err != nil {
		return nil, err
	}

	if _, ok := data["supported"]; !ok {
		data["supported"] = true
	}
	if _, ok := data["databases"]; !ok {
		data["databases"] = []any{}
	}

	if dbs, ok := data["databases"].([]any); ok {
		sort.SliceStable(dbs, func(i, j int) bool {
			left, _ := dbs[i].(map[string]any)
			right, _ := dbs[j].(map[string]any)
			leftName, _ := left["name"].(string)
			rightName, _ := right["name"].(string)
			return leftName < rightName
		})
		data["databases"] = dbs
	}

	return data, nil
}

// Entries reads up to limit rows from one object store. It returns an error when the page
// reports failure (missing database, missing store, or a serialization fault).
func Entries(cap *capture.Store, database, store string, limit int) (map[string]any, error) {
	script := buildEntriesScript(database, store, limit)
	data, err := executeScript(cap, script, "observe_indexeddb_entries", queryTimeout)
	if err != nil {
		return nil, err
	}

	if ok, hasOK := data["ok"].(bool); hasOK && !ok {
		return nil, errors.New(resultErrorMessage(data))
	}

	if _, ok := data["entries"]; !ok {
		data["entries"] = []any{}
	}
	if _, ok := data["count"]; !ok {
		if entries, ok := data["entries"].([]any); ok {
			data["count"] = len(entries)
		} else {
			data["count"] = 0
		}
	}
	if _, ok := data["database"]; !ok {
		data["database"] = database
	}
	if _, ok := data["store"]; !ok {
		data["store"] = store
	}

	return data, nil
}

func executeScript(cap *capture.Store, script, reason string, timeout time.Duration) (map[string]any, error) {
	params, _ := json.Marshal(map[string]any{
		"script":     script,
		"timeout_ms": int(timeout.Milliseconds()),
		"world":      "auto",
		"reason":     reason,
	})

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "execute",
			Params: params,
		},
		timeout,
		"",
	)
	if qerr != nil {
		return nil, qerr
	}

	result, err := cap.WaitForResult(queryID, timeout)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse execute result: %w", err)
	}

	if successRaw, hasSuccess := payload["success"]; hasSuccess {
		success, _ := successRaw.(bool)
		if !success {
			return nil, errors.New(resultErrorMessage(payload))
		}
		if rawResult, ok := payload["result"].(map[string]any); ok {
			return rawResult, nil
		}
		return map[string]any{"value": payload["result"]}, nil
	}

	if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
		return nil, errors.New(errMsg)
	}

	return payload, nil
}

func resultErrorMessage(payload map[string]any) string {
	if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
		return errMsg
	}
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}
	if result, ok := payload["result"].(map[string]any); ok {
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			return errMsg
		}
		if msg, ok := result["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return "extension execution failed"
}
