#!/usr/bin/env python3
"""Splice main's CHANGELOG.md into versions.md, replacing the marker.

Run by the Pages workflow after checking main's changelog out into .main/.
Keeping the changelog on main and rendering it here means a release is written
down in exactly one place.
"""

from __future__ import annotations

import pathlib
import re
import sys

MARKER = "<!-- CHANGELOG -->"
SITE = pathlib.Path(__file__).resolve().parent.parent
CHANGELOG = SITE / ".main" / "CHANGELOG.md"
VERSIONS = SITE / "versions.md"

REPO = "https://github.com/tarakm89/go-cli-go-template"


def body_of(changelog: str) -> str:
    """Drop the changelog's H1 and its preamble, keeping the releases."""
    match = re.search(r"^## ", changelog, flags=re.MULTILINE)
    return changelog[match.start():] if match else changelog


def latest_release(body: str) -> str | None:
    """The most recent numbered version, ignoring an Unreleased section."""
    for version in re.findall(r"^## \[([^\]]+)\]", body, flags=re.MULTILINE):
        if version.lower() != "unreleased":
            return version
    return None


def link_releases(body: str) -> str:
    """Turn `## [1.2.3] - date` into a heading that links to its tag.

    Unreleased points at the diff since the last tag, which is the thing
    somebody reading that section actually wants.
    """
    latest = latest_release(body)

    def replace(match: re.Match[str]) -> str:
        version, rest = match.group(1), match.group(2)
        if version.lower() == "unreleased":
            if latest is None:
                return f"## Unreleased{rest}"
            return f"## [Unreleased]({REPO}/compare/v{latest}...main){rest}"
        return f"## [{version}]({REPO}/releases/tag/v{version}){rest}"

    return re.sub(r"^## \[([^\]]+)\](.*)$", replace, body, flags=re.MULTILINE)


def main() -> int:
    if not CHANGELOG.is_file():
        print(f"error: {CHANGELOG} not found; was main checked out?", file=sys.stderr)
        return 1

    page = VERSIONS.read_text(encoding="utf-8")
    if MARKER not in page:
        print(f"error: {VERSIONS} has no {MARKER} marker", file=sys.stderr)
        return 1

    body = link_releases(body_of(CHANGELOG.read_text(encoding="utf-8")))

    # The changelog talks about `{{cookiecutter.project_slug}}`, which Liquid
    # would try to evaluate. Hand it to Jekyll as raw text.
    rendered = "{% raw %}\n" + body.strip() + "\n{% endraw %}"

    VERSIONS.write_text(page.replace(MARKER, rendered), encoding="utf-8")
    releases = len(re.findall(r"^## ", body, flags=re.MULTILINE))
    print(f"injected {releases} release sections into versions.md")
    return 0


if __name__ == "__main__":
    sys.exit(main())
