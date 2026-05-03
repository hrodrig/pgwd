#!/usr/bin/env python3
"""Lightweight HTTP mock server for pgwd notification testing.

Captures POST bodies to /tmp/pgwd-mock-<slug>.json where slug is derived
from the request path (e.g. /loki/api/v1/push -> loki-api-v1-push,
/slack -> slack). Returns 200 + {} for all requests.
"""

import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler


CAPTURE_DIR = "/tmp"
PID_FILE = "/tmp/pgwd-mock.pid"


class MockHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length else b""

        slug = self.path.strip("/").replace("/", "-") or "root"
        capture_path = os.path.join(CAPTURE_DIR, f"pgwd-mock-{slug}.json")
        with open(capture_path, "wb") as f:
            f.write(body)

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b"{}")

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(b"pgwd mock server\n")

    def log_message(self, fmt, *args):
        sys.stderr.write(f"[mock] {fmt % args}\n")


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9999
    with open(PID_FILE, "w") as f:
        f.write(str(os.getpid()))
    server = HTTPServer(("0.0.0.0", port), MockHandler)
    sys.stderr.write(f"[mock] listening on 0.0.0.0:{port}\n")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)


if __name__ == "__main__":
    main()
