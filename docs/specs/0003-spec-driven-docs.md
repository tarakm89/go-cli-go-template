---
layout: "doc"
nav_id: "specification"
title: "0003 — Publish these documents, and show what changed"
eyebrow: "Spec"
doc_path: "docs/specs/0003-spec-driven-docs.md"
doc_status: "Implemented"
doc_source: "https://github.com/tarakm89/go-cli-go-template/blob/main/docs/specs/0003-spec-driven-docs.md"
doc_history: "https://github.com/tarakm89/go-cli-go-template/commits/main/docs/specs/0003-spec-driven-docs.md"
doc_updated: "2026-08-27"
doc_commit: "03b166d"
doc_commit_url: "https://github.com/tarakm89/go-cli-go-template/commit/03b166df3b4acf244c5c193e648964b04d76df59"
doc_subject: "Add the spec-driven documentation set"
doc_index: false
breadcrumb_section: "Specification"
breadcrumb_url: "/docs/specs/index.html"
doc_prev_url: "/docs/specs/0002-observability-in-ci.html"
doc_prev_title: "0002 — OpenTelemetry for tools that run inside pipelines"
doc_next_url: "/docs/specs/TEMPLATE.html"
doc_next_title: "NNNN — Short title"
---

{% raw %}
- **Status:** Implemented
- **Date:** 2026-08-27

## Problem

We work spec first, but the specs live in a directory that people have to
remember to open. Meanwhile the documentation site describes the template in
prose that is maintained separately, so the two drift.

Worse, when a release goes out there is no way to see *which documents changed
with it*. "What is new in v0.2.0" and "why did it change" live in different
places, and nobody joins them up.

## Goals

- Every document under `docs/` on `main` is published, automatically, on every
  commit that touches it.
- A reader can see when a document last changed and what the commit said.
- A reader can see which documents changed in each release.
- Nothing about the above is maintained by hand.

## Non-goals

- Versioned documentation — one published copy per release, switchable. That is
  a much larger job and we do not need it yet; the released *tags* are already
  browsable on GitHub.
- Rendering anything but Markdown.

## Proposal

**Source of truth stays on `main`.** `docs/**/*.md` is written and reviewed
like code.

**A workflow on `main` syncs.** On any push touching `docs/`, `CHANGELOG.md` or
the sync script, `scripts/sync-docs.py` renders each document into the
`gh_pages` branch with Jekyll front matter, writes `_data/docs.json`, commits
the result, and fires a `repository_dispatch` so the site rebuilds.

Committing into `gh_pages` rather than only publishing means the branch is a
readable, diffable record of what the site says at any point.

**Metadata carries the history.** For every document `_data/docs.json` records
the last commit that touched it — sha, date, subject, author — and which
release it fell into. The site turns that into a "last updated" line on each
page and a "what changed" list per release.

## Acceptance criteria

- [x] Given a push to `main` that edits `docs/architecture.md`, then the
      `gh_pages` branch gains the change and the site rebuilds without anyone
      pressing anything.
- [x] Given a push that touches no documentation, then `gh_pages` is not
      committed to.
- [x] Given a published document, then the page shows when it last changed and
      links to its history.
- [x] Given a release tag, then the documents changed since the previous tag
      are listed on the site.
- [x] Given a document containing a Mermaid diagram, then the published page
      renders it in both light and dark themes.

## Alternatives considered

**Build the site from `main` and drop `gh_pages` entirely.** Fewer moving
parts. Rejected because the branch is useful in its own right: it is a plain
record of what was published, diffable without rebuilding anything.

**Pull the documents at build time instead of committing them.** Slightly
simpler, but then `gh_pages` does not contain what it publishes, and the "what
changed" question has to be answered by a live API call.

## Risks and costs

The sync workflow can push to `gh_pages`, so a bug there can churn that branch.
It is guarded: nothing is committed when the rendered output is unchanged.
{% endraw %}
