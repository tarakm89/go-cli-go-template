#!/usr/bin/env python3
"""Render docs/ into the documentation site's branch.

Run by .github/workflows/publish-docs.yml with the gh_pages branch checked out
into --site. For every Markdown file under docs/ it writes a page with Jekyll
front matter, and it records each document's history in _data/docs.json so the
site can show when something last changed and which release carried it.

The source of truth stays on main: nothing here is written by hand, and the
whole output is regenerated on every run.
"""

from __future__ import annotations

import argparse
import json
import pathlib
import re
import shutil
import subprocess
import sys

REPO = "https://github.com/tarakm89/go-cli-go-template"
ROOT = pathlib.Path(__file__).resolve().parent.parent
SOURCE = ROOT / "docs"


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], cwd=ROOT, check=True, capture_output=True, text=True,
    ).stdout.strip()


def tags_newest_first() -> list[str]:
    out = git("tag", "--list", "v*", "--sort=-creatordate")
    return [line for line in out.splitlines() if line]


def title_of(text: str, fallback: str) -> str:
    """The first level-one heading, which becomes the page title."""
    match = re.search(r"^#\s+(.+?)\s*$", text, flags=re.MULTILINE)
    return match.group(1) if match else fallback


def strip_title(text: str) -> str:
    """Remove the leading H1; the layout renders it from the front matter."""
    return re.sub(r"\A\s*#\s+.+?\n+", "", text, count=1)


# Specs and plans open with a short definition list: Status, Date, Author.
META_LINE = re.compile(r"^-\s*\*\*(?P<key>[A-Za-z ]+):\*\*\s*(?P<value>.*?)\s*$")


def meta_of(text: str) -> dict[str, str]:
    """Read the leading `- **Key:** value` block, if there is one."""
    fields: dict[str, str] = {}
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped:
            if fields:
                break
            continue
        if stripped.startswith("#"):
            continue
        match = META_LINE.match(stripped)
        if not match:
            break
        value = match.group("value").strip()
        if value and value not in {"—", "-"}:
            fields[match.group("key").strip().lower()] = value
    return fields


def strip_meta_block(text: str) -> str:
    """Drop that block from the body; the page header shows it instead."""
    lines = text.splitlines()
    out, removing, seen = [], False, False
    for line in lines:
        stripped = line.strip()
        if not seen and META_LINE.match(stripped):
            removing, seen = True, True
            continue
        if removing:
            if not stripped:
                continue
            if META_LINE.match(stripped):
                continue
            removing = False
        out.append(line)
    return "\n".join(out).lstrip("\n")


def role_of(relative: pathlib.Path) -> str:
    """Whether a file is a section landing page, a template, or a document."""
    if relative.name == "README.md":
        return "index"
    if relative.stem.upper() == "TEMPLATE":
        return "template"
    return "doc"


def last_commit(path: pathlib.Path) -> dict | None:
    """The most recent commit that touched this file."""
    out = git("log", "-1", "--format=%H%x1f%h%x1f%aI%x1f%an%x1f%s", "--", str(path))
    if not out:
        return None
    sha, short, date, author, subject = out.split("\x1f")
    return {
        "sha": sha,
        "short_sha": short,
        "date": date[:10],
        "author": author,
        "subject": subject,
        "url": f"{REPO}/commit/{sha}",
    }


def release_for(sha: str, tags: list[str]) -> str | None:
    """The earliest tag that contains this commit, or None if unreleased."""
    contained = git("tag", "--contains", sha, "--list", "v*", "--sort=creatordate")
    for tag in contained.splitlines():
        if tag:
            return tag
    return None


def changed_between(old: str | None, new: str) -> list[dict]:
    """Documents added or modified between two refs."""
    if old is None:
        args = ["show", "--name-status", "--format=", new, "--", "docs"]
    else:
        args = ["diff", "--name-status", f"{old}..{new}", "--", "docs"]

    changes = []
    for line in git(*args).splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        status, path = parts[0][:1], parts[-1]
        if not path.endswith(".md"):
            continue
        changes.append({
            "status": {"A": "added", "M": "updated", "D": "removed",
                       "R": "renamed"}.get(status, status.lower()),
            "path": path,
            "name": pathlib.Path(path).name,
        })
    return sorted(changes, key=lambda c: c["path"])


def nav_id_for(relative: pathlib.Path) -> str:
    """Which top-level section of the site this document belongs to."""
    if relative.parts[0] in ("specs", "plans"):
        return "specification"
    if relative.as_posix() == "architecture.md":
        return "architecture"
    return "specification"


def breadcrumb_section(relative: pathlib.Path) -> str | None:
    """The section a document sits in, for the trail above its title."""
    if relative.as_posix() == "README.md":
        return None
    if relative.parts[0] == "specs":
        return "Specification"
    if relative.parts[0] == "plans":
        return "Plans"
    return "Documentation"


def breadcrumb_url(relative: pathlib.Path) -> str | None:
    if relative.as_posix() == "README.md":
        return None
    if relative.parts[0] == "specs":
        return "/docs/specs/index.html"
    if relative.parts[0] == "plans":
        return "/docs/plans/index.html"
    return "/docs/index.html"


def neighbours_of(relative: pathlib.Path, by_section: dict, root: pathlib.Path) -> dict:
    """The previous and next document in the same section.

    Index pages are skipped: they are a way in, not a step in a sequence.
    """
    section = relative.parts[0] if len(relative.parts) > 1 else "root"
    ordered = [r for r in by_section[section] if r.name != "README.md"]
    if relative.name == "README.md" or relative not in ordered:
        return {"prev": None, "next": None}

    at = ordered.index(relative)

    def entry(other: pathlib.Path | None):
        if other is None:
            return None
        text = (root / other).read_text(encoding="utf-8")
        return (page_url(other), title_of(text, other.stem))

    return {
        "prev": entry(ordered[at - 1] if at > 0 else None),
        "next": entry(ordered[at + 1] if at + 1 < len(ordered) else None),
    }


def page_url(relative: pathlib.Path) -> str:
    """Where this document will live on the site."""
    stem = relative.with_suffix("")
    if stem.name == "README":
        stem = stem.parent / "index"
    return "/docs/" + str(stem).replace("\\", "/") + ".html"


def front_matter(fields: dict) -> str:
    lines = ["---"]
    for key, value in fields.items():
        if value is None:
            continue
        if isinstance(value, bool):
            lines.append(f"{key}: {str(value).lower()}")
        else:
            lines.append(f"{key}: {json.dumps(str(value), ensure_ascii=False)}")
    lines.append("---")
    return "\n".join(lines) + "\n"


def rewrite_links(text: str) -> str:
    """Point in-repo Markdown links at the published pages.

    `[x](specs/0001-a.md)` becomes `[x](0001-a.html)`; a link that leaves docs/
    goes to the file on GitHub instead, since it has no page here.
    """

    def replace(match: re.Match[str]) -> str:
        label, target = match.group(1), match.group(2)
        if re.match(r"^(https?:|mailto:|#)", target):
            return match.group(0)

        path, _, fragment = target.partition("#")
        fragment = f"#{fragment}" if fragment else ""

        if not path.endswith(".md"):
            return match.group(0)
        if path.startswith("../../") or path.startswith("../CHANGELOG"):
            return f"[{label}]({REPO}/blob/main/{path.lstrip('./')}{fragment})"

        stem = path[:-3]
        if stem.endswith("README"):
            stem = stem[: -len("README")] + "index"
        return f"[{label}]({stem}.html{fragment})"

    return re.sub(r"\[([^\]]*)\]\(([^)]+)\)", replace, text)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--site", required=True, help="checkout of the gh_pages branch")
    args = parser.parse_args()

    site = pathlib.Path(args.site).resolve()
    if not site.is_dir():
        print(f"error: {site} is not a directory", file=sys.stderr)
        return 1
    if not SOURCE.is_dir():
        print(f"error: {SOURCE} does not exist", file=sys.stderr)
        return 1

    # This script owns docs/ apart from docs/reference, which sync-reference.py
    # writes. Clearing the whole tree would delete the reference whenever this
    # ran on its own, so remove only what belongs to this script.
    out = site / "docs"
    out.mkdir(parents=True, exist_ok=True)
    for entry in out.iterdir():
        if entry.name == "reference":
            continue
        shutil.rmtree(entry) if entry.is_dir() else entry.unlink()

    tags = tags_newest_first()
    documents = []
    sources = sorted(SOURCE.rglob("*.md"))

    # Neighbours within a section, so a reader can walk the specs in order.
    by_section: dict[str, list[pathlib.Path]] = {}
    for candidate in sources:
        rel = candidate.relative_to(SOURCE)
        by_section.setdefault(rel.parts[0] if len(rel.parts) > 1 else "root", []).append(rel)

    for source in sources:
        relative = source.relative_to(SOURCE)
        text = source.read_text(encoding="utf-8")
        fields = meta_of(text)
        role = role_of(relative)
        commit = last_commit(source.relative_to(ROOT))

        document = {
            "title": title_of(text, relative.stem),
            "path": f"docs/{relative.as_posix()}",
            "url": page_url(relative),
            "section": relative.parts[0] if len(relative.parts) > 1 else "root",
            "status": fields.get("status"),
            "date": fields.get("date"),
            "role": role,
            "listed": role == "doc",
            "commit": commit,
            "release": release_for(commit["sha"], tags) if commit else None,
            "source_url": f"{REPO}/blob/main/docs/{relative.as_posix()}",
            "history_url": f"{REPO}/commits/main/docs/{relative.as_posix()}",
        }
        documents.append(document)

        neighbours = neighbours_of(relative, by_section, SOURCE)

        target = out / relative
        if relative.name == "README.md":
            target = target.with_name("index.md")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(
            front_matter({
                "layout": "doc",
                "nav_id": nav_id_for(relative),
                "title": document["title"],
                "eyebrow": {"specs": "Spec", "plans": "Plan"}.get(document["section"], "Docs"),
                "doc_path": document["path"],
                "doc_status": document["status"],
                "doc_date": document["date"],
                "doc_author": fields.get("author"),
                "doc_supersedes": fields.get("supersedes"),
                "doc_shipped": fields.get("shipped in"),
                "doc_implements": fields.get("implements"),
                "doc_source": document["source_url"],
                "doc_history": document["history_url"],
                "doc_updated": commit["date"] if commit else None,
                "doc_commit": commit["short_sha"] if commit else None,
                "doc_commit_url": commit["url"] if commit else None,
                "doc_subject": commit["subject"] if commit else None,
                "doc_release": document["release"],
                "doc_index": relative.as_posix() == "README.md",
                "breadcrumb_section": breadcrumb_section(relative),
                "breadcrumb_url": breadcrumb_url(relative),
                "doc_prev_url": neighbours["prev"][0] if neighbours["prev"] else None,
                "doc_prev_title": neighbours["prev"][1] if neighbours["prev"] else None,
                "doc_next_url": neighbours["next"][0] if neighbours["next"] else None,
                "doc_next_title": neighbours["next"][1] if neighbours["next"] else None,
            })
            # The docs quote `{{cookiecutter.…}}` and Actions expressions, which
            # Liquid would try to evaluate.
            + "\n{% raw %}\n" + strip_meta_block(strip_title(rewrite_links(text))).strip() + "\n{% endraw %}\n",
            encoding="utf-8",
        )

    # What changed in each release, newest first, plus anything since the last.
    releases = []
    for index, tag in enumerate(tags):
        previous = tags[index + 1] if index + 1 < len(tags) else None
        releases.append({
            "tag": tag,
            "previous": previous,
            "compare_url": (f"{REPO}/compare/{previous}...{tag}" if previous
                            else f"{REPO}/releases/tag/{tag}"),
            "changes": changed_between(previous, tag),
        })

    unreleased = {
        "tag": None,
        "previous": tags[0] if tags else None,
        "compare_url": f"{REPO}/compare/{tags[0]}...main" if tags else None,
        "changes": changed_between(tags[0], "HEAD") if tags else [],
    }

    data = site / "_data"
    data.mkdir(parents=True, exist_ok=True)
    (data / "docs.json").write_text(json.dumps({
        "documents": documents,
        "releases": releases,
        "unreleased": unreleased,
    }, indent=2) + "\n", encoding="utf-8")

    print(f"rendered {len(documents)} documents across {len(releases)} releases; "
          f"{len(unreleased['changes'])} changed since {unreleased['previous'] or 'the start'}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
