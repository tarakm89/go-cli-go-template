---
layout: "doc"
nav_id: "specification"
title: "0001 — Bringing the template to v0.1.0"
eyebrow: "Plan"
doc_path: "docs/plans/0001-template-foundation.md"
doc_status: "Done"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/plans/0001-template-foundation.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/plans/0001-template-foundation.md"
doc_updated: "2026-08-27"
doc_commit: "03b166d"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/03b166df3b4acf244c5c193e648964b04d76df59"
doc_subject: "Add the spec-driven documentation set"
doc_index: false
breadcrumb_section: "Plans"
breadcrumb_url: "/docs/plans/index.html"
---

{% raw %}
- **Status:** Done
- **Shipped in:** [v0.1.0](https://github.com/tarakm89/go-cli-go-template/blob/main/CHANGELOG.md)
- **Implements:** [0001](../specs/0001-hexagonal-core.html), [0002](../specs/0002-observability-in-ci.html)

## Order of work

Each step had to leave the generated project passing `make check`, so the
template was never in a state that produced a broken project.

| # | Step | Verified by |
| --- | --- | --- |
| 1 | Cookiecutter skeleton, prompts, pre- and post-generation hooks | The template renders; no Jinja markers remain |
| 2 | Hexagonal packages with a worked example | `go build ./...`, table-driven unit tests |
| 3 | `depguard` rules for the dependency rule | A deliberate violation fails `make lint` |
| 4 | Cobra command tree, exit codes, `--output` | End-to-end specs against the built binary |
| 5 | Fake adapters, then the functional suite | The suite runs with no network |
| 6 | OpenTelemetry, applied by decoration | Functional specs asserting spans and metrics |
| 7 | Pre-commit hooks and cross-platform bootstrap | `pre-commit run --all-files` on a generated project |
| 8 | Generated documentation, published to `gh-pages` | CI fails if `make docs` produces a diff |
| 9 | The template's own CI | Renders on Linux, macOS and Windows, then runs the generated project's checks |

## What went differently than planned

**Step 5 came later than intended.** The functional suite was originally going
to use generated mocks. Writing the first one made it obvious the test would
assert on call order rather than on outcome, so the fakes were written by hand
and moved out of `_test.go` into a shipped package. That decision is now in
[spec 0001](../specs/0001-hexagonal-core.html).

**Step 3 found a real bug in step 6's code.** The dependency rule was added
before telemetry, and when the telemetry decorator was written the linter
caught a slice aliasing bug in it — `append` onto a caller's slice, shared
across two metric call sites.

**A step was added.** The Go toolchain normalises the `go` directive to a full
patch version, so a project generated with `go_version: 1.25` failed its own
"go mod tidy is up to date" check on the first run. The post-generation hook
now runs `go mod tidy`.
{% endraw %}
