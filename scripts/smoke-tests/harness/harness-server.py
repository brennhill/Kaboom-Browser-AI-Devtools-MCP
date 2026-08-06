#!/usr/bin/env python3
"""Deterministic local harness server for smoke/integration tests.

Serves static files from a caller-provided root and exposes deterministic API
endpoints for telemetry/performance scenarios.
"""

from __future__ import annotations

import argparse
import json
import time
from http import HTTPStatus
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse


class HarnessHandler(SimpleHTTPRequestHandler):
    def __init__(self, *args, directory: str, **kwargs):
        super().__init__(*args, directory=directory, **kwargs)

    def log_message(self, fmt: str, *args) -> None:
        # Keep smoke output readable while retaining request visibility in harness.log.
        super().log_message(fmt, *args)

    def _send_json(self, status: int, payload: dict) -> None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def end_headers(self) -> None:
        self.send_header("Cache-Control", "no-store")
        self.send_header("Access-Control-Allow-Origin", "*")
        super().end_headers()

    def do_GET(self) -> None:  # noqa: N802
        path = urlparse(self.path).path

        if path == "/healthz":
            self._send_json(HTTPStatus.OK, {"status": "ok"})
            return

        # Smoke URL rewriting maps https://example.com* into /example.com* on this
        # local harness. Serve deterministic HTML instead of Python's default 404 page.
        if path in ("/example.com", "/example.com/", "/example.org", "/example.org/"):
            self.path = "/index.html"
            super().do_GET()
            return
        if path.startswith("/example.com/") or path.startswith("/example.org/"):
            self.path = "/index.html"
            super().do_GET()
            return

        if path == "/api/status/404":
            self._send_json(HTTPStatus.NOT_FOUND, {"status": 404, "error": "not_found"})
            return

        if path == "/api/status/500":
            self._send_json(HTTPStatus.INTERNAL_SERVER_ERROR, {"status": 500, "error": "server_error"})
            return

        if path == "/api/slow":
            time.sleep(2.0)
            self._send_json(HTTPStatus.OK, {"status": "ok", "delayed_ms": 2000})
            return

        super().do_GET()


def main() -> None:
    parser = argparse.ArgumentParser(description="Kaboom deterministic test harness server")
    parser.add_argument("--root", required=True, help="Directory to serve")
    parser.add_argument("--port", type=int, default=8787, help="Bind port")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    if not root.exists() or not root.is_dir():
        raise SystemExit(f"invalid harness root: {root}")

    handler = lambda *a, **kw: HarnessHandler(*a, directory=str(root), **kw)  # noqa: E731
    server = ThreadingHTTPServer(("127.0.0.1", args.port), handler)
    print(f"Harness server listening on http://127.0.0.1:{args.port} root={root}", flush=True)
    try:
        server.serve_forever(poll_interval=0.2)
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
