#!/usr/bin/env python3
# Purpose: Branded zero-dependency upload harness used by smoke and UAT tests.
# Why: Keeps upload tests deterministic while mirroring Kaboom visual language.
# Docs: docs/features/feature/file-upload/index.md

"""upload-server.py — Zero-dependency test upload server for Stage 3/4 upload flows.

Endpoints:
  GET  /                          — Landing page, sets session cookie
  GET  /upload                    — Upload form (requires session cookie, generates CSRF token)
  GET  /upload/hardened           — Upload form with inline event.isTrusted guard
  GET  /upload/hardened-addeventlistener — Upload form with addEventListener event.isTrusted guard
  POST /upload                    — Upload receiver (validates cookie, CSRF, file, required fields)
  GET  /upload/success            — Confirmation page after successful upload
  GET  /api/last-upload           — JSON of last upload for programmatic verification
  GET  /logout                    — Clears session cookie
  GET  /health                    — Health check

Usage:
  python3 upload-server.py [PORT]       # default 9876
"""

import hashlib
import html
import http.server
import json
import sys
import time
import urllib.parse


LOGO_SVG = """
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="26" height="26" aria-hidden="true">
  <defs>
    <linearGradient id="upload-fl" x1="0%" y1="100%" x2="0%" y2="0%">
      <stop offset="0%" style="stop-color:#f97316"/>
      <stop offset="50%" style="stop-color:#fb923c"/>
      <stop offset="100%" style="stop-color:#fbbf24"/>
    </linearGradient>
    <linearGradient id="upload-ifl" x1="0%" y1="100%" x2="0%" y2="0%">
      <stop offset="0%" style="stop-color:#fbbf24"/>
      <stop offset="100%" style="stop-color:#fef3c7"/>
    </linearGradient>
  </defs>
  <circle cx="64" cy="64" r="60" fill="#302d2a"/>
  <path d="M64 16 C40 40,28 60,28 80 C28 100,44 116,64 116 C84 116,100 100,100 80 C100 60,88 40,64 16 Z" fill="url(#upload-fl)"/>
  <path d="M64 48 C52 60,44 72,44 84 C44 96,52 104,64 104 C76 104,84 96,84 84 C84 72,76 60,64 48 Z" fill="url(#upload-ifl)"/>
</svg>
"""


BRAND_CSS = """
:root {
  --bg: #1a1a1a;
  --bg-section: #252525;
  --bg-card: #2a2a2a;
  --bg-input: #1e1e1e;
  --text: #e0e0e0;
  --text-dim: #93a1ad;
  --text-bright: #ffffff;
  --border: #333;
  --border-bright: #444;
  --green: #3fb950;
  --green-bg: rgba(63, 185, 80, 0.12);
  --red: #f85149;
  --red-bg: rgba(248, 81, 73, 0.12);
  --blue: #58a6ff;
  --blue-bg: rgba(88, 166, 255, 0.12);
  --amber: #d29922;
  --amber-bg: rgba(210, 153, 34, 0.12);
  --flame-base: #f97316;
  --flame-mid: #fb923c;
  --flame-tip: #fbbf24;
}
*,
*::before,
*::after {
  box-sizing: border-box;
}
html,
body {
  margin: 0;
  padding: 0;
}
body {
  min-height: 100vh;
  background: radial-gradient(circle at 20% 0%, rgba(249, 115, 22, 0.1), transparent 40%), var(--bg);
  color: var(--text);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  line-height: 1.45;
}
.gh-header {
  background: var(--bg-section);
  border-bottom: 1px solid var(--border);
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 10px;
}
.gh-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-bright);
  text-decoration: none;
  font-weight: 700;
}
.gh-badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 600;
  border: 1px solid rgba(88, 166, 255, 0.3);
  background: var(--blue-bg);
  color: var(--blue);
}
.gh-main {
  max-width: 860px;
  margin: 0 auto;
  padding: 24px 16px 56px;
}
.hero {
  margin-bottom: 16px;
}
.hero h1 {
  margin: 0 0 6px;
  font-size: 24px;
  color: var(--text-bright);
}
.hero p {
  margin: 0;
  color: var(--text-dim);
  font-size: 14px;
}
.panel {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg-section);
  overflow: hidden;
  margin-bottom: 14px;
}
.panel-head {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 8px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--green);
}
.panel-head h2 {
  margin: 0;
  font-size: 14px;
  color: var(--text-bright);
}
.panel-meta {
  margin-left: auto;
  color: var(--text-dim);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}
.panel-body {
  padding: 14px;
}
.row {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr 1fr;
}
.field {
  margin-bottom: 10px;
}
label {
  display: block;
  margin-bottom: 4px;
  color: var(--text-dim);
  font-size: 12px;
}
input[type="text"],
input[type="file"],
textarea {
  width: 100%;
  background: var(--bg-input);
  color: var(--text);
  border: 1px solid var(--border-bright);
  border-radius: 7px;
  padding: 8px 10px;
  font-size: 13px;
  font-family: inherit;
}
textarea {
  min-height: 78px;
  resize: vertical;
}
input:focus,
textarea:focus {
  outline: none;
  border-color: var(--blue);
  box-shadow: 0 0 0 3px rgba(88, 166, 255, 0.15);
}
.btn-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 13px;
  border-radius: 7px;
  padding: 8px 12px;
  border: none;
  cursor: pointer;
  font-family: inherit;
  font-weight: 600;
  text-decoration: none;
}
.btn-primary {
  background: linear-gradient(90deg, var(--flame-base), var(--flame-mid), var(--flame-tip));
  color: #111827;
}
.btn-ghost {
  background: var(--bg-card);
  border: 1px solid var(--border-bright);
  color: var(--text);
}
.status {
  padding: 9px 11px;
  border-radius: 7px;
  font-size: 13px;
  margin-bottom: 10px;
  border: 1px solid transparent;
}
.status-ok {
  background: var(--green-bg);
  border-color: rgba(63, 185, 80, 0.3);
  color: var(--green);
}
.status-warn {
  background: var(--amber-bg);
  border-color: rgba(210, 153, 34, 0.3);
  color: #f5bf42;
}
.status-error {
  background: var(--red-bg);
  border-color: rgba(248, 81, 73, 0.35);
  color: #ff9f9f;
}
.kv {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 8px;
  font-size: 13px;
}
.kv code {
  color: var(--text-bright);
}
a {
  color: var(--blue);
}
@media (max-width: 720px) {
  .row {
    grid-template-columns: 1fr;
  }
  .hero h1 {
    font-size: 21px;
  }
}
"""


def render_page(title, badge, heading, subtitle, content_html):
    """Render a branded HTML shell for all upload harness pages."""
    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{title}</title>
  <style>{BRAND_CSS}</style>
</head>
<body>
  <header class="gh-header">
    <a class="gh-logo" href="/">
      {LOGO_SVG}
      <span>Kaboom</span>
    </a>
    <span class="gh-badge">{badge}</span>
  </header>
  <main class="gh-main">
    <section class="hero">
      <h1>{heading}</h1>
      <p>{subtitle}</p>
    </section>
    {content_html}
  </main>
</body>
</html>"""


def render_error_page(code, title, detail, call_to_action):
    """Render a consistent branded error page."""
    panel = f"""
<section class="panel">
  <div class="panel-head"><span class="dot" style="background:var(--red)"></span><h2>{code} {title}</h2></div>
  <div class="panel-body">
    <div class="status status-error">{detail}</div>
    <div class="btn-row">
      <a class="btn btn-ghost" href="/">Back to home</a>
      {call_to_action}
    </div>
  </div>
</section>"""
    return render_page(f"{code} {title}", "Upload Harness", f"{code} {title}", detail, panel)


def render_upload_form(token, variant):
    """Render standard/hardened upload form variants."""
    trust_status = ""
    onchange_attr = ""
    listener_script = ""
    headline = "Upload File"
    subtitle = "MCP-compatible form with deterministic fields for smoke and UAT flows."

    if variant == "hardened-inline":
        headline = "Upload File (Hardened)"
        subtitle = "Inline onchange guard rejects synthetic file input events."
        trust_status = '<div class="status status-warn" id="trust-status">Waiting for file selection...</div>'
        onchange_attr = (
            ' onchange="if(!event.isTrusted){this.value=\'\';document.getElementById(\'trust-status\').textContent='
            '\'REJECTED: event.isTrusted=false\';}else{document.getElementById(\'trust-status\').textContent='
            '\'OK: trusted event\';}"'
        )
    elif variant == "hardened-listener":
        headline = "Upload File (Hardened addEventListener)"
        subtitle = "Event listener guard validates trust after file input change."
        trust_status = '<div class="status status-warn" id="trust-status">Waiting for file selection...</div>'
        listener_script = """
<script>
document.getElementById('file-input').addEventListener('change', function(event) {
  if (!event.isTrusted) {
    this.value = '';
    document.getElementById('trust-status').textContent =
      'REJECTED: event.isTrusted=false (addEventListener)';
  } else {
    document.getElementById('trust-status').textContent =
      'OK: trusted event (addEventListener)';
  }
});
</script>"""

    form_panel = f"""
<section class="panel">
  <div class="panel-head">
    <span class="dot"></span>
    <h2>{headline}</h2>
    <span class="panel-meta">POST /upload</span>
  </div>
  <div class="panel-body">
    <p style="margin:0 0 10px;color:var(--text-dim);font-size:13px;">{subtitle}</p>
    {trust_status}
    <form method="POST" action="/upload" enctype="multipart/form-data">
      <input type="hidden" name="csrf_token" value="{token}">
      <div class="field">
        <label for="file-input">File</label>
        <input type="file" id="file-input" name="Filedata" accept="video/*,image/*,.txt,.pdf"{onchange_attr}>
      </div>
      <div class="row">
        <div class="field">
          <label for="title">Title</label>
          <input type="text" id="title" name="title" required>
        </div>
        <div class="field">
          <label for="tags">Tags</label>
          <input type="text" id="tags" name="tags">
        </div>
      </div>
      <div class="field">
        <label for="description">Description</label>
        <textarea id="description" name="description"></textarea>
      </div>
      <div class="btn-row">
        <button class="btn btn-primary" type="submit">Upload</button>
        <a class="btn btn-ghost" href="/logout">Logout</a>
      </div>
    </form>
  </div>
</section>
{listener_script}"""
    return render_page(
        "Upload",
        "Upload Harness",
        headline,
        "Branded local fixture for file upload smoke tests.",
        form_panel,
    )


# ── State ──────────────────────────────────────────────────
csrf_tokens = {}  # session -> token
last_upload = {}  # last successful upload details
upload_counter = 0


def parse_multipart(content_type, body):
    """Parse multipart/form-data body. Returns (fields, files) dicts."""
    # Extract boundary from content-type
    parts = content_type.split("boundary=")
    if len(parts) < 2:
        return {}, {}
    boundary = parts[1].strip()
    if boundary.startswith('"') and boundary.endswith('"'):
        boundary = boundary[1:-1]

    fields = {}
    files = {}
    boundary_bytes = ("--" + boundary).encode()

    # Split body by boundary
    raw_parts = body.split(boundary_bytes)
    for part in raw_parts:
        if not part or part.strip() in (b"", b"--"):
            continue
        if part.startswith(b"--"):
            continue

        # Split headers from body
        header_end = part.find(b"\r\n\r\n")
        if header_end == -1:
            continue
        header_data = part[:header_end].decode("utf-8", errors="replace")
        part_body = part[header_end + 4 :]

        # Strip trailing \r\n
        if part_body.endswith(b"\r\n"):
            part_body = part_body[:-2]

        # Parse Content-Disposition
        name = None
        filename = None
        for line in header_data.split("\r\n"):
            if "Content-Disposition" in line:
                for item in line.split(";"):
                    item = item.strip()
                    if item.startswith("name="):
                        name = item.split("=", 1)[1].strip('"')
                    elif item.startswith("filename="):
                        filename = item.split("=", 1)[1].strip('"')

        if name is None:
            continue
        if filename is not None:
            files[name] = {"filename": filename, "data": part_body}
        else:
            fields[name] = part_body.decode("utf-8", errors="replace")

    return fields, files


class UploadHandler(http.server.BaseHTTPRequestHandler):
    """HTTP handler for the branded upload test server."""

    def log_message(self, _format, *_args):
        """Suppress default request logging."""
        pass

    def _send_html(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(body.encode())

    def _send_json(self, code, obj):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(obj).encode())

    def _get_session(self):
        cookie_header = self.headers.get("Cookie", "")
        for pair in cookie_header.split(";"):
            pair = pair.strip()
            if pair.startswith("session="):
                return pair.split("=", 1)[1]
        return None

    def _require_session(self):
        session = self._get_session()
        if session:
            return session
        self._send_html(
            401,
            render_error_page(
                401,
                "Not logged in",
                "Visit / first to get a session cookie.",
                '<a class="btn btn-primary" href="/">Get session</a>',
            ),
        )
        return None

    def _read_request_body(self):
        if "chunked" not in self.headers.get("Transfer-Encoding", "").lower():
            return self.rfile.read(int(self.headers.get("Content-Length", 0)))

        chunks = []
        while True:
            size_line = self.rfile.readline().strip().split(b";", 1)[0]
            size = int(size_line, 16)
            if size == 0:
                while self.rfile.readline() not in (b"\r\n", b"\n", b""):
                    pass
                break
            chunks.append(self.rfile.read(size))
            if self.rfile.read(2) != b"\r\n":
                raise ValueError("invalid chunk terminator")
        return b"".join(chunks)

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)

        if path == "/health":
            self._send_json(200, {"ok": True})
            return

        if path == "/":
            session_id = f"test-session-{int(time.time())}"
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Set-Cookie", f"session={session_id}; Path=/; HttpOnly")
            self.end_headers()

            session_safe = html.escape(session_id)
            panel = f"""
<section class="panel">
  <div class="panel-head">
    <span class="dot"></span>
    <h2>Session Ready</h2>
    <span class="panel-meta">Cookie: session=...</span>
  </div>
  <div class="panel-body">
    <div class="status status-ok">Session created and stored in cookie jar.</div>
    <div class="kv">
      <div>Session ID</div><code>{session_safe}</code>
      <div>Next step</div><div>Open <code>/upload</code> to generate CSRF token and submit form data.</div>
    </div>
    <div class="btn-row">
      <a class="btn btn-primary" href="/upload">Go to Upload</a>
      <a class="btn btn-ghost" href="/upload/hardened">Go to Hardened Upload</a>
    </div>
  </div>
</section>"""
            self.wfile.write(
                render_page(
                    "Upload Harness",
                    "Upload Harness",
                    "Test Upload Platform",
                    "Deterministic fixture for smoke Category 15 and UAT Category 24.",
                    panel,
                ).encode()
            )
            return

        if path == "/upload":
            session = self._require_session()
            if not session:
                return
            token = hashlib.sha256(f"{session}-{time.time()}".encode()).hexdigest()[:32]
            csrf_tokens[session] = token
            self._send_html(200, render_upload_form(token, "standard"))
            return

        if path == "/upload/hardened":
            session = self._require_session()
            if not session:
                return
            token = hashlib.sha256(f"{session}-{time.time()}".encode()).hexdigest()[:32]
            csrf_tokens[session] = token
            self._send_html(200, render_upload_form(token, "hardened-inline"))
            return

        if path == "/upload/hardened-addeventlistener":
            session = self._require_session()
            if not session:
                return
            token = hashlib.sha256(f"{session}-{time.time()}".encode()).hexdigest()[:32]
            csrf_tokens[session] = token
            self._send_html(200, render_upload_form(token, "hardened-listener"))
            return

        if path == "/upload/success":
            upload_id = html.escape(query.get("id", ["unknown"])[0])
            info = last_upload if last_upload.get("id") == upload_id else {}
            panel = f"""
<section class="panel">
  <div class="panel-head">
    <span class="dot"></span>
    <h2>Upload Successful</h2>
    <span class="panel-meta">{upload_id}</span>
  </div>
  <div class="panel-body">
    <div class="status status-ok">Server accepted multipart payload and recorded verification metadata.</div>
    <div class="kv">
      <div>Upload ID</div><code>{upload_id}</code>
      <div>Filename</div><code>{html.escape(str(info.get("name", "N/A")))}</code>
      <div>Size</div><code>{html.escape(str(info.get("size", "N/A")))} bytes</code>
      <div>MD5</div><code>{html.escape(str(info.get("md5", "N/A")))}</code>
      <div>Title</div><code>{html.escape(str(info.get("title", "N/A")))}</code>
    </div>
    <div class="btn-row">
      <a class="btn btn-primary" href="/upload">Upload another file</a>
      <a class="btn btn-ghost" href="/api/last-upload">View /api/last-upload</a>
    </div>
  </div>
</section>"""
            self._send_html(
                200,
                render_page(
                    "Upload Success",
                    "Upload Harness",
                    "Upload Successful",
                    "Stage 3 verification fixture confirms file arrived with expected digest.",
                    panel,
                ),
            )
            return

        if path == "/logout":
            self.send_response(200)
            self.send_header("Set-Cookie", "session=; Path=/; HttpOnly; Max-Age=0")
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            panel = """
<section class="panel">
  <div class="panel-head">
    <span class="dot" style="background:var(--amber)"></span>
    <h2>Logged out</h2>
  </div>
  <div class="panel-body">
    <div class="status status-warn">Session cookie cleared. Upload pages now return 401 until / is visited again.</div>
    <div class="btn-row">
      <a class="btn btn-primary" href="/">Create new session</a>
      <a class="btn btn-ghost" href="/upload">Try /upload (should 401)</a>
    </div>
  </div>
</section>"""
            self.wfile.write(
                render_page(
                    "Logged out",
                    "Upload Harness",
                    "Logged out",
                    "Session was intentionally cleared for auth-path testing.",
                    panel,
                ).encode()
            )
            return

        if path == "/api/last-upload":
            self._send_json(200, last_upload if last_upload else {"error": "no uploads yet"})
            return

        self._send_html(
            404,
            render_error_page(
                404,
                "Not Found",
                "No route matches this upload harness path.",
                '<a class="btn btn-primary" href="/upload">Open upload form</a>',
            ),
        )

    def do_POST(self):
        global last_upload, upload_counter

        parsed = urllib.parse.urlparse(self.path)
        if parsed.path != "/upload":
            self._send_html(
                404,
                render_error_page(
                    404,
                    "Not Found",
                    "POST target is not implemented on this harness.",
                    '<a class="btn btn-primary" href="/upload">Open upload form</a>',
                ),
            )
            return

        session = self._get_session()
        cookie_ok = session is not None
        if not cookie_ok:
            self._send_html(
                401,
                render_error_page(
                    401,
                    "Not logged in",
                    "Session cookie required.",
                    '<a class="btn btn-primary" href="/">Get session</a>',
                ),
            )
            return

        body = self._read_request_body()
        content_type = self.headers.get("Content-Type", "")

        fields, files = parse_multipart(content_type, body)

        csrf_sent = fields.get("csrf_token", "")
        csrf_expected = csrf_tokens.get(session, "")
        csrf_ok = csrf_sent != "" and csrf_sent == csrf_expected
        if not csrf_ok:
            self._send_html(
                403,
                render_error_page(
                    403,
                    "CSRF token expired",
                    "CSRF token mismatch.",
                    '<a class="btn btn-primary" href="/upload">Reload form</a>',
                ),
            )
            return

        file_entry = files.get("Filedata")
        if not file_entry:
            self._send_html(
                422,
                render_error_page(
                    422,
                    "No file uploaded",
                    "The Filedata field is required.",
                    '<a class="btn btn-primary" href="/upload">Back to form</a>',
                ),
            )
            return
        if len(file_entry["data"]) == 0:
            self._send_html(
                422,
                render_error_page(
                    422,
                    "Empty file",
                    "File must not be empty.",
                    '<a class="btn btn-primary" href="/upload">Back to form</a>',
                ),
            )
            return

        title = fields.get("title", "")
        if not title:
            self._send_html(
                422,
                render_error_page(
                    422,
                    "Missing title",
                    "The title field is required.",
                    '<a class="btn btn-primary" href="/upload">Back to form</a>',
                ),
            )
            return

        upload_counter += 1
        upload_id = f"upload-{upload_counter}"
        file_data = file_entry["data"]
        md5 = hashlib.md5(file_data).hexdigest()

        last_upload = {
            "id": upload_id,
            "name": file_entry["filename"],
            "size": len(file_data),
            "md5": md5,
            "title": title,
            "tags": fields.get("tags", ""),
            "csrf_ok": csrf_ok,
            "cookie_ok": cookie_ok,
        }

        self.send_response(302)
        self.send_header("Location", f"/upload/success?id={upload_id}")
        self.end_headers()


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9876
    server = http.server.HTTPServer(("127.0.0.1", port), UploadHandler)
    print(f"Upload test server on http://127.0.0.1:{port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    server.server_close()


if __name__ == "__main__":
    main()
