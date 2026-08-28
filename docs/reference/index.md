---
layout: "doc"
nav_id: "docs"
eyebrow: "Reference"
title: "Reference"
reference_index: true
---

{% raw %}
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
{% endraw %}
