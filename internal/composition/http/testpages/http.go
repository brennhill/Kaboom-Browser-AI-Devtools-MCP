// Purpose: Serves embedded test/demo pages and deterministic HTTP fixtures under /tests/.
// Why: Provides self-contained smoke-test HTTP fixtures (404, 500, CORS, slow) without external dependencies.
// Docs: docs/features/feature/self-testing/index.md

package testpages

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

//go:embed pages
var testPagesFS embed.FS

// handleTestPages serves embedded test pages at /tests/.
// GET /tests/          → branded HTML index listing all pages.
// GET /tests/{file}    → embedded HTML/CSS file.
// GET /tests/404       → 404 response (network error test).
// GET /tests/500       → 500 response (network error test).
// GET /tests/cors-test → 200 JSON, no CORS headers (CORS-block test).
// GET /tests/slow      → 200 after 3 s delay (latency/waterfall test).
func Handler() http.HandlerFunc {
	sub, err := fs.Sub(testPagesFS, "pages")
	if err != nil {
		panic(fmt.Sprintf("testpages: embed misconfigured: %v", err))
	}
	fileServer := http.StripPrefix("/tests/", http.FileServerFS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/tests")
		trimmed = strings.TrimPrefix(trimmed, "/")

		switch trimmed {
		case "":
			serveTestIndex(w)
		case "404":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintln(w, `{"error":"not_found","endpoint":"/tests/404","test":"network_error"}`)
		case "500":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, `{"error":"internal_server_error","endpoint":"/tests/500","test":"network_error"}`)
		case "cors-test":
			// Intentionally omit CORS headers — browser blocks cross-origin fetch.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"cors":"no_headers","test":"cors_block"}`)
		case "slow":
			time.Sleep(3 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"delayed":true,"delay_ms":3000,"test":"slow_response"}`)
		default:
			fileServer.ServeHTTP(w, r)
		}
	}
}

// serveTestIndex generates a branded HTML index of available test pages.
// File names and labels are HTML-escaped to prevent injection from any
// unexpected entries in the embedded filesystem.
func serveTestIndex(w http.ResponseWriter) {
	entries, err := fs.ReadDir(testPagesFS, "pages")
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal_error",
			"message": "failed to read test pages",
		})
		return
	}

	var links []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		safeName := html.EscapeString(name)
		safeLabel := html.EscapeString(strings.TrimSuffix(name, path.Ext(name)))
		links = append(links, fmt.Sprintf(`<li><a href="/tests/%s">%s</a></li>`, safeName, safeLabel))
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Kaboom Test Pages</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
     background:#1a1a1a;color:#e0e0e0;padding:40px;max-width:600px;margin:0 auto}
h1{font-size:20px;color:#fff;margin-bottom:4px}
p{font-size:13px;color:#888;margin-bottom:24px}
ul{list-style:none;padding:0}
li{margin:6px 0}
a{color:#58a6ff;text-decoration:none;font-size:14px}
a:hover{text-decoration:underline}
.ep{font-family:monospace;font-size:12px;color:#888;margin-top:20px;padding-top:16px;border-top:1px solid #333}
.ep a{color:#d29922}
</style></head>
<body>
<h1><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="24" height="24" style="vertical-align:middle;margin-right:6px"><defs><linearGradient id="fl" x1="0%%" y1="100%%" x2="0%%" y2="0%%"><stop offset="0%%" style="stop-color:#f97316"/><stop offset="50%%" style="stop-color:#fb923c"/><stop offset="100%%" style="stop-color:#fbbf24"/></linearGradient><linearGradient id="ifl" x1="0%%" y1="100%%" x2="0%%" y2="0%%"><stop offset="0%%" style="stop-color:#fbbf24"/><stop offset="100%%" style="stop-color:#fef3c7"/></linearGradient></defs><circle cx="64" cy="64" r="60" fill="#1a1a1a"/><path d="M64 16 C40 40,28 60,28 80 C28 100,44 116,64 116 C84 116,100 100,100 80 C100 60,88 40,64 16 Z" fill="url(#fl)"/><path d="M64 48 C52 60,44 72,44 84 C44 96,52 104,64 104 C76 104,84 96,84 84 C84 72,76 60,64 48 Z" fill="url(#ifl)"/></svg>Kaboom — Test Harness</h1>
<p>Deterministic smoke-test pages served by the Kaboom Go daemon.</p>
<ul>%s</ul>
<div class="ep">
  <div>Test endpoints:</div>
  <div><a href="/tests/404">/tests/404</a> — 404 Not Found</div>
  <div><a href="/tests/500">/tests/500</a> — 500 Server Error</div>
  <div><a href="/tests/cors-test">/tests/cors-test</a> — no CORS headers</div>
  <div><a href="/tests/slow">/tests/slow</a> — 3 s delay</div>
  <div>ws://{host}/tests/ws — WebSocket echo</div>
</div>
</body></html>`, strings.Join(links, "\n"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, body)
}
