# Neither this repository nor the projects it generates have a licence

## Where it stands

`git ls-files` shows no `LICENSE`, no `COPYING`, and no licence header anywhere in
this repository. The generated project has none either — the manifest in
`internal/generate/generate.go` lists 23 files and not one of them says under what
terms any of this may be used.

For the tool itself that is a real gap: a repository with no licence is, by
default, all rights reserved. Anyone who finds it cannot legally use the binary it
builds, and nobody can contribute to it with any certainty about what they are
contributing to. The README invites both — "Download the binary for your platform
from the releases, or build it".

For the generated project it is a smaller but more frequent gap: every project
this tool creates starts life unlicensed, and the moment one of them is pushed to
a public repository its author has published something nobody may use.

## What to do

**For this repository:** pick one and commit it. Apache-2.0 is the usual choice
for a tool in the Java and Vaadin ecosystem — it grants patent rights explicitly,
which MIT does not — and it is what most of the dependencies here already use. MIT
is the shorter answer and is fine if a patent grant is not a consideration. Either
way the file is `LICENSE` at the root, and the README grows one line naming it.
This is the user's decision, not a technical one; it needs an owner, not a plan.

**For the generated project:** do not guess. A generated `LICENSE` that says
Apache-2.0 when the author intended something else is worse than none, because it
looks deliberate. Two defensible options:

1. **A `--license` answer** — `none` (default), `apache-2.0`, `mit`, with the
   author name and year filled in from the coordinates and the clock. It is
   another non-boolean answer, so it waits on
   `02-non-boolean-answers-in-the-tui.md`.
2. **A line in the generated README** under a "before you publish this" heading:
   this project has no licence, which means all rights reserved; add one if it is
   going anywhere public. Costs nothing and puts the decision where the author is
   already reading.

Prefer (2) now and (1) later, if licences turn out to be asked for. The paragraph
is useful even after the flag exists, for the projects generated with `none`.

## Also missing, and worth a sentence

`.githooks/commit-msg` in this repository points at a `CODING_CONVENTIONS.md` that
does not exist — see
`23-the-commit-hook-cites-a-file-that-is-not-there.md`. Contribution terms,
conduct and conventions are the same family of file as the licence; all of them
are absent, and only the licence is urgent.
