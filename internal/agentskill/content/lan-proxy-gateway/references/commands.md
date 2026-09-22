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

`agent snapshot` includes `status`, optional `runtime`, and optional `runtime_error`. `status` describes saved settings; `runtime.http_proxy` and `runtime.components` describe applied settings and listener health. Compare them before claiming a setting is active. Runtime recent/active connections are capped at 100 and history arrays are trimmed for agent context.

For detailed telemetry, use GET `/api/stats` on `127.0.0.1:<status.ports.api>`, bypassing system/environment proxies. GET `/api/health` reads cached health. GET `/api/nat-diag` performs active network probes: use it for a NAT diagnosis, not as an unrelated first step. Never expose this loopback control API through a tunnel.

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

`gateway system-proxy on --type http --host 127.0.0.1 --port <upstream-port>` changes both the host system proxy and gateway egress on macOS. `system-proxy off` also changes host proxy settings. Do not use these for a gateway-only change unless the user authorized both effects. The upstream VPN's local proxy port differs from the LAN sharing port; pointing the gateway's upstream at its own LAN listener creates a loop.

## Export

```text
gateway skill export --output /chosen/path/lan-proxy-gateway-skill.zip
```

The archive contains a `lan-proxy-gateway/` skill directory and static instructions only, never local configuration or credentials. Install that whole directory using the target agent's supported skill installer/location; do not install it into the gateway's built-in AI workspace when the user wants an external agent.

## Rule suggestions and usage history

`gateway learning accept|ignore|restore|undo <host>` manages individual suggestions through the running loopback API. Accept requires at least three observations and preserves existing explicit rules; undo removes only the learned rule for that host. These are user-authorized routing changes, never actions inferred merely from telemetry.

`usage_history` in runtime stats contains daily device and ingress upload/download totals. Connection destination history remains memory-only. Totals start when this version is enabled, persist across restarts, and are keyed by local calendar date and source IP. Tunnel clients may share one observed source IP. Do not represent this as all traffic on a device.
