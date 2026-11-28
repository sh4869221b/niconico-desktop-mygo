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

    def __init__(self, root, latency_ms=0, kib_s=0, log=None, port=8765, delay_from_segment=None):
        self.root = Path(root).resolve()
        self.latency = latency_ms / 1000
        self.rate = kib_s * 1024
        self.delay_from_segment = delay_from_segment
        self.log = log
        self.lock = threading.Lock()
        self.sequence = 0
        super().__init__(("127.0.0.1", port), Handler)

    def limits(self, filename):
        if self.delay_from_segment is None:
            return self.latency, self.rate
        segment = re.fullmatch(r"seg(\d+)\.ts", filename)
        if segment and int(segment.group(1)) >= self.delay_from_segment:
            return self.latency, self.rate
        return 0, 0

    def event(self, **fields):
        with self.lock:
            self.sequence += 1
            row = dict(seq=self.sequence, utc_ns=time.time_ns(), mono_ns=time.monotonic_ns(), **fields)
            if self.log:
                self.log.write(json.dumps(row) + "\n")
                self.log.flush()
            return self.sequence


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def send_error(self, code, message=None, explain=None):
        if hasattr(self, "request_id"):
            self.server.event(event="response", request_id=self.request_id, path=self.media_path, status=code, bytes=0, elapsed_ms=(time.monotonic()-self.started)*1000, outcome="http-error")
        super().send_error(code, message, explain)

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
        self.started = time.monotonic()
        self.media_path = u.path
        self.request_id = self.server.event(event="request", path=u.path, range=self.headers.get("Range", ""), method=self.command)
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
        begin = time.monotonic()
        sent, outcome = 0, "complete"
        latency, rate = self.server.limits(path.name)
        try:
            time.sleep(latency)
            self.send_response(status)
            self.send_header("Content-Length", str(end - start + 1))
            content_type = {".m3u8": "application/vnd.apple.mpegurl", ".ts": "video/mp2t"}.get(path.suffix)
            self.send_header("Content-Type", content_type or mimetypes.guess_type(path)[0] or "application/octet-stream")
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
                        if rate:
                            time.sleep(max(0, (sent + len(chunk)) / rate - (time.monotonic() - transfer)))
                        self.wfile.write(chunk)
                        self.wfile.flush()
                        sent += len(chunk)
        except (BrokenPipeError, ConnectionResetError):
            outcome = "client-disconnected"
        finally:
            self.server.event(event="response", request_id=self.request_id, path=u.path, range=value, status=status,
                              bytes=sent, elapsed_ms=(time.monotonic()-begin)*1000, outcome=outcome,
                              applied_latency_ms=latency*1000, applied_kib_s=rate/1024)


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("root", type=Path)
    p.add_argument("--latency-ms", type=float, default=0)
    p.add_argument("--kib-s", type=float, default=0, help="per request, not aggregate link bandwidth; 0 = unlimited")
    p.add_argument("--port", type=int, default=8765)
    p.add_argument("--log", type=Path, required=True)
    p.add_argument("--delay-from-segment", type=int,
                   help="apply delays and rate only to segNNN.ts at this index or later")
    args = p.parse_args()
    if (args.latency_ms < 0 or args.kib_s < 0 or not args.root.is_dir()
            or args.delay_from_segment is not None and args.delay_from_segment < 0):
        p.error("root must exist; latency, rate and segment index must be nonnegative")
    with args.log.open("x", encoding="utf-8") as log:
        server = Server(args.root, args.latency_ms, args.kib_s, log, args.port, args.delay_from_segment)
        server.event(event="config", latency_ms=args.latency_ms, kib_s_per_request=args.kib_s,
                     delay_from_segment=args.delay_from_segment)
        print(f"http://127.0.0.1:{server.server_port}/index.m3u8", flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass
        finally:
            server.server_close()
