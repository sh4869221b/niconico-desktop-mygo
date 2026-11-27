import io
import json
import tempfile
import threading
import unittest
import urllib.error
import urllib.request
from pathlib import Path
from hls_server import Server


class FixtureTest(unittest.TestCase):
    def test_http_evidence_ranges_and_root_boundary(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d) / "root"
            root.mkdir()
            (root / "index.m3u8").write_bytes(b"#EXTM3U\n")
            (root / "seg.ts").write_bytes(b"0123456789")
            (Path(d) / "secret").write_text("outside")
            log = io.StringIO()
            server = Server(root, log=log, port=0)
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            base = f"http://127.0.0.1:{server.server_port}"
            try:
                for value, expected in [("bytes=2-4", b"234"), ("bytes=-3", b"789"), ("bytes=7-", b"789")]:
                    req = urllib.request.Request(base + "/seg.ts", headers={"Range": value})
                    with urllib.request.urlopen(req) as res:
                        self.assertEqual(res.status, 206)
                        self.assertEqual(res.read(), expected)
                        self.assertIn("/10", res.headers["Content-Range"])
                for path in ["/%2e%2e/secret", "/missing"]:
                    with self.assertRaises(urllib.error.HTTPError) as error:
                        urllib.request.urlopen(base + path)
                    self.assertEqual(error.exception.code, 404)
                with urllib.request.urlopen(base + "/__mark?label=drag-1") as res:
                    self.assertEqual(res.status, 204)
            finally:
                server.shutdown()
                thread.join()
                server.server_close()
            rows = [json.loads(line) for line in log.getvalue().splitlines()]
            self.assertEqual(sum(r["event"] == "request" for r in rows), 5)
            self.assertEqual(sum(r.get("bytes", 0) for r in rows), 9)
            self.assertEqual(sum(r.get("status") == 404 for r in rows), 2)
            self.assertEqual({r["seq"] for r in rows if r["event"] == "request"}, {r["request_id"] for r in rows if r["event"] == "response"})
            self.assertTrue(any(r.get("label") == "drag-1" for r in rows))
