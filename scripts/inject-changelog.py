#!/usr/bin/env python3
"""Render main's CHANGELOG.md into the site.

Run by the Pages workflow after checking main's changelog out into .main/.
Keeping the changelog on main and rendering it here means a release is written
down in exactly one place. Two things come out of it:

  versions.md         the marker is replaced by the release sections, with a
                      stable anchor and a tag link on each heading
  _data/versions.json the release list, which the layout turns into the
                      version menu in the header

Neither output is committed; both are regenerated on every build.
"""

from __future__ import annotations

import json
import pathlib
import re
import sys

MARKER = "<!-- CHANGELOG -->"
SITE = pathlib.Path(__file__).resolve().parent.parent
CHANGELOG = SITE / ".main" / "CHANGELOG.md"
VERSIONS = SITE / "versions.md"
DATA = SITE / "_data" / "versions.json"

REPO = "https://github.com/tarakm89/go-cli-go-template"

# `## [1.2.3] - 2026-01-01` or `## [Unreleased]`
HEADING = re.compile(r"^## \[([^\]]+)\](.*)$", re.MULTILINE)


def body_of(changelog: str) -> str:
    """Drop the changelog's H1 and preamble, keeping the releases."""
    match = re.search(r"^## ", changelog, flags=re.MULTILINE)
    return changelog[match.start():] if match else changelog


def anchor_for(name: str) -> str:
    if name.lower() == "unreleased":
        return "unreleased"
    return "release-" + re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")


def parse(body: str) -> list[dict]:
    """Read the release headings in the order the changelog lists them."""
    releases = []
    for match in HEADING.finditer(body):
        name = match.group(1)
        unreleased = name.lower() == "unreleased"
        # ` - 2026-08-27` after the link, when there is one.
        date = re.sub(r"^\s*[-–—]\s*", "", match.group(2)).strip()
        releases.append({
            "name": "Unreleased" if unreleased else name,
            "date": date,
            "anchor": anchor_for(name),
            "unreleased": unreleased,
            "tag": None if unreleased else f"v{name}",
        })
    return releases


def add_links(body: str, releases: list[dict]) -> str:
    """Link each heading to its tag and give it a stable anchor.

    Kramdown reads the trailing `{#id}` as the heading's id, which is what the
    version menu and the table of contents both link to.
    """
    ordered = iter(releases)

    def replace(match: re.Match[str]) -> str:
        release = next(ordered)
        rest, anchor = match.group(2), release["anchor"]
        if release["unreleased"]:
            target = release["compare"] or f"{REPO}/commits/main"
            return f"## [Unreleased]({target}){rest} {{#{anchor}}}"
        return f"## [{release['name']}]({REPO}/releases/tag/{release['tag']}){rest} {{#{anchor}}}"

    return HEADING.sub(replace, body)


def add_compare_urls(releases: list[dict]) -> None:
    """Point each release at the diff against the one before it."""
    tagged = [r for r in releases if not r["unreleased"]]
    latest = tagged[0]["tag"] if tagged else None

    for release in releases:
        if release["unreleased"]:
            release["compare"] = f"{REPO}/compare/{latest}...main" if latest else None
            continue
        index = tagged.index(release)
        previous = tagged[index + 1]["tag"] if index + 1 < len(tagged) else None
        release["compare"] = (
            f"{REPO}/compare/{previous}...{release['tag']}" if previous
            else f"{REPO}/releases/tag/{release['tag']}"
        )
        release["tag_url"] = f"{REPO}/releases/tag/{release['tag']}"


def main() -> int:
    if not CHANGELOG.is_file():
        print(f"error: {CHANGELOG} not found; was main checked out?", file=sys.stderr)
        return 1

    page = VERSIONS.read_text(encoding="utf-8")
    if MARKER not in page:
        print(f"error: {VERSIONS} has no {MARKER} marker", file=sys.stderr)
        return 1

    body = body_of(CHANGELOG.read_text(encoding="utf-8"))
    releases = parse(body)
    add_compare_urls(releases)
    body = add_links(body, releases)

    # The changelog talks about `{{cookiecutter.project_slug}}`, which Liquid
    # would try to evaluate. Hand it to Jekyll as raw text.
    VERSIONS.write_text(
        page.replace(MARKER, "{% raw %}\n" + body.strip() + "\n{% endraw %}"),
        encoding="utf-8",
    )

    tagged = [r for r in releases if not r["unreleased"]]
    DATA.parent.mkdir(parents=True, exist_ok=True)
    DATA.write_text(json.dumps({
        "latest": tagged[0]["name"] if tagged else None,
        "latest_tag": tagged[0]["tag"] if tagged else None,
        "releases": releases,
    }, indent=2) + "\n", encoding="utf-8")

    print(f"injected {len(releases)} sections into versions.md "
          f"({len(tagged)} tagged); wrote {DATA.relative_to(SITE)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
