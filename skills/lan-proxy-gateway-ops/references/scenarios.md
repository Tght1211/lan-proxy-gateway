# Scenarios — headless recipes

All commands are non-interactive. Read state with `--json`, change with `config`/`node`, verify by re-reading. Only `start`/`stop`/`restart` need root.

## Point the gateway at a subscription, then pick a node

```bash
gateway config source --type subscription --url "https://example.com/sub"
gateway config mode rule
sudo gateway start            # needs root: brings up TUN + forwarding
gateway node list             # see groups + nodes (needs running)
gateway node switch "Proxy" "🇺🇸 US-01"
gateway status --json         # verify running + source
```

## Chain behind a local Clash / Verge (no airport subscription)

```bash
gateway config source --type external --server 127.0.0.1 --port 7890 --kind http
gateway config tun on
sudo gateway start
```
The gateway forwards LAN traffic into the local proxy client at 127.0.0.1:7890.

## Route specific domains a certain way

```bash
gateway config rule add proxy DOMAIN-SUFFIX,openai.com
gateway config rule add direct DOMAIN-SUFFIX,internal.corp
gateway config rule list --json          # find the index to remove
gateway config rule rm proxy 0
```
Rules hot-reload immediately if the gateway is running.

## Local-machine bypass (share LAN, don't proxy this host)

Keep TUN for LAN devices but avoid capturing the host's own traffic — set the
gateway to forward mode (host traffic untouched, only forwarded LAN traffic is proxied):

```bash
gateway config gateway-mode forward      # restarts mihomo
gateway status --json                     # confirm gateway_mode + running
```

## Quick health / recovery check

```bash
gateway status --json                     # running? mode? source?
gateway config show --json                # source url, rules, tun, dns
# if "Running": false →
sudo gateway start
```

## First-time setup on a fresh machine

```bash
gateway install                           # downloads mihomo + GeoIP, guided setup
gateway config source --type subscription --url "https://example.com/sub"
sudo gateway start
gateway status --json
```

## Privilege handling

- `status`, `config *`, `node *` → no root, run freely.
- `start` / `stop` / `restart` → root. With passwordless sudo run `sudo gateway start`; otherwise ask the user to run it. Don't pipe a sudo password.
