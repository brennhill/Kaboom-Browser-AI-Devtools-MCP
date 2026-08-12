// draw_sessions_handler.go — Persisted draw-session listing, loading, and store hydration.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type sessionSummary struct {
	File      string `json:"file"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	ModTime   int64  `json:"mod_time"`
}

// drawHistoryDefaultLimit bounds a listing that would otherwise return every
// session on disk. Measured on a real machine: 4051 sessions, 1,084,063 bytes,
// clamped by the response safety net rather than by anything that understood
// what the caller wanted. Newest sessions are the useful ones, and the listing
// is sorted newest-first, so a page from the front is the right default.
const drawHistoryDefaultLimit = 200

// ListDrawHistory lists draw sessions newest-first, returning at most limit of
// them. A non-positive limit falls back to the default rather than meaning
// "unbounded": an accidental zero must not resurrect the megabyte response.
func ListDrawHistory(req mcp.JSONRPCRequest, dir string, dirErr error, limit int) mcp.JSONRPCResponse {
	if limit <= 0 {
		limit = drawHistoryDefaultLimit
	}
	if dirErr != nil {
		return mcp.Fail(req, mcp.ErrNoData, "Cannot access screenshots directory: "+dirErr.Error(), "Check file permissions")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "Cannot read screenshots directory: "+err.Error(), "Check file permissions")
	}
	sessions := make([]sessionSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "draw-session-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionSummary{
			File: entry.Name(), Path: filepath.Join(dir, entry.Name()),
			SizeBytes: info.Size(), ModTime: info.ModTime().UnixMilli(),
		})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ModTime > sessions[j].ModTime })
	total := len(sessions)
	truncated := total > limit
	if truncated {
		sessions = sessions[:limit]
	}
	payload := map[string]any{
		// count keeps its existing meaning — how many sessions are in this
		// response — because callers already read it that way. total is the new
		// field, and the pair is what tells a caller the listing was cut.
		"sessions": sessions, "count": len(sessions), "storage_dir": dir,
		"total": total,
	}
	if truncated {
		payload["truncated"] = true
		payload["hint"] = "Showing the newest sessions. Pass limit to widen the page."
	}
	return mcp.Succeed(req, "Draw session history", payload)
}

func LoadDrawSession(store *Store, req mcp.JSONRPCRequest, args json.RawMessage, dir string, dirErr error) mcp.JSONRPCResponse {
	var params struct {
		File string `json:"file"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	if strings.TrimSpace(params.File) == "" {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'file' is missing",
			"Provide the session filename from draw_history results", mcp.WithParam("file"))
	}
	if strings.Contains(params.File, "/") || strings.Contains(params.File, "\\") || strings.Contains(params.File, "..") {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid filename: path traversal not allowed",
			"Use only the filename from draw_history results", mcp.WithParam("file"))
	}
	if dirErr != nil {
		return mcp.Fail(req, mcp.ErrNoData, "Cannot access screenshots directory: "+dirErr.Error(), "Check file permissions")
	}
	path := filepath.Join(dir, params.File)
	if !withinDir(path, dir) {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid filename: resolved path outside screenshots directory",
			"Use only the filename from draw_history results", mcp.WithParam("file"))
	}
	data, err := os.ReadFile(path) // #nosec G304 -- filename is traversal-checked and constrained to dir.
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.Fail(req, mcp.ErrNoData, "Draw session file not found: "+params.File,
				"Use analyze({what:'draw_history'}) to list available sessions")
		}
		return mcp.Fail(req, mcp.ErrNoData, "Cannot read draw session file: "+err.Error(), "Check file permissions")
	}
	var session map[string]any
	if err := json.Unmarshal(data, &session); err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Corrupted draw session file: "+err.Error(),
			"The file may be damaged. Try a different session.")
	}
	hydrateDrawSession(store, data)
	if name, ok := session["annot_session_name"].(string); ok && strings.TrimSpace(name) != "" {
		session["annot_session"] = name
	}
	session["_file"] = params.File
	session["_path"] = path
	return mcp.Succeed(req, "Draw session loaded", session)
}

type persistedDrawSession struct {
	Annotations      []Annotation               `json:"annotations"`
	ElementDetails   map[string]json.RawMessage `json:"element_details"`
	PageURL          string                     `json:"page_url"`
	TabID            int                        `json:"tab_id"`
	Screenshot       string                     `json:"screenshot"`
	Timestamp        int64                      `json:"timestamp"`
	AnnotSessionName string                     `json:"annot_session_name"`
}

func hydrateDrawSession(store *Store, raw []byte) {
	var persisted persistedDrawSession
	if store == nil || json.Unmarshal(raw, &persisted) != nil {
		return
	}
	if persisted.TabID > 0 {
		session := &Session{
			Annotations: persisted.Annotations, ScreenshotPath: persisted.Screenshot,
			PageURL: persisted.PageURL, TabID: persisted.TabID, Timestamp: persisted.Timestamp,
		}
		if session.Timestamp == 0 {
			session.Timestamp = store.now().UnixMilli()
		}
		store.StoreSession(session.TabID, session)
		name := strings.TrimSpace(persisted.AnnotSessionName)
		if name != "" && !namedSessionHasPage(store, name, session) {
			store.AppendToNamedSession(name, session)
		}
	}
	for correlationID, rawDetail := range persisted.ElementDetails {
		var detail Detail
		if json.Unmarshal(rawDetail, &detail) != nil {
			continue
		}
		detail.CorrelationID = correlationID
		store.StoreDetail(correlationID, detail)
	}
}

func namedSessionHasPage(store *Store, name string, session *Session) bool {
	named := store.GetNamedSession(name)
	if named == nil {
		return false
	}
	for _, page := range named.Pages {
		if page.TabID == session.TabID && page.Timestamp == session.Timestamp && page.PageURL == session.PageURL {
			return true
		}
	}
	return false
}

func withinDir(path, dir string) bool {
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
