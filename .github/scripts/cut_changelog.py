#!/usr/bin/env python3
"""Promote CHANGELOG Unreleased notes into a dated version section.

auto-pack writes Version.txt from UpdateNotes when a release is cut.
This script does the same job for Keep a Changelog: ## [Unreleased] is the
note bucket, and the release step records ## [x.y.z] - YYYY-MM-DD.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

HEADING = re.compile(r"^## \[(.+?)\](.*)$")
NOTE = re.compile(r"^\s*[-*]\s+\S")
SEMVER = re.compile(r"^\d+\.\d+\.\d+$")


def parse(text: str) -> tuple[list[str], list[dict]]:
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    preamble: list[str] = []
    sections: list[dict] = []
    current: dict | None = None
    for line in text.split("\n"):
        match = HEADING.match(line)
        if match:
            if current is not None:
                sections.append(current)
            current = {
                "title": match.group(1).strip(),
                "suffix": match.group(2).strip(),
                "lines": [],
            }
            continue
        if current is None:
            preamble.append(line)
        else:
            current["lines"].append(line)
    if current is not None:
        sections.append(current)
    return preamble, sections


def has_notes(lines: list[str]) -> bool:
    return any(NOTE.match(line) for line in lines)


def render(preamble: list[str], sections: list[dict]) -> str:
    parts: list[str] = ["\n".join(preamble).rstrip("\n")]
    for section in sections:
        suffix = section["suffix"]
        heading = f"## [{section['title']}]"
        if suffix:
            heading = f"{heading} {suffix}"
        body = "\n".join(section["lines"]).strip("\n")
        block = heading if not body else f"{heading}\n\n{body}"
        parts.append(block)
    return "\n\n".join(part for part in parts if part != "") + "\n"


def section_body(section: dict) -> str:
    return "\n".join(section["lines"]).strip() + "\n"


def plan_release(sections: list[dict], published: set[str], version: str) -> str:
    """Release the package.json version when it is unpublished and has notes."""
    version = version.strip()
    if not SEMVER.fullmatch(version):
        raise ValueError("package.json version must be x.y.z")
    clash = [
        section["title"]
        for section in sections
        if SEMVER.fullmatch(section["title"]) and section["title"] != version and section["title"] not in published
    ]
    if clash:
        raise ValueError(f"CHANGELOG has unpublished {clash[0]}, package.json is {version}")
    if version in published:
        return ""
    unreleased = next((section for section in sections if section["title"] == "Unreleased"), None)
    current = next((section for section in sections if section["title"] == version), None)
    if unreleased and has_notes(unreleased["lines"]):
        return version
    if current and has_notes(current["lines"]):
        return version
    return ""


def promote(sections: list[dict], version: str, release_date: str) -> list[dict]:
    sections = [
        {"title": s["title"], "suffix": s["suffix"], "lines": list(s["lines"])}
        for s in sections
    ]
    unreleased = next((s for s in sections if s["title"] == "Unreleased"), None)
    if unreleased is None:
        unreleased = {"title": "Unreleased", "suffix": "", "lines": []}
        sections.insert(0, unreleased)
    target = next((s for s in sections if s["title"] == version), None)
    notes = list(unreleased["lines"]) if has_notes(unreleased["lines"]) else []
    dated = f"- {release_date}"
    if target is None:
        insert_at = sections.index(unreleased) + 1
        sections.insert(
            insert_at,
            {"title": version, "suffix": dated, "lines": notes},
        )
        if notes:
            unreleased["lines"] = []
        return sections
    if not has_notes(target["lines"]) and notes:
        target["lines"] = notes
        unreleased["lines"] = []
    if not target["suffix"]:
        target["suffix"] = dated
    return sections


def notes_for(sections: list[dict], version: str) -> str:
    target = next((s for s in sections if s["title"] == version), None)
    if target is None:
        return f"v{version}\n"
    body = section_body(target).strip()
    if not body:
        return f"v{version}\n"
    return body + "\n"


def published_set(raw: str) -> set[str]:
    found: set[str] = set()
    for item in raw.split(","):
        item = item.strip()
        if item.startswith("v"):
            item = item[1:]
        if SEMVER.fullmatch(item):
            found.add(item)
    return found


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("plan", "apply"))
    parser.add_argument("--changelog", default="CHANGELOG.md")
    parser.add_argument("--notes", default="release-notes.md")
    parser.add_argument("--published", default="")
    parser.add_argument("--version", default="")
    parser.add_argument("--date", required=True)
    args = parser.parse_args()

    path = Path(args.changelog)
    preamble, sections = parse(path.read_text(encoding="utf-8"))
    version = args.version.strip()
    if args.command == "plan":
        try:
            version = plan_release(sections, published_set(args.published), version)
        except ValueError as exc:
            print(exc, file=sys.stderr)
            return 2
        if not version:
            print("")
            return 0
    if not SEMVER.fullmatch(version):
        if args.command == "plan":
            print("")
            return 0
        print("version must be x.y.z", file=sys.stderr)
        return 1

    updated = promote(sections, version, args.date.strip())
    Path(args.notes).write_text(notes_for(updated, version), encoding="utf-8", newline="\n")
    if args.command == "apply":
        path.write_text(render(preamble, updated), encoding="utf-8", newline="\n")
    print(version)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
