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


def status_of(text: str) -> str | None:
    """Specs and plans carry `- **Status:** Accepted` near the top."""
    match = re.search(r"^-\s*\*\*Status:\*\*\s*(.+?)\s*$", text, flags=re.MULTILINE)
    return match.group(1) if match else None


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

    out = site / "docs"
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)

    tags = tags_newest_first()
    documents = []

    for source in sorted(SOURCE.rglob("*.md")):
        relative = source.relative_to(SOURCE)
        text = source.read_text(encoding="utf-8")
        commit = last_commit(source.relative_to(ROOT))

        document = {
            "title": title_of(text, relative.stem),
            "path": f"docs/{relative.as_posix()}",
            "url": page_url(relative),
            "section": relative.parts[0] if len(relative.parts) > 1 else "root",
            "status": status_of(text),
            "commit": commit,
            "release": release_for(commit["sha"], tags) if commit else None,
            "source_url": f"{REPO}/blob/main/docs/{relative.as_posix()}",
            "history_url": f"{REPO}/commits/main/docs/{relative.as_posix()}",
        }
        documents.append(document)

        target = out / relative
        if relative.name == "README.md":
            target = target.with_name("index.md")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(
            front_matter({
                "layout": "doc",
                "nav_id": "docs",
                "title": document["title"],
                "eyebrow": {"specs": "Spec", "plans": "Plan"}.get(document["section"], "Docs"),
                "doc_path": document["path"],
                "doc_status": document["status"],
                "doc_source": document["source_url"],
                "doc_history": document["history_url"],
                "doc_updated": commit["date"] if commit else None,
                "doc_commit": commit["short_sha"] if commit else None,
                "doc_commit_url": commit["url"] if commit else None,
                "doc_subject": commit["subject"] if commit else None,
                "doc_release": document["release"],
            })
            # The docs quote `{{cookiecutter.…}}` and Actions expressions, which
            # Liquid would try to evaluate.
            + "\n{% raw %}\n" + rewrite_links(text).strip() + "\n{% endraw %}\n",
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
