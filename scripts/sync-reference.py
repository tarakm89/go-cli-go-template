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
# The scaffold's directory, URL-encoded. gomarkdoc builds source links from the
# rendered copy it was pointed at, which lives in a gitignored directory; they
# have to be redirected at the template the code actually comes from.
SCAFFOLD = "%7B%7Bcookiecutter.project_slug%7D%7D"


LEADING_HEADING = re.compile(r"\A\s*(#{1,2})\s+(.+?)\s*$", flags=re.MULTILINE)
# gomarkdoc opens with `<!-- Code generated ... -->`, which has to be stepped
# over before the heading is reachable.
LEADING_COMMENT = re.compile(r"\A\s*(<!--.*?-->\s*)+", flags=re.DOTALL)


def without_preamble(text: str) -> tuple[str, str]:
    """Split any leading HTML comments off the front of the document."""
    match = LEADING_COMMENT.match(text)
    return (match.group(0), text[match.end():]) if match else ("", text.lstrip())


def title_of(text: str, fallback: str) -> str:
    """The page's own heading. gomarkdoc opens with an H1, Cobra with an H2."""
    _, body = without_preamble(text)
    match = LEADING_HEADING.match(body)
    return match.group(2) if match else fallback


def strip_title(text: str) -> str:
    """Remove the leading heading; the layout renders it from the front matter."""
    preamble, stripped = without_preamble(text)
    match = LEADING_HEADING.match(stripped)
    if not match:
        return preamble + stripped
    body = stripped[match.end():].lstrip("\n")
    # Cobra nests everything under its H2, which would leave the page with no
    # H2s at all and an empty table of contents. Promote one level.
    if match.group(1) == "##":
        body = re.sub(r"^(#{3,6})\s", lambda m: "#" * (len(m.group(1)) - 1) + " ",
                      body, flags=re.MULTILINE)
    return preamble + body


def retarget_source_links(text: str, render_path: str) -> str:
    """Point gomarkdoc's source links at the scaffold rather than the render.

    gomarkdoc links to the file it read, which is a rendered copy under a
    gitignored directory — those links 404. The scaffold is a line-for-line
    template of the same file, so the line numbers still hold.
    """
    return text.replace(f"/blob/main/{render_path}/", f"/blob/main/{SCAFFOLD}/")


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
    parser.add_argument("--render-path", default=".out/probe-cli",
                        help="where the render sits relative to the repository root, "
                             "so gomarkdoc's source links can be redirected at the scaffold")
    args = parser.parse_args()

    site = pathlib.Path(args.site).resolve()
    project = pathlib.Path(args.project).resolve()
    source = project / "docs"
    # How gomarkdoc referred to the render, relative to the repository root.
    render_path = args.render_path.strip("/")

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
        text = retarget_source_links(page.read_text(encoding="utf-8"), render_path)

        write_page(out / "api" / relative, {
            "layout": "doc",
            "nav_id": "reference",
            "eyebrow": "Package",
            "title": import_path,
            "doc_generated": True,
            "doc_generator": "gomarkdoc",
        }, text)

        packages.append({
            "import_path": import_path,
            "name": relative.stem,
            "layer": layer_of(import_path),
            "url": f"/docs/reference/api/{relative.with_suffix('').as_posix()}.html",
            "summary": summary_of(text),
            "symbols": symbols_of(text),
        })

    # Cobra: docs/cli/<binary>_<command>.md, one per command.
    for page in sorted((source / "cli").glob("*.md")):
        text = page.read_text(encoding="utf-8")
        name = title_of(text, page.stem).strip()

        write_page(out / "cli" / page.name, {
            "layout": "doc",
            "nav_id": "reference",
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
        "nav_id": "reference",
        "eyebrow": "Reference",
        "title": "Reference",
        "reference_index": True,
    }, REFERENCE_INTRO)

    data = site / "_data"
    data.mkdir(parents=True, exist_ok=True)
    layers = [{"name": name,
               "packages": [p for p in packages if p["layer"] == name]}
              for name in LAYER_ORDER]

    (data / "reference.json").write_text(json.dumps({
        "binary": args.binary,
        "packages": packages,
        "layers": [layer for layer in layers if layer["packages"]],
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


ANCHOR = re.compile(r'<a name="([^"]+)"></a>')
TYPE_HEADING = re.compile(r"^##\s+type\s+\\?\[?([A-Za-z0-9_]+)")
METHOD_HEADING = re.compile(r"^###\s+func\s+\\\((?:[^)]*?)\\\)\s+\\?\[?([A-Za-z0-9_]+)")
FUNC_HEADING = re.compile(r"^(##|###)\s+func\s+\\?\[?([A-Za-z0-9_]+)")


def symbols_of(text: str) -> list[dict]:
    """The package's exported surface, as a tree.

    gomarkdoc's output mirrors the structure `go/doc` derives from the AST:
    types at level two, their constructors and methods at level three, package
    level functions at level two. Reading it back gives a code map without
    parsing Go a second time.
    """
    lines = text.splitlines()
    symbols: list[dict] = []
    current_type: dict | None = None
    pending_anchor: str | None = None

    for line in lines:
        anchor_match = ANCHOR.search(line)
        if anchor_match and line.strip() == anchor_match.group(0):
            pending_anchor = anchor_match.group(1)
            continue

        heading = line.strip()
        anchor, pending_anchor = pending_anchor, None

        if heading in ("## Constants", "## Variables"):
            symbols.append({"kind": heading[3:].lower(), "name": heading[3:],
                            "anchor": anchor or heading[3:].lower(), "children": []})
            current_type = None
            continue

        type_match = TYPE_HEADING.match(heading)
        if type_match:
            current_type = {"kind": "type", "name": type_match.group(1),
                            "anchor": anchor or type_match.group(1), "children": []}
            symbols.append(current_type)
            continue

        method_match = METHOD_HEADING.match(heading)
        if method_match and current_type:
            current_type["children"].append({
                "kind": "method", "name": method_match.group(1),
                "anchor": anchor or method_match.group(1)})
            continue

        func_match = FUNC_HEADING.match(heading)
        if func_match:
            entry = {"kind": "func", "name": func_match.group(2),
                     "anchor": anchor or func_match.group(2)}
            # gomarkdoc nests a constructor under the type it returns.
            if func_match.group(1) == "###" and current_type:
                entry["kind"] = "constructor"
                current_type["children"].append(entry)
            else:
                entry["children"] = []
                symbols.append(entry)
                current_type = None

    return symbols


def layer_of(import_path: str) -> str:
    """Group packages the way the architecture describes them."""
    if import_path.startswith("cmd/"):
        return "Entry point"
    if import_path.startswith("internal/core"):
        return "Core"
    if import_path.startswith("internal/adapter/inbound"):
        return "Driving adapters"
    if import_path.startswith("internal/adapter/outbound"):
        return "Driven adapters"
    return "Infrastructure"


LAYER_ORDER = ["Core", "Driving adapters", "Driven adapters", "Infrastructure", "Entry point"]


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
