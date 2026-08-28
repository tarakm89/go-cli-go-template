---
layout: "doc"
nav_id: "specification"
title: "Specification"
eyebrow: "Spec"
doc_path: "docs/specs/README.md"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/specs/README.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/specs/README.md"
doc_updated: "2026-08-28"
doc_commit: "c29c2bb"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/c29c2bbca092e57be6cf66818d5b50d07bc8c5d9"
doc_subject: "Give specification and reference their own sections"
doc_index: false
breadcrumb_section: "Specification"
breadcrumb_url: "/docs/specs/index.html"
---

{% raw %}
What this project should do, and how we will know it does. One document per
capability, written before it is built.

A change of any size starts here rather than on a branch: a spec is cheap to
argue with, an implementation is not. The acceptance criteria in each one
become the tests — usually a functional spec in `test/functional`.

| Spec | Status | What it covers |
| --- | --- | --- |
| [0001](0001-hexagonal-core.html) | Implemented | The hexagonal core and its enforced dependency rule |
| [0002](0002-observability-in-ci.html) | Implemented | OpenTelemetry for tools that run inside pipelines |
| [0003](0003-spec-driven-docs.html) | Implemented | Publishing these documents, and showing what changed |

Start a new one by copying [`TEMPLATE.md`](TEMPLATE.html) to
`NNNN-short-name.md`, taking the next free number.

The table above is maintained by hand and is the one thing here that can go
stale. The [published documentation
index](https://tarakm89.github.io/go-cli-go-template/docs/index.html) is
generated from the files themselves, along with when each last changed and
which release carried it — trust that one if the two disagree.

## Status values

| Status | Means |
| --- | --- |
| `Draft` | Being written or argued about. Not safe to build from. |
| `Accepted` | Agreed. Build it. |
| `Implemented` | Shipped, and the acceptance criteria are covered by tests. |
| `Superseded by NNNN` | A later spec replaced this decision. Kept for the record. |
| `Rejected` | Considered and declined. Kept so the question is not reopened blind. |

Nothing is deleted and numbers are never reused: a rejected spec is a record of
a decision, and that is worth as much as an accepted one.

## Related

- **[Architecture](../architecture.html)** — the shape these specs are built
  into, and the decisions behind it.
- **[Plans](../plans/index.html)** — how the larger specs were actually carried
  out, in what order, and what went differently than intended.
- **[All documents](../index.html)** — every document with its status, and what
  changed in each release.
- **[Reference](https://tarakm89.github.io/go-cli-go-template/docs/reference/index.html)**
  — the packages and commands these specs became. Generated, so it only exists
  on the site.
{% endraw %}
