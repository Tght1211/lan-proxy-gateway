import argparse
import json
import os
import subprocess
import sys
from collections import Counter


def pick_fields(source, names):
    return {name: source[name] for name in names if name in source}


def service_name(value):
    value = value.strip()
    if not value or "." in value or value in {"未知目标", "未解析域名", "未识别流量", "IP 地址流量"}:
        return None
    return value


def learning_records(runtime):
    fallback = runtime.get("fallback")
    if not isinstance(fallback, dict):
        return None
    services = {}
    relay = runtime.get("relay") or {}
    for connection in (relay.get("active") or []) + (relay.get("recent") or []):
        host = connection.get("dst_host", "").lower().strip(".")
        service = service_name(connection.get("service", ""))
        if service and host not in services:
            services[host] = service
    records = []
    for state, items in (("pending", fallback.get("candidates")),
                         ("saved", fallback.get("learned")),
                         ("ignored", fallback.get("ignored"))):
        for item in items or []:
            host = item if state == "ignored" else item.get("value" if state == "saved" else "host", "")
            group = item.get("group", "") if state == "saved" else ""
            prefix = "自动学习 · "
            service = service_name(group[len(prefix):]) if group.startswith(prefix) else None
            record = {"state": state, "host": host, "service": service or services.get(host.lower().strip(".")) or "未分类"}
            if state == "saved":
                record.update(pick_fields(item, ("type", "action", "group", "learned")))
            elif state == "pending":
                record.update(pick_fields(item, ("count", "last_at")))
            records.append(record)
    return records


def learning_summary(runtime, records):
    if records is None:
        return {"available": False}
    fallback = runtime["fallback"]
    states = {state: 0 for state in ("pending", "saved", "ignored")}
    groups = Counter((record["state"], record["service"]) for record in records)
    for record in records:
        states[record["state"]] += 1
    categories = [{"state": state, "service": service, "count": count}
                  for (state, service), count in sorted(groups.items())]
    return {"available": True, **pick_fields(fallback, ("strategy", "settings", "threshold", "window_hours")),
            "counts": states, "categories": categories[:20], "category_count": len(categories),
            "categories_truncated": len(categories) > 20}


def bounded_snapshot(snapshot, *, view="summary", state="saved", service="", action="", scope="", search="", page=1, page_size=50):
    status = snapshot.get("status") or {}
    rules = status.get("routing") or []
    output = {"version": snapshot.get("version"), "runtime_available": isinstance(snapshot.get("runtime"), dict),
              "status": pick_fields(status, ("configured", "running", "access_mode", "egress", "proxy", "dns", "ports", "ipv6_policy", "proxy_fail_action")),
              "routing": {"count": len(rules), "learned_count": sum(bool(rule.get("learned")) for rule in rules),
                          "by_action": dict(Counter(rule.get("action", "unknown") for rule in rules))}}
    output["status"]["http_proxy"] = pick_fields(status.get("http_proxy") or {}, ("enabled", "port", "auth", "password_set"))
    if "runtime_error" in snapshot:
        output["runtime_error"] = snapshot["runtime_error"]
    runtime = snapshot.get("runtime")
    if not isinstance(runtime, dict):
        output["learning"] = {"available": False}
        return output
    relay = runtime.get("relay") or {}
    output["runtime"] = {**pick_fields(runtime, ("schema_version", "uptime_sec", "egress", "proxy")),
                         "http_proxy": pick_fields(runtime.get("http_proxy") or {}, ("enabled", "port", "auth", "password_set")),
                         "health": pick_fields(runtime.get("health") or {}, ("target", "checked_at", "healthy", "latency_ms", "jitter_ms", "availability", "fail_count")),
                         "relay": {**pick_fields(relay, ("up_total", "down_total")),
                                   "active_sample_count": len(relay.get("active") or []),
                                   "recent_sample_count": len(relay.get("recent") or [])},
                         "usage_day_count": len(runtime.get("usage_history") or [])}
    if isinstance(runtime.get("exit_health"), dict):
        output["runtime"]["exit_health"] = {
            exit_name: pick_fields(runtime["exit_health"][exit_name], ("target", "checked_at", "healthy", "latency_ms", "jitter_ms", "availability", "fail_count"))
            for exit_name in ("proxy", "direct") if isinstance(runtime["exit_health"].get(exit_name), dict)
        }
    records = learning_records(runtime)
    output["learning"] = learning_summary(runtime, records)
    if view != "learning" or records is None:
        return output
    query = search.strip().lower()
    rows = [record for record in records if record["state"] == state]
    matches = [record for record in rows if (not service or record["service"] == service)
               and (not action or record.get("action") == action)
               and (not scope or record.get("type") == scope)
               and (not query or query in " ".join(str(record.get(key, "")) for key in ("host", "service", "group", "type", "action")).lower())]
    if state == "saved":
        matches.sort(key=lambda record: (record["service"] == "未分类", record["service"], record["host"], record.get("type", ""), record.get("action", "")))
    elif state == "ignored":
        matches.sort(key=lambda record: record["host"])
    page_size = min(100, max(1, page_size))
    page_count = max(1, (len(matches) + page_size - 1) // page_size)
    page = min(max(1, page), page_count)
    start = (page - 1) * page_size
    output["learning"]["page"] = {"state": state, "total": len(rows), "matched": len(matches),
                                  "number": page, "page_count": page_count, "page_size": page_size,
                                  "records": matches[start:start + page_size]}
    return output


def main():
    parser = argparse.ArgumentParser(description="Read-only, bounded LAN Proxy Gateway snapshot and learning inspection.")
    parser.add_argument("--gateway", required=True, help="Exact executable for the intended installation")
    parser.add_argument("--view", choices=("summary", "learning"), default="summary")
    parser.add_argument("--state", choices=("pending", "saved", "ignored"), default="saved")
    for name in ("service", "action", "scope", "search"):
        parser.add_argument("--" + name, default="")
    parser.add_argument("--page", type=int, default=1)
    parser.add_argument("--page-size", type=int, default=50)
    args = parser.parse_args()
    if args.page < 1 or not 1 <= args.page_size <= 100:
        parser.error("--page must be at least 1; --page-size must be 1–100")
    if args.state != "saved" and (args.action or args.scope):
        parser.error("--action and --scope apply only to saved routing rules")
    environment = {name: value for name, value in os.environ.items() if name.lower() not in {"http_proxy", "https_proxy", "all_proxy"}}
    environment["NO_PROXY"] = "127.0.0.1,localhost,::1"
    try:
        result = subprocess.run([args.gateway, "agent", "snapshot"], capture_output=True, text=True, timeout=30, env=environment)
        if result.returncode:
            raise ValueError("Gateway snapshot command failed; verify executable/version and inspect its error locally.")
        snapshot = json.loads(result.stdout)
        if not isinstance(snapshot, dict):
            raise ValueError("Snapshot is not a JSON object.")
        output = bounded_snapshot(snapshot, **{name: getattr(args, name) for name in ("view", "state", "service", "action", "scope", "search", "page", "page_size")})
    except (OSError, subprocess.TimeoutExpired, ValueError) as error:
        print("Inspection failed: " + (str(error) if not isinstance(error, json.JSONDecodeError) else "Invalid snapshot JSON."), file=sys.stderr)
        return 1
    print(json.dumps(output, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
