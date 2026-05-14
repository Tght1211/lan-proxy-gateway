# Advanced Guide

[Chinese version](../advanced.md)

## Keep These 4 Config Areas in Mind

The project currently groups its capabilities into 4 areas:

1. `proxy`
   Proxy source: subscription URL, local config file, subscription name
2. `runtime`
   Gateway runtime: ports, API secret, TUN, LAN sharing, local bypass
3. `rules`
   Rule system: mainland direct access, ad blocking, custom rules
4. `extension`
   Extension mode: `chains` or `script`

This helps the project keep its two main strengths clear as it grows:

- LAN sharing
- chained proxying

---

## Chains Mode: The Core AI Account Workflow

Routing overview:

```text
Normal mode: device -> airport node -> AI service
Chains mode: device -> airport node -> residential proxy -> AI service
```

This is especially useful for:

- Claude / ChatGPT signup
- everyday Codex / Cursor usage
- keeping normal traffic on airport nodes while AI traffic goes through a residential exit

### Configure It

```bash
gateway chains
sudo gateway restart
```

The wizard will ask for:

1. residential proxy address and port
2. username and password
3. airport latency-test group name
4. `rule` or `global` routing mode

### Mode Differences

- `rule`
  Normal traffic stays on the airport node. AI-related traffic goes to the residential exit.
- `global`
  All traffic goes to the residential exit.

### Verify It

```bash
gateway status
```

The status page shows:

- entry node
- regular exit
- residential exit

That gives you a direct view of whether the chain is actually working.

---

## Extension Scripts

If you already have a Clash Verge Rev script, or you want more complex custom logic, use `script` mode.

```yaml
extension:
  mode: script
  script_path: /etc/gateway/my-script.js
```

You can also switch directly:

```bash
gateway switch extension script /etc/gateway/my-script.js
sudo gateway restart
```

The repo includes an example script:

```text
./script-demo.js
```

---

## Rule System

Rules are no longer hardcoded inside the template. They are built through a dedicated rule layer. The defaults are tuned around real-world usage patterns common in China:

- LAN and private-network direct access
- WeChat, QQ, and Tencent ecosystem direct access
- Xiaohongshu, Douyin, Toutiao, and other common mainland platforms direct access
- Honor of Kings and other common mainland gaming platforms direct access
- Apple service routing
- Nintendo service proxying
- common ad and tracking domain blocking
- proxy rules for overseas sites and AI services

### Custom Rules

```yaml
rules:
  extra_direct_rules:
    - "DOMAIN-SUFFIX,corp.example.com,DIRECT"
  extra_proxy_rules:
    - "DOMAIN-SUFFIX,example-overseas.com,Proxy"
  extra_reject_rules:
    - "DOMAIN-SUFFIX,annoying-ads.example,REJECT"
```

### Edit Interactively

```bash
gateway config
```

Then go to:

- `Rules and custom entries`

That lets you edit rules without hand-writing YAML.

---

## Local Bypass

A common real-world need is not "make this computer use the proxy too", but:

- let this computer keep using its familiar local network path
- let Switch, PS5, phones, and TVs on the LAN use this computer as a shared gateway

You can do that with:

```yaml
runtime:
  tun:
    enabled: true
    bypass_local: true
```

The recommended path is still:

```bash
gateway config
```

Then open:

- `LAN sharing / TUN / ports`

---

## Normal-Permission Control

Start, stop, and restart are system-level operations, so they usually need root or administrator privileges. To reduce friction for AI clients and everyday users, the project now supports:

```bash
gateway permission print
sudo gateway permission install
gateway permission status
```

After setup, the CLI can try `sudo -n` automatically instead of requiring users to manually rewrite every command with `sudo`.

---

## Runtime Menu Console

Run `gateway start`, or use `gateway console` after the gateway is already running, to enter the runtime menu-driven CLI console.

From there you can:

- view full runtime status and the current config summary
- switch proxy groups and nodes, with latency retest and sorting
- manage subscriptions, proxy source, TUN, local bypass, rules, and extension mode
- open the full config center
- read logs, device setup notes, and upgrade hints

This menu console is now the default interaction path; the old `--tui` entry has been removed.

---

## AI Skill

Show skill info with:

```bash
gateway skill
gateway skill path
```

The repository already includes a scenario-oriented skill so AI clients can operate the system through workflows rather than forcing users to memorize every command and config detail.

Recommended scenarios:

- first-time setup on a new machine
- LAN sharing troubleshooting
- chains setup and verification
- node switching
- local bypass toggling
- health checks and log diagnosis

---

## Automatic Updates

The project supports two upgrade paths:

1. manual upgrade

```bash
sudo gateway update
sudo gateway update v3.4.3
```

2. passive reminders

- `gateway`
- `gateway start`
- the runtime menu console

When a new version is available, they can show an update hint. The check is cached internally so the CLI does not block on every run.

---

## Services and Long-Running Stability

After installing the system service, the gateway can start automatically after reboot:

```bash
sudo gateway service install
```

This is a good fit for:

- an always-on Mac mini or Linux mini PC at home
- a lightweight soft-router alternative
- long-term transparent proxying for the whole home
