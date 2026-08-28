"""Validate the answers before anything is written to disk."""

import re
import sys

MODULE_RE = re.compile(r"^[a-zA-Z0-9][a-zA-Z0-9._~/-]*[a-zA-Z0-9]$")
SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$")
BINARY_RE = re.compile(r"^[a-z0-9][a-z0-9_-]*$")
GO_VERSION_RE = re.compile(r"^1\.\d{2}(\.\d+)?$")

CHECKS = (
    ("project_slug", "{{ cookiecutter.project_slug }}", SLUG_RE,
     "lowercase letters, digits and dashes"),
    ("binary_name", "{{ cookiecutter.binary_name }}", BINARY_RE,
     "lowercase letters, digits, dashes and underscores"),
    ("module_path", "{{ cookiecutter.module_path }}", MODULE_RE,
     "a valid Go module path, e.g. github.com/acme/my-cli"),
    ("go_version", "{{ cookiecutter.go_version }}", GO_VERSION_RE,
     "a Go release such as 1.25 or 1.25.1"),
)


def main() -> int:
    failed = False
    for name, value, pattern, expected in CHECKS:
        if not pattern.match(value):
            print(f"ERROR: {name} = {value!r} is invalid; expected {expected}.")
            failed = True

    # The `tool` directive used in go.mod needs Go 1.24 or newer.
    minor = int("{{ cookiecutter.go_version }}".split(".")[1])
    if minor < 24:
        print("ERROR: go_version must be 1.24 or newer (the go.mod `tool` directive).")
        failed = True

    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
