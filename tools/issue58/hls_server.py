"""Loopback-only HLS fixture with per-request latency/rate and JSONL evidence."""
import argparse
import http.server
import json
import mimetypes
from pathlib import Path
import re
import threading
import time
import urllib.parse


class Server(http.server.ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, root, latency_ms=0, kib_s=0, log=None, port=8765):
        self.root = Path(root).resolve()
        self.latency = latency_ms / 1000
        self.rate = kib_s * 1024
        self.log = log
        self.lock = threading.Lock()
        self.sequence = 0
        super().__init__(("127.0.0.1", port), Handler)

    def event(self, **fields):
        with self.lock:
            self.sequence += 1
            row = dict(seq=self.sequence, utc_ns=time.time_ns(), mono_ns=time.monotonic_ns(), **fields)
            if self.log:
                self.log.write(json.dumps(row) + "\n")
                self.log.flush()


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_HEAD(self):
        self.serve(False)

    def do_GET(self):
        self.serve(True)

    def serve(self, body):
        u = urllib.parse.urlsplit(self.path)
        if u.path == "/__mark":
            label = urllib.parse.parse_qs(u.query).get("label", [""])[0][:80]
            self.server.event(event="mark", label=label)
            self.send_response(204)
            self.end_headers()
            return
        path = (self.server.root / urllib.parse.unquote(u.path).lstrip("/")).resolve()
        if not path.is_relative_to(self.server.root) or not path.is_file():
            self.send_error(404)
            return
        size = path.stat().st_size
        start, end, status = 0, size - 1, 200
        value = self.headers.get("Range", "")
        if value:
            m = re.fullmatch(r"bytes=(\d*)-(\d*)", value)
            if not m or not any(m.groups()):
                self.send_error(416)
                return
            a, b = m.groups()
            if a:
                start = int(a)
                end = min(int(b), end) if b else end
            else:
                start = max(0, size - int(b))
            if start > end or start >= size:
                self.send_error(416)
                return
            status = 206
        self.server.event(event="request", path=u.path, range=value, method=self.command)
        begin = time.monotonic()
        sent, outcome = 0, "complete"
        try:
            time.sleep(self.server.latency)
            self.send_response(status)
            self.send_header("Content-Length", str(end - start + 1))
            self.send_header("Content-Type", "application/vnd.apple.mpegurl" if path.suffix == ".m3u8" else mimetypes.guess_type(path)[0] or "application/octet-stream")
            self.send_header("Accept-Ranges", "bytes")
            self.send_header("Cache-Control", "no-store")
            if status == 206:
                self.send_header("Content-Range", f"bytes {start}-{end}/{size}")
            self.end_headers()
            if body:
                transfer = time.monotonic()
                with path.open("rb") as f:
                    f.seek(start)
                    while sent < end - start + 1:
                        chunk = f.read(min(4096, end - start + 1 - sent))
                        if not chunk:
                            raise OSError("fixture changed during request")
                        if self.server.rate:
                            time.sleep(max(0, (sent + len(chunk)) / self.server.rate - (time.monotonic() - transfer)))
                        self.wfile.write(chunk)
                        self.wfile.flush()
                        sent += len(chunk)
        except (BrokenPipeError, ConnectionResetError):
            outcome = "client-disconnected"
        finally:
            self.server.event(event="response", path=u.path, range=value, status=status, bytes=sent, elapsed_ms=(time.monotonic()-begin)*1000, outcome=outcome)


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("root", type=Path)
    p.add_argument("--latency-ms", type=float, default=0)
    p.add_argument("--kib-s", type=float, default=0, help="per request, not aggregate link bandwidth; 0 = unlimited")
    p.add_argument("--port", type=int, default=8765)
    p.add_argument("--log", type=Path, required=True)
    args = p.parse_args()
    if args.latency_ms < 0 or args.kib_s < 0 or not args.root.is_dir():
        p.error("root must exist; latency and rate must be nonnegative")
    with args.log.open("x", encoding="utf-8") as log:
        server = Server(args.root, args.latency_ms, args.kib_s, log, args.port)
        server.event(event="config", latency_ms=args.latency_ms, kib_s_per_request=args.kib_s)
        print(f"http://127.0.0.1:{server.server_port}/index.m3u8", flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass
        finally:
            server.server_close()
