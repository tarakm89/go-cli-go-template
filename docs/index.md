---
layout: "doc"
nav_id: "specification"
title: "Documentation"
eyebrow: "Docs"
doc_path: "docs/README.md"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/README.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/README.md"
doc_updated: "2026-08-28"
doc_commit: "c29c2bb"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/c29c2bbca092e57be6cf66818d5b50d07bc8c5d9"
doc_subject: "Give specification and reference their own sections"
doc_index: true
---

{% raw %}
The written record of how this template works and why. Everything here is
published to <https://tarakm89.github.io/go-cli-go-template/docs/> on every
push to `main`, so the site and the source can never disagree.

## How this is organised

| Directory | Holds | Answers |
| --- | --- | --- |
| [`architecture.md`](architecture.html) | The shape of the system and the decisions behind it | *How is this built, and why that way?* |
| [`specs/`](specs/) | One document per capability, written before it is built | *What should this do, and how will we know it does?* |
| [`plans/`](plans/) | Implementation plans for work in flight | *How are we going to get there, in what order?* |
| [Reference](reference/) | Package and command documentation, generated | *What does this package expose? What flags does this command take?* |

The reference has a section of its own on the site and is not in this
directory: it is generated in CI from the template's worked example. `make
docs` in your own project produces the same pages into `docs/api` and
`docs/cli`.

## Spec-driven, in practice

A change of any size starts as a spec, not as a branch.

1. **Write the spec.** Copy [`specs/TEMPLATE.md`](specs/TEMPLATE.html) to
   `specs/NNNN-short-name.md`. State the problem, the behaviour you want, and
   how it will be verified. If you cannot write the acceptance criteria, you do
   not understand the problem yet.
2. **Get it read.** A spec is cheap to argue with; an implementation is not.
3. **Plan it**, if it is large enough to need one. Small specs go straight to
   code.
4. **Build it.** The spec's acceptance criteria become the tests — a functional
   spec in `test/functional`, usually.
5. **Close the loop.** Mark the spec `Accepted`, and record the change in
   [`CHANGELOG.md`](https://github.com/tarakm89/go-cli-go-template/blob/main/CHANGELOG.md) under the release that carries it.

The last step is what makes the site useful: every release lists the documents
that changed in it, so *what shipped* and *why it shipped* stay attached to
each other.

## Conventions

- **Numbering.** `NNNN-kebab-case.md`, allocated in order. Numbers are never
  reused, even if a spec is rejected — a rejected spec is a record of a
  decision.
- **Status.** Every spec carries one: `Draft`, `Accepted`, `Implemented`,
  `Superseded by NNNN` or `Rejected`. Nothing is deleted.
- **One heading one.** Each file starts with a single `#` heading; the
  published site uses it as the page title.
- **Write for someone new.** These pages are read most often by whoever joins
  next, or by you in six months. Say why, not just what.
{% endraw %}
