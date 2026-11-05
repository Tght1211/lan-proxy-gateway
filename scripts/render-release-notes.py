#!/usr/bin/env python3
"""Prepend the beginner download guide, validating links against actual assets."""
import argparse
import json
from pathlib import Path
import re
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[1]


def render_notes(tag: str, repo: str, notes: str, assets: set[str]) -> str:
    if not re.fullmatch(r"v[0-9][A-Za-z0-9._-]*", tag):
        raise ValueError(f"Invalid release tag: {tag!r}")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise ValueError(f"Invalid repository: {repo!r}")
    if not re.search(r"^# " + re.escape(tag) + r"(?:\s|$)", notes, re.MULTILINE):
        raise ValueError("Release notes heading does not match the tag; update latest.md first")
    template = (ROOT / "docs/releases/download-guide.md").read_text()
    guide = template.replace("{{TAG}}", tag)
    linked_assets = re.findall(r"\{\{BASE\}\}/([^\s)]+)", guide)
    missing = sorted(set(linked_assets) - assets)
    if missing:
        raise ValueError("Download guide references missing assets: " + ", ".join(missing))
    repo_url = f"https://github.com/{repo}"
    base = f"{repo_url}/releases/download/{quote(tag, safe='')}"
    guide = guide.replace("{{BASE}}", base).replace("{{REPO_URL}}", repo_url)
    if "{{" in guide:
        raise ValueError("Unresolved placeholder in download guide")
    return guide.rstrip() + "\n\n" + notes.strip() + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--notes", type=Path, default=ROOT / "docs/releases/latest.md")
    parser.add_argument("--output", type=Path, required=True)
    sources = parser.add_mutually_exclusive_group()
    sources.add_argument("--assets-dir", type=Path, default=ROOT / "dist")
    sources.add_argument("--release-json", type=Path, help="gh release view --json tagName,body,assets output")
    args = parser.parse_args()
    if args.release_json:
        release = json.loads(args.release_json.read_text())
        if release["tagName"] != args.tag:
            parser.error("Release JSON tag does not match --tag")
        assets = {asset["name"] for asset in release["assets"]}
    else:
        assets = {p.name for p in args.assets_dir.iterdir() if p.is_file()}
    result = render_notes(args.tag, args.repo, args.notes.read_text(), assets)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(result)


if __name__ == "__main__":
    main()
