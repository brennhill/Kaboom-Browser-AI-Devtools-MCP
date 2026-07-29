#!/usr/bin/env python3
# Purpose: Automate test-upload-server.py workflow behavior for repository tooling.
# Why: Keeps repetitive maintenance and verification steps deterministic.
# Docs: docs/DEVELOPMENT.md

"""test-upload-server.py — Unit tests for the upload server hardened form endpoint.

Tests that GET /upload/hardened returns a form with isTrusted validation.

Usage:
  python3 scripts/smoke-tests/test-upload-server.py
"""
import http.client
import json
import os
import re
import subprocess
import sys
import time
import unittest


SERVER_PORT = 19876  # Use a distinct port to avoid conflicts
SERVER_PROC = None


def setUpModule():
    """Start the upload server before all tests."""
    global SERVER_PROC
    server_script = os.path.join(os.path.dirname(__file__), "upload-server.py")
    SERVER_PROC = subprocess.Popen(
        [sys.executable, server_script, str(SERVER_PORT)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    # Wait for server to be ready
    for _ in range(20):
        try:
            conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT, timeout=1)
            conn.request("GET", "/health")
            resp = conn.getresponse()
            if resp.status == 200:
                conn.close()
                return
            conn.close()
        except (ConnectionRefusedError, OSError):
            time.sleep(0.25)
    raise RuntimeError(f"Upload server did not start on port {SERVER_PORT}")


def tearDownModule():
    """Stop the upload server after all tests."""
    if SERVER_PROC:
        SERVER_PROC.terminate()
        SERVER_PROC.wait(timeout=5)


def _get_session_cookie():
    """Visit / to get a session cookie."""
    conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
    conn.request("GET", "/")
    resp = conn.getresponse()
    resp.read()
    cookie = resp.getheader("Set-Cookie", "")
    conn.close()
    # Extract session=... part
    for part in cookie.split(";"):
        part = part.strip()
        if part.startswith("session="):
            return part
    return ""


class TestHardenedForm(unittest.TestCase):
    """Tests for GET /upload/hardened endpoint."""

    def setUp(self):
        self.cookie = _get_session_cookie()
        self.assertTrue(self.cookie, "Should get a session cookie from /")

    def _get_hardened(self):
        conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
        conn.request("GET", "/upload/hardened", headers={"Cookie": self.cookie})
        resp = conn.getresponse()
        body = resp.read().decode("utf-8")
        status = resp.status
        conn.close()
        return status, body

    def test_hardened_returns_200(self):
        status, _ = self._get_hardened()
        self.assertEqual(status, 200)

    def test_hardened_contains_isTrusted(self):
        _, body = self._get_hardened()
        self.assertIn("isTrusted", body, "Hardened form should check event.isTrusted")

    def test_hardened_contains_trust_status_element(self):
        _, body = self._get_hardened()
        self.assertIn('id="trust-status"', body, "Hardened form should have #trust-status element")

    def test_hardened_contains_file_input(self):
        _, body = self._get_hardened()
        self.assertIn('id="file-input"', body, "Hardened form should have #file-input element")

    def test_hardened_form_action_is_upload(self):
        _, body = self._get_hardened()
        self.assertIn('action="/upload"', body, "Hardened form should POST to /upload")

    def test_hardened_requires_session(self):
        """GET /upload/hardened without session cookie should return 401."""
        conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
        conn.request("GET", "/upload/hardened")
        resp = conn.getresponse()
        resp.read()
        status = resp.status
        conn.close()
        self.assertEqual(status, 401)

    def test_hardened_contains_onchange(self):
        _, body = self._get_hardened()
        self.assertIn("onchange", body, "Hardened form should have onchange handler")

    def test_hardened_contains_csrf_token(self):
        _, body = self._get_hardened()
        self.assertIn("csrf_token", body, "Hardened form should include CSRF token")


class TestStandardForm(unittest.TestCase):
    """Verify standard form still works (regression guard)."""

    def setUp(self):
        self.cookie = _get_session_cookie()

    def test_standard_form_no_isTrusted(self):
        conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
        conn.request("GET", "/upload", headers={"Cookie": self.cookie})
        resp = conn.getresponse()
        body = resp.read().decode("utf-8")
        conn.close()
        self.assertNotIn("isTrusted", body, "Standard form should NOT check isTrusted")

    def test_chunked_multipart_submission_accepts_valid_csrf(self):
        conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
        conn.request("GET", "/upload", headers={"Cookie": self.cookie})
        resp = conn.getresponse()
        form = resp.read().decode("utf-8")
        conn.close()
        token_match = re.search(r'name="csrf_token" value="([^"]+)"', form)
        self.assertIsNotNone(token_match, "Upload form should expose a CSRF token")

        boundary = "kaboom-test-boundary"
        body = (
            f"--{boundary}\r\n"
            'Content-Disposition: form-data; name="csrf_token"\r\n\r\n'
            f"{token_match.group(1)}\r\n"
            f"--{boundary}\r\n"
            'Content-Disposition: form-data; name="title"\r\n\r\n'
            "Chunked upload\r\n"
            f"--{boundary}\r\n"
            'Content-Disposition: form-data; name="Filedata"; filename="test.txt"\r\n'
            "Content-Type: text/plain\r\n\r\n"
            "file content\r\n"
            f"--{boundary}--\r\n"
        ).encode()
        conn = http.client.HTTPConnection("127.0.0.1", SERVER_PORT)
        conn.request(
            "POST",
            "/upload",
            body=(body[index : index + 17] for index in range(0, len(body), 17)),
            headers={
                "Cookie": self.cookie,
                "Content-Type": f"multipart/form-data; boundary={boundary}",
            },
            encode_chunked=True,
        )
        resp = conn.getresponse()
        resp.read()
        status = resp.status
        conn.close()

        self.assertEqual(status, 302)


if __name__ == "__main__":
    unittest.main()
