---
layout: default
eyebrow: Releases
nav_id: versions
title: Versions
description: Every release of the template, with links to the diffs.
---

Versioning applies to **the template**, not to the projects generated from it.
A project you generated is yours from that moment on; these versions tell you
what the scaffold looked like when you generated it, and what has changed
since.

| Bump | Means |
| --- | --- |
| **Major** | A generated project's layout or contract changes in a way that makes the previous shape wrong. Regenerating is not a drop-in. |
| **Minor** | New capability in generated projects, or a new prompt with a sensible default. |
| **Patch** | Fixes, dependency bumps, documentation. |

## Reviewing what changed

Each heading below links to its release tag, and the version numbers in
brackets link to a diff against the release before it. To do the same locally:

{% raw %}
```sh
# Everything that changed in a release
git diff v0.1.0..v0.2.0

# Only the scaffold — usually what you actually want
git diff v0.1.0..v0.2.0 -- '{{cookiecutter.project_slug}}'

# Which files moved, without the noise
git diff --stat v0.1.0..v0.2.0
```
{% endraw %}

To bring an existing project up to date with a newer template, read the diff of
the scaffold and apply what is relevant by hand. Regenerating over a project
you have already changed will overwrite your work.

<!-- CHANGELOG -->

## Bringing this list up to date

The list above is generated at build time from
[`CHANGELOG.md`]({{ site.repository_url }}/blob/main/CHANGELOG.md) on the
`main` branch, so a release is written down in exactly one place. Add an entry
there, tag the commit, and this page follows on the next deploy.
