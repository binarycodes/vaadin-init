# The generated project has no `.gitattributes`, and needs one for the same reason this repository does

## Where it stands

This repository's `.gitattributes` is one line with nine lines of reasoning above
it:

> `templates/run.sh` is embedded in the binary and copied verbatim into every
> generated project, so a checkout that converted it to CRLF would produce
> projects whose task runner dies on its first line with `$'\r': command not
> found`. The same goes for the commit-msg hook. Windows defaults to
> `core.autocrlf=true` […] and it would happen at build time, silently, with
> nothing in the diff to show it.

Every word of that applies one step downstream. The generated project contains
the same `run.sh` and the same `.githooks/commit-msg`, is initialised as a git
repository by the tool itself, and ships no `.gitattributes` at all
(`internal/generate/generate.go`'s manifest has no entry for one). The moment
that project is pushed and cloned on a Windows machine with the default
`core.autocrlf=true`, its task runner and its hook grow carriage returns and stop
working — with, again, nothing in the diff to show it.

The tool protected itself from this and did not pass the protection on.

## What to do

Add `templates/gitattributes.tmpl` to the manifest, landing at `.gitattributes`,
unconditional — the same shape as `gitignore.tmpl`:

```
# Line endings are LF in the repository and in every checkout, on every platform.
#
# run.sh and .githooks/commit-msg are shell scripts: a checkout that converted
# them to CRLF produces a task runner that dies on its first line with
# "$'\r': command not found", and a commit hook that silently rejects every
# message. Windows defaults to core.autocrlf=true, which is exactly where that
# happens.
* text=auto eol=lf
```

Whether to narrow it to the two scripts rather than `*` is a real question. `* …
eol=lf` is what this repository chose and it is the simpler rule; a project that
later adds a Windows-only file that genuinely wants CRLF can override that one
path. Ship the simple rule and say so in the comment.

## Related

While in the manifest: an `.editorconfig` is the same class of file — one that
costs nothing and prevents a category of noise — and the generated project has
none either. It is a weaker case: it changes nothing about whether the project
works, only about whose editor reformats what. Decide it separately, and do not
let it delay this.

## Test

`internal/generate`: `.gitattributes` is generated for every combination and
contains `eol=lf`. The existing `TestCoreFilesAreAlwaysGenerated` has the list to
add it to.
