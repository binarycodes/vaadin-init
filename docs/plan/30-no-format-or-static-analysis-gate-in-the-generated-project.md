# The tool gates its own formatting and hands the project nothing

## Where it stands

This repository holds itself to a standard and enforces it mechanically:
`make check` is `fmt vet test`, `make dist` depends on `check`, and CI re-runs
`gofmt -l` on three platforms with a comment explaining that formatting is
*checked* rather than applied, "because a build that rewrites the tree under you
is a build that makes `git diff` lie".

The generated project inherits the philosophy and none of the mechanism. Its build
gates warnings (`-Xlint:all -Werror`, with the categories that fire on correct
Vaadin and Spring code excluded) and, optionally, coverage. It gates nothing about
formatting, import order, or the static-analysis findings that `javac` does not
produce. `./run.sh` has no `format` or `check` task. Two people working on a
generated project will disagree about indentation in week one, and the first
argument about it will be settled by whoever reviews the pull request.

The gap is worth closing not because formatting matters much, but because *this
tool's whole thesis* is that these decisions are worth making once, up front, with
the reasoning written down — and it made this one for itself and skipped it for
its output.

## What to do

Add one plugin to the generated pom, bound so that the build checks and a task
applies — the same split this repository chose for `gofmt`:

- **Spotless** with a Java formatter (palantir-java-format or google-java-format
  in AOSP style) plus import ordering and a trailing-whitespace rule. Bind
  `spotless:check` into `verify`, and add `./run.sh format` for
  `spotless:apply`. The check failing tells you exactly which files and how to fix
  them in one command.
- Keep it out of `compile`. A format check that fires on every `./run.sh compile`
  during a debugging session is a format check people disable.

**Static analysis is a separate decision, and a heavier one.** SpotBugs or
Error Prone find real defects, and both produce findings on correct code often
enough that adding one unasked would repeat the mistake `-Werror`'s exclusion list
was written to avoid. If either goes in, it goes in with a curated exclusion file
and a paragraph in the generated README saying what it will complain about. Better
still: leave it out, and say in the README that it is the obvious next gate and
where to put it.

**Whether it should be optional.** Every other gate in this tool is a flag —
`--coverage`, `--traceable`. Formatting is cheaper than both and less arguable, so
unconditional is defensible; if it becomes a flag, it is one more boolean and the
existing machinery takes it without complaint.

## For the tool itself

The Go side has the mirror gap: `go vet` is the whole of the static analysis, and
`staticcheck` or `golangci-lint` would say more about a 3,500-line codebase than
vet does. It is a one-line CI step and a `make check` addition — worth doing, and
worth doing separately from the templates, since the two land on different
audiences.

## Test

The gate is the test. What `internal/generate` should assert is only that the
plugin's configuration renders and the pom stays well-formed; CI's
`generated-project` job then proves the check actually passes on the code the
templates produce — which is the assertion that matters, because a generated
project failing its own format check on the first commit would be the worst
possible version of this.
