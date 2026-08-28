# gh_pages

This branch is the source of the documentation site at
<https://tarakm89.github.io/go-cli-go-template/>.

It is hand-written, not generated. The template itself lives on `main`; when
the template changes in a way that affects the docs, update these pages in the
same pull request.

| File | Page |
| --- | --- |
| `index.md` | Landing page — what the template is and why |
| `usage.md` | Generating a project, the prompts, the day-to-day commands |
| `configured.md` | Every moving part in a generated project, and why |
| `mentality.md` | How we expect code to be written |

A push to this branch triggers `.github/workflows/pages.yml`, which builds the
site with Jekyll and deploys it.
