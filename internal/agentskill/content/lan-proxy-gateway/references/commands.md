# Commands and semantics

The examples use `gateway`; substitute the installation path discovered in SKILL.md. Check subcommand `--help` on older installations. Commands operate on the invoking user's gateway configuration. Elevated lifecycle commands must preserve that user identity (normal `sudo` preserves `SUDO_USER`); do not accidentally initialize or manage a separate root configuration.

## Read-only diagnosis

```text
gateway --version
gateway status --json
gateway agent snapshot
gateway routing list --json
gateway service status
```

`agent snapshot` includes `status`, optional `runtime`, and optional `runtime_error`. `status` describes saved settings; `runtime.http_proxy` and `runtime.components` describe applied settings and listener health. Compare them before claiming a setting is active. Active/recent connections are capped at 100 each; `relay.traffic` and `health.history` are removed. Routing, learning and usage arrays are NOT all bounded. Capture and summarize locally before sending them into agent context. A missing runtime or fallback object means unavailable telemetry, not zero learned rules.

The bundled read-only helper uses the supplied executable, calls `agent snapshot` without a shell, and never changes settings:

```text
python3 <skill-dir>/scripts/gateway_inspect.py --gateway <exact-executable>
python3 <skill-dir>/scripts/gateway_inspect.py --gateway <exact-executable> --view learning --state saved --service Google --action proxy --scope domain --page 1 --page-size 50
python3 <skill-dir>/scripts/gateway_inspect.py --gateway <exact-executable> --view learning --state pending --search example
```

Pages are 1-based; sizes are 1–100. Filters combine with AND. Summary service counts are limited to 20 groups (with total group count and truncation flag); request an exact service to inspect its records. Counts and pagination reflect the captured snapshot, which may change between calls. A row cap is not a byte/token cap: bound any unusually long host/group/error strings before quoting them. Full rules must still be retained locally for mutations; displayed pages are not replacement rule lists.

For detailed telemetry, use GET `/api/stats` on `127.0.0.1:<status.ports.api>`, bypassing system/environment proxies. GET `/api/health` reads cached health. GET `/api/nat-diag` performs active network probes: use it for a NAT diagnosis, not as an unrelated first step. Never expose this loopback control API through a tunnel.

Current management requests require `Authorization: Bearer <token>`. The rotating `api-token` file beside `status.config_file` has owner-only permissions; CLI clients read it automatically. Prefer the CLI. If direct API access is needed, load only that token into an HTTP client's memory, disable proxies and redirects to other origins, and never print/log it or put the header in shell arguments. An unauthenticated curl request returning 401 is not evidence that the gateway is down. LAN Basic proxy credentials and management Bearer credentials are separate.

Useful fields: `relay.active`, `relay.recent`, `relay.ingress`, `relay.devices`, `egress_health`, `components`, and `health`. `ingress: http-proxy` identifies explicit HTTP clients; `via_proxy` identifies the chosen upstream route, a different concept. Old cores may omit ingress. Tunnel traffic can appear under the tunnel client's source IP, not the remote phone's real IP.

## Routing / device policy

```text
gateway routing list --json
gateway routing set --rules-json '<complete updated JSON array>'
```

Supported rule types: `domain`, `domain-suffix`, `ip-cidr`, `src-ip`. Actions: `proxy`, `direct`, `reject`. Device (`src-ip`) policy precedes domain routing; within applicable rules preserve priority. Example rule object:

```json
{"type":"domain","value":"example.com","action":"proxy","group":"User rule","learned":false}
```

Read the existing array, construct the minimal edit, retain all other objects and order, and pass JSON as a single subprocess argument (not interpolated shell code). Never run `routing set` without `--rules-json`: its default is an empty array. Keep the original list for rollback; before rollback check for concurrent edits and avoid overwriting unrelated changes. Successful saving alone is insufficient: verify runtime behavior and any reload error.

## LAN manual HTTP proxy

`gateway http-proxy set` reads one JSON object from stdin:

```json
{"enabled":true,"port":17890,"auth":"basic","username":"existing-user"}
```

This is a schema example, not the user's current setting. Always retain actual values from `status.http_proxy` for fields the user did not request to change. `enabled`, `port`, `auth` (`none` or `basic`), and `username` are replacement values, not a partial patch. Omitting `password` retains the stored password; a first-time basic setup needs a password. Pass a new password through stdin only and never echo it. Turning authentication off changes access to every existing tunnel that targets this listener.

Verify the saved and runtime settings agree. For authenticated proxy traffic, HTTPS uses CONNECT with `Proxy-Authorization`; origin `Authorization` is different. Use an HTTP library or curl stdin configuration to keep credentials out of process arguments. Verify that missing credentials yield 407 when basic authentication is enabled. A direct GET to `/proxy.pac` on the listener should succeed without authentication and return PAC JavaScript, never credentials. Forwarded HTTP and CONNECT must still enforce the configured proxy authentication. A TCP tunnel PAC URL should return that external host and port, not the LAN address.

## Lifecycle and egress

```text
gateway start
gateway restart
gateway stop
```

Lifecycle operations can require OS administrator authorization. Report an OS denial and let the user complete authentication; do not request their administrator password in chat. After restart, verify new uptime, component health and an actual proxied request. Existing connections cannot be migrated across a restart.

For a gateway-only egress change, use `gateway egress proxy --type socks5 --host 127.0.0.1 --port <actual-upstream-port>` or `gateway egress direct`. These preserve Mac system proxy and DNS settings. Inspect current values and command help first; do not assume the example type or port matches the user's VPN.

`gateway system-proxy on --type http --host 127.0.0.1 --port <upstream-port>` changes both the host system proxy and gateway egress on macOS. `system-proxy off` also changes host proxy settings. Do not use these for a gateway-only change unless the user authorized both effects. The upstream VPN's local proxy port differs from the LAN sharing port; pointing the gateway's upstream at its own LAN listener creates a loop.

## Export

```text
gateway skill export --output /chosen/path/lan-proxy-gateway-skill.zip
```

The archive contains a `lan-proxy-gateway/` skill directory with static instructions, references and a read-only helper, never local configuration or credentials. Install that whole directory using the target agent's supported skill installer/location; do not install it into the gateway's built-in AI workspace when the user wants an external agent. Export rejects existing files by default; `--force` replaces an export only when requested. Re-export after an app update to receive the current embedded Skill.

## Automatic learning

Inspect `runtime.fallback.strategy`, `settings`, `threshold`, `window_hours`, `candidates`, `learned`, and `ignored`. On current cores the strategy is `direct-first`: unmatched destinations try direct first; direct failure or no initial response can lead to a proxy retry. Only observed proxy response data contributes evidence. Response bytes do not prove successful application behavior. Explicit rules and deny-direct protections take precedence. If the strategy/settings are missing or old, do not interpret old evidence as current proxy-learning evidence or apply current settings blindly.

New saved rules are `type: domain`, `action: proxy`, `learned: true`. Service classification is stored in `group`, for example `自动学习 · YouTube`; generic `自动学习` is unclassified. Retained older `domain-suffix`/`direct` rules remain a separate category. Keep exact-domain versus suffix scope explicit; do not guess root domains by taking the last two labels. Pending and ignored hosts are not saved routing rules.

Each operation has TWO positional arguments (an action and one host/settings JSON), not a literal pipe-separated command:

```text
gateway learning accept example.com
gateway learning ignore example.com
gateway learning restore example.com
gateway learning undo example.com
gateway learning configure '<complete current settings JSON with the authorized edit>'
```

- `accept`: requires unexpired observations in the 24-hour window meeting the current `settings.confirmations`/`threshold`, and the host must not be ignored. The default is 1, configurable from 1 to 10; never hardcode three. It creates an exact proxy rule only if current proxy mode and existing domain rules permit it; re-check the result and handle stale evidence or overlapping rules.
- `ignore`: pauses new evidence for that exact host and clears its pending evidence; it does not delete an existing rule or block traffic.
- `restore`: removes the pause; observation resumes, but no rule is immediately written and old evidence is not restored.
- `undo`: removes learned rules whose value exactly equals the host, leaves explicit rules intact, clears evidence and pauses learning. Multiple learned entries with that same value may be removed; inspect all affected scopes first. Undoing a suffix rule can affect subdomains.
- `configure`: REPLACES settings, not a partial patch. Read the complete current settings, preserve unrelated values, re-read before applying, and pass JSON as a single subprocess argument without shell interpolation. Defaults are `enabled: true`, `confirmations: 1`, `auto_save: true`, `direct_wait_seconds: 5`, `proxy_wait_seconds: 5`, `max_direct_wait_seconds: 30`, `cooldown_seconds: 30`, `memory_minutes: 10`. These are defaults, not permission to overwrite user choices. Direct/proxy waits: 1–30 seconds; max direct wait: at least direct wait, at most 60; cooldown: 5–600 seconds; memory: 1–60 minutes. Missing/zero timing values receive defaults, while missing booleans become false. Do not omit fields. There is no separate CLI save-mode field: `auto_save` chooses automatic persistence versus manual acceptance.

These are user-authorized mutations, not actions inferred from telemetry. They require the running loopback API. Verify settings and pending/saved/ignored categories from a fresh snapshot after each action; do not restart the core just to view records. Thousands of saved rules are distinct from the learner's bounded temporary evidence (currently at most 512 candidate hosts and 256 timestamps per host).

## Access modes and measurements

`gateway hotspot status` is read-only. `gateway hotspot enable`, `disable`, and `use-lan` save gateway access settings and require a core restart to apply. They do not turn macOS Internet Sharing on/off. `enable` requires an available sharing interface and enables gateway DNS (port 53 moves to the internal default); preserve/inspect this effect. `disable` stops hotspot takeover while retaining system sharing. `use-lan` returns to manual gateway access. A manual gateway client uses gateway/DNS settings; an HTTP/PAC client uses the explicit listener instead. Do not require all HTTP clients to change their default gateway or DNS.

The overview's exit cards replace the old separate exit menu. Global stability summarizes direct/proxy observations and upstream probes, not multiple independently configured proxy nodes; only one upstream is configured. Use per-exit observations and `health`/`egress_health` separately when identifying a failing route. Saved proxy egress does not prove the upstream is reachable.

## Usage history

`usage_history` in runtime stats contains daily device and ingress upload/download totals, split by recorded proxy/direct routes. Connection destination history remains memory-only. Totals start when this version is enabled, persist across restarts, and are keyed by local calendar date and source IP. Tunnel clients may share one observed source IP. Quarterly totals sum the recorded days in the current local calendar quarter; missing days are not historical measurements. Do not represent this as all traffic on a device.
