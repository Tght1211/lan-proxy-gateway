#!/usr/bin/env bash
set -euo pipefail

gateway_stats() {
    "${GATEWAY_BIN:-gateway}" status --json | python3 -c '
import json, pathlib, sys, urllib.parse, urllib.request
status = json.load(sys.stdin)
port = status["ports"]["api"]
base = sys.argv[1] or f"http://127.0.0.1:{port}"
endpoint = urllib.parse.urlparse(base)
if not (endpoint.scheme == "http" and endpoint.hostname in ("127.0.0.1", "localhost", "::1") and endpoint.port == port and not endpoint.username and not endpoint.password and endpoint.path in ("", "/") and not endpoint.query and not endpoint.fragment):
    sys.exit("Management API must match the selected installation and use a loopback HTTP endpoint")
try:
    token = (pathlib.Path(status["config_file"]).parent / "api-token").read_text().strip()
except OSError:
    sys.exit("Management token unavailable; check the selected installation and core status")
if not token:
    sys.exit("Management token is empty")
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None
request = urllib.request.Request(base.rstrip("/") + "/api/stats", headers={"Authorization": "Bearer " + token})
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
with opener.open(request, timeout=15) as response:
    sys.stdout.write(response.read().decode())
' "${GATEWAY_API:-}"
}
