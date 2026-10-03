import importlib.util
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest


SCRIPT = Path(__file__).parent / "content/lan-proxy-gateway/scripts/gateway_inspect.py"
spec = importlib.util.spec_from_file_location("gateway_inspect", SCRIPT)
inspection = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inspection)


class InspectTests(unittest.TestCase):
    def test_exit_health_summaries_remain_independent_and_bounded(self):
        runtime = {"egress": "proxy", "health": {"healthy": False},
                   "exit_health": {"direct": {"target": "www.baidu.com:80", "healthy": True, "latency_ms": 12,
                                              "history": [{}] * 180},
                                   "proxy": {"healthy": False, "fail_count": 2}, "unexpected": {"secret": "omit"}}}
        result = inspection.bounded_snapshot({"runtime": runtime})["runtime"]
        self.assertFalse(result["health"]["healthy"])
        self.assertTrue(result["exit_health"]["direct"]["healthy"])
        self.assertEqual(result["exit_health"]["direct"]["target"], "www.baidu.com:80")
        self.assertNotIn("history", result["exit_health"]["direct"])
        self.assertEqual(set(result["exit_health"]), {"proxy", "direct"})
        self.assertNotIn("exit_health", inspection.bounded_snapshot({"runtime": {}})["runtime"])

    def test_ten_thousand_rules_are_bounded_without_losing_records(self):
        rules = [{"type": "domain", "value": f"domain-{number:05d}.test", "action": "proxy",
                  "group": "自动学习 · Google", "learned": True} for number in range(10_000)]
        snapshot = {"version": "test", "status": {"routing": rules}, "runtime": {"fallback": {"learned": rules}}}
        summary = inspection.bounded_snapshot(snapshot)
        self.assertEqual(summary["routing"]["count"], 10_000)
        self.assertEqual(summary["learning"]["counts"]["saved"], 10_000)
        self.assertNotIn("records", json.dumps(summary))
        hosts = []
        for number in range(1, 101):
            result = inspection.bounded_snapshot(snapshot, view="learning", page=number, page_size=100)["learning"]["page"]
            self.assertEqual(result["page_count"], 100)
            self.assertEqual(len(result["records"]), 100)
            hosts.extend(record["host"] for record in result["records"])
        self.assertEqual(len(hosts), 10_000)
        self.assertEqual(len(set(hosts)), 10_000)
        result = inspection.bounded_snapshot(snapshot, view="learning", search=" DOMAIN-09999 ", service="Google", action="proxy", scope="domain", page=999)
        self.assertEqual(result["learning"]["page"]["matched"], 1)
        self.assertEqual(result["learning"]["page"]["number"], 1)

    def test_categories_scope_routes_and_duplicates_remain_distinct(self):
        rules = [{"type": "domain", "value": "shared.test", "action": "proxy", "group": "自动学习 · Google"},
                 {"type": "domain-suffix", "value": "shared.test", "action": "direct", "group": "自动学习 · Google"},
                 {"type": "ip-cidr", "value": "192.0.2.0/24", "action": "reject", "group": "custom"},
                 {"type": "domain", "value": "unknown.co.uk", "action": "proxy", "group": "自动学习"}]
        rules.append(dict(rules[0]))
        runtime = {"fallback": {"strategy": "direct-first", "threshold": 1, "learned": rules,
                                 "candidates": [{"host": "pending.test", "count": 2}], "ignored": ["paused.test"]},
                   "relay": {"active": [{"dst_host": "shared.test", "service": "Wrong live label"},
                                         {"dst_host": "unknown.co.uk", "service": "unknown.co.uk"},
                                         {"dst_host": "PENDING.TEST.", "service": "Google"}]}}
        snapshot = {"runtime": runtime}
        result = inspection.bounded_snapshot(snapshot, view="learning", state="saved", service="Google", action="direct", scope="domain-suffix")
        self.assertEqual(result["learning"]["counts"], {"pending": 1, "saved": 5, "ignored": 1})
        self.assertEqual(result["learning"]["page"]["matched"], 1)
        self.assertEqual(result["learning"]["page"]["records"][0]["action"], "direct")
        records = inspection.learning_records(runtime)
        self.assertEqual(next(record for record in records if record["host"] == "unknown.co.uk")["service"], "未分类")
        self.assertEqual(next(record for record in records if record["host"] == "pending.test")["service"], "Google")
        self.assertNotIn("action", next(record for record in records if record["state"] == "ignored"))
        self.assertEqual(len([record for record in records if record["host"] == "shared.test"]), 3)

    def test_unavailable_runtime_is_not_an_empty_history(self):
        result = inspection.bounded_snapshot({"status": {"running": False}, "runtime_error": "core unavailable"}, view="learning")
        self.assertFalse(result["runtime_available"])
        self.assertFalse(result["learning"]["available"])
        self.assertNotIn("counts", result["learning"])
        self.assertEqual(result["runtime_error"], "core unavailable")
        result = inspection.bounded_snapshot({"runtime": {}})
        self.assertFalse(result["learning"]["available"])
        result = inspection.bounded_snapshot({"runtime": {"fallback": {}}}, view="learning", search="missing")
        self.assertEqual(result["learning"]["page"]["records"], [])

    def test_category_summary_is_bounded(self):
        rules = [{"value": f"domain-{number}.test", "group": f"自动学习 · Service {number}", "action": "proxy"} for number in range(30)]
        result = inspection.bounded_snapshot({"runtime": {"fallback": {"learned": rules}}})
        self.assertEqual(len(result["learning"]["categories"]), 20)
        self.assertEqual(result["learning"]["category_count"], 30)
        self.assertTrue(result["learning"]["categories_truncated"])

    def test_actual_helper_uses_only_exact_read_command(self):
        with tempfile.TemporaryDirectory() as directory:
            executable = Path(directory) / "fake gateway"
            executable.write_text("#!" + sys.executable + "\nimport json, os, sys\n"
                                  "assert sys.argv[1:] == ['agent', 'snapshot']\n"
                                  "assert not any(key.lower() in {'http_proxy', 'https_proxy', 'all_proxy'} for key in os.environ)\n"
                                  "print(json.dumps({'status': {'running': False, 'http_proxy': {'password': 'DO-NOT-PRINT', 'username': 'PRIVATE'}}, 'runtime_error': 'unavailable'}))\n")
            executable.chmod(0o700)
            result = subprocess.run([sys.executable, "-B", str(SCRIPT), "--gateway", str(executable)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertNotIn("DO-NOT-PRINT", result.stdout)
            self.assertNotIn("PRIVATE", result.stdout)
            self.assertFalse(json.loads(result.stdout)["learning"]["available"])
            invalid = subprocess.run([sys.executable, "-B", str(SCRIPT), "--gateway", str(executable), "--page-size", "101"], capture_output=True)
            self.assertNotEqual(invalid.returncode, 0)

    def test_legacy_api_helper_authenticates_without_proxy_or_redirects(self):
        received = []
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                received.append(self.headers.get("Authorization"))
                if len(received) == 1:
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b'{"relay":{},"fallback":{}}')
                else:
                    self.send_response(302)
                    self.send_header("Location", "/unapproved-redirect")
                    self.end_headers()

            def log_message(self, *args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                config = Path(directory) / "gateway.yaml"
                token = "fixture-only-token"
                (Path(directory) / "api-token").write_text(token)
                executable = Path(directory) / "fake gateway"
                status = {"ports": {"api": server.server_port}, "config_file": str(config)}
                executable.write_text("#!" + sys.executable + "\nimport sys\nassert sys.argv[1:] == ['status', '--json']\nprint(" + repr(json.dumps(status)) + ")\n")
                executable.chmod(0o700)
                script = Path(__file__).resolve().parents[2] / "skills/gateway/scripts/api-base.sh"
                environment = dict(os.environ, GATEWAY_BIN=str(executable), GATEWAY_API="", HTTP_PROXY="http://127.0.0.1:1", HTTPS_PROXY="http://127.0.0.1:1")
                command = ["bash", "-c", 'source "$1"; gateway_stats', "test", str(script)]
                result = subprocess.run(command, capture_output=True, text=True, env=environment)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("relay", json.loads(result.stdout))
                self.assertNotIn(token, result.stdout + result.stderr)
                result = subprocess.run(command, capture_output=True, text=True, env=environment)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(received, ["Bearer " + token, "Bearer " + token])
                self.assertNotIn(token, result.stdout + result.stderr)
        finally:
            server.shutdown()
            server.server_close()
            worker.join()


if __name__ == "__main__":
    unittest.main()
