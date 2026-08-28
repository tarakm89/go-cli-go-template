#!/usr/bin/env python3
"""Publish the Go reference for the template's worked example.

The scaffold under {{cookiecutter.project_slug}}/ is Jinja, so gomarkdoc cannot
read it directly. The workflow renders the template first, runs `make docs` in
the result, and points this script at it. What lands on the site is therefore
the real output of the real pipeline a generated project runs — package
documentation from gomarkdoc, command documentation from the Cobra tree.
"""

from __future__ import annotations

import argparse
import json
import pathlib
import re
import shutil
import sys

REPO = "https://github.com/tarakm89/go-cli-go-template"


LEADING_HEADING = re.compile(r"\A\s*(#{1,2})\s+(.+?)\s*$", flags=re.MULTILINE)


def title_of(text: str, fallback: str) -> str:
    """The page's own heading. gomarkdoc opens with an H1, Cobra with an H2."""
    match = LEADING_HEADING.match(text.lstrip())
    return match.group(2) if match else fallback


def strip_title(text: str) -> str:
    """Remove the leading heading; the layout renders it from the front matter."""
    stripped = text.lstrip()
    match = LEADING_HEADING.match(stripped)
    if not match:
        return text
    body = stripped[match.end():].lstrip("\n")
    # Cobra nests everything under its H2, which would leave the page with no
    # H2s at all and an empty table of contents. Promote one level.
    if match.group(1) == "##":
        body = re.sub(r"^(#{3,6})\s", lambda m: "#" * (len(m.group(1)) - 1) + " ",
                      body, flags=re.MULTILINE)
    return body


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


def write_page(target: pathlib.Path, fields: dict, body: str) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    # gomarkdoc emits Go source, which is full of braces Liquid would try to
    # read as tags.
    target.write_text(
        front_matter(fields) + "\n{% raw %}\n" + strip_title(body).strip() + "\n{% endraw %}\n",
        encoding="utf-8",
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--site", required=True, help="checkout of the gh_pages branch")
    parser.add_argument("--project", required=True, help="a rendered project with docs/ generated")
    parser.add_argument("--binary", default="probe-cli", help="the rendered project's binary name")
    args = parser.parse_args()

    site = pathlib.Path(args.site).resolve()
    project = pathlib.Path(args.project).resolve()
    source = project / "docs"

    if not source.is_dir():
        print(f"error: {source} not found; run `make docs` in the project first", file=sys.stderr)
        return 1

    out = site / "docs" / "reference"
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)

    packages, commands = [], []

    # gomarkdoc: docs/api/<import path>.md, one per package.
    for page in sorted((source / "api").rglob("*.md")):
        relative = page.relative_to(source / "api")
        import_path = str(relative.with_suffix("")).replace("\\", "/")
        text = page.read_text(encoding="utf-8")

        write_page(out / "api" / relative, {
            "layout": "doc",
            "nav_id": "docs",
            "eyebrow": "Package",
            "title": import_path,
            "doc_generated": True,
            "doc_generator": "gomarkdoc",
        }, text)

        packages.append({
            "import_path": import_path,
            "name": relative.stem,
            "url": f"/docs/reference/api/{relative.with_suffix('').as_posix()}.html",
            "summary": summary_of(text),
        })

    # Cobra: docs/cli/<binary>_<command>.md, one per command.
    for page in sorted((source / "cli").glob("*.md")):
        text = page.read_text(encoding="utf-8")
        name = title_of(text, page.stem).strip()

        write_page(out / "cli" / page.name, {
            "layout": "doc",
            "nav_id": "docs",
            "eyebrow": "Command",
            "title": name,
            "doc_generated": True,
            "doc_generator": "cobra",
        }, text)

        commands.append({
            "name": name,
            "url": f"/docs/reference/cli/{page.stem}.html",
            "summary": summary_of(text),
            "depth": name.count(" "),
        })

    write_page(out / "index.md", {
        "layout": "doc",
        "nav_id": "docs",
        "eyebrow": "Reference",
        "title": "Reference",
        "reference_index": True,
    }, REFERENCE_INTRO)

    data = site / "_data"
    data.mkdir(parents=True, exist_ok=True)
    (data / "reference.json").write_text(json.dumps({
        "binary": args.binary,
        "packages": packages,
        "commands": commands,
    }, indent=2) + "\n", encoding="utf-8")

    print(f"published reference for {len(packages)} packages and {len(commands)} commands")
    return 0


REFERENCE_INTRO = """# Reference

Generated from the template's worked example, not written by hand. Every push
that changes the scaffold renders the template, runs `make docs` in the result
and republishes what comes out, so this is the actual output of the pipeline a
generated project runs.

- **Packages** come from [gomarkdoc](https://github.com/princjef/gomarkdoc),
  which reads the doc comments.
- **Commands** come from the Cobra tree itself, so the flags listed are the
  flags the binary has.

In your own project the same two commands produce the same pages, into
`docs/api` and `docs/cli`:

```sh
make docs
```
"""


def summary_of(text: str) -> str:
    """The first sentence of prose, for the index listing."""
    body = strip_title(text)
    for line in body.splitlines():
        line = line.strip()
        if not line or line.startswith(("#", "```", "<!--", "|", "-", "*", "import")):
            continue
        sentence = re.split(r"(?<=[.!?])\s", line)[0]
        return re.sub(r"[*_`\\]", "", sentence)[:180]
    return ""


if __name__ == "__main__":
    sys.exit(main())
