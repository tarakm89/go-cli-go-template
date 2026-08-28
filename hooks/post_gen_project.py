"""Tidy up the generated project: licence, docs placeholder, git init."""

import datetime
import os
import pathlib
import shutil
import subprocess
import sys

LICENSE = "{{ cookiecutter.license }}"
INIT_GIT = "{{ cookiecutter.init_git_repo }}" == "yes"
PROJECT = pathlib.Path.cwd()


def finish_license() -> None:
    path = PROJECT / "LICENSE"
    if LICENSE == "None":
        path.unlink(missing_ok=True)
        return
    year = str(datetime.date.today().year)
    path.write_text(path.read_text().replace("__YEAR__", year), encoding="utf-8")


def tidy_module() -> None:
    """Normalise go.mod for the chosen Go version.

    The toolchain rewrites the `go` directive to a full patch version, so a
    project generated with `go_version: 1.26` would otherwise fail its own
    "go mod tidy is up to date" CI check on the very first run.
    """
    if shutil.which("go") is None:
        print("note: Go not found on PATH; run `go mod tidy` before your first commit")
        return
    try:
        subprocess.run(["go", "mod", "tidy"], cwd=PROJECT, check=True)
    except (subprocess.CalledProcessError, OSError) as err:  # non-fatal, may be offline
        print(f"warning: `go mod tidy` did not complete ({err}); run it before your first commit")


def generate_docs() -> None:
    """Render docs/cli and docs/api so the first commit already has them.

    Without this the generated project starts life with a documentation
    directory that CI would flag as out of date on the first push.
    """
    if shutil.which("go") is None:
        return

    commands = [
        ["go", "run", "./cmd/{{ cookiecutter.binary_name }}", "docs", "--output-dir", "docs/cli"],
        ["go", "tool", "gomarkdoc",
         "--output", "docs/api/{% raw %}{{.Dir}}{% endraw %}.md",
         "./cmd/...", "./internal/..."],
    ]
    for command in commands:
        try:
            subprocess.run(command, cwd=PROJECT, check=True)
        except (subprocess.CalledProcessError, OSError) as err:  # non-fatal
            print(f"warning: could not generate documentation ({err}); run `make docs` later")
            return


def init_git() -> None:
    if not INIT_GIT or shutil.which("git") is None:
        return
    try:
        subprocess.run(["git", "init", "--quiet", "--initial-branch=main"],
                       cwd=PROJECT, check=True)
        subprocess.run(["git", "add", "."], cwd=PROJECT, check=True)
    except (subprocess.CalledProcessError, OSError) as err:  # non-fatal
        print(f"warning: could not initialise a git repository: {err}")


def main() -> int:
    finish_license()
    tidy_module()
    generate_docs()
    init_git()
    print(
        "\n{{ cookiecutter.project_name }} is ready in "
        f"{os.path.basename(PROJECT)}/\n"
        "\n  cd {{ cookiecutter.project_slug }}\n"
        "  make bootstrap   # download modules and dev tools\n"
        "  make check       # fmt, lint, unit, functional and e2e tests\n"
        "  make run -- check https://example.com\n"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
