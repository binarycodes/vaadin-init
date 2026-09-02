# The 551-line shell script in every generated project is checked by nobody

## Where it stands

The tool's own CI is thorough about Go: `gofmt` on three platforms, `go vet`, the
tests, a binary smoke test, and a cross-compile. `make check` is `fmt vet test`.

`templates/run.sh` gets none of it. It is 551 lines of bash — the largest single
file in the repository, larger than any Go file in it — it is copied verbatim into
every generated project, it is the documented way to build, test, run and package
those projects, and nothing in this repository parses it, lints it or executes it.
The only time it runs at all is inside the `generated-project` CI job, which
exercises exactly one task (`test`) on exactly one platform.

The tasks nobody has ever run in CI include `env` in all four of its actions,
`bundle`, `styles`, `deps`, `verify`, `package` and `clean`, plus the whole
quadlet path and every error branch — the socket-resolution logic, which is the
most intricate code in the file and the most likely to be wrong on a machine
nobody tested.

## What to do

Three steps, cheapest first:

1. **`shellcheck` in CI, and in `make check`.** It is one job step
   (`shellcheck templates/run.sh templates/commit-msg`) and it catches the whole
   class of bash faults that only fire on the branch nobody took: an unquoted
   expansion with a space in it, a `local` that swallows an exit status, a `cd`
   whose failure is ignored. Add `.shellcheckrc` for the handful of intentional
   exceptions rather than sprinkling `# shellcheck disable` through the file.
   Expect real findings on the first run; triage them, do not bulk-disable.

2. **`bash -n` on the rendered file, in the Go tests.** A parse check needs no
   external tool, runs in milliseconds, and would have caught a template that
   renders into broken shell. `internal/generate` already renders every
   combination; `run.sh` is verbatim, but `run.conf` and `run.tasks.sh` are not,
   and both are sourced by the runner — so a project name carrying shell syntax
   reaches a file that is executed, not just read.

3. **Exercise more than one task in the generated-project job.** `deps`,
   `clean` and `package` cost little beyond what the job already pays for. `env up`
   and `env down` for the full configuration would actually exercise the compose
   path on a runner that has docker.

## Note on the project name

Point (2) is not hypothetical. `run.conf` is generated as
`PROJECT_NAME="{{.ProjectName}}"` and `ValidProjectName` accepts anything
non-blank, so the name lands unescaped inside double quotes in a file `run.sh`
sources on every task. An apostrophe is harmless there, but a `$(…)` is not:
`--name '$(id) app'` produces a `run.conf` that executes `id` every time anyone
runs a task, and a name containing a double quote silently produces a different
value than the one asked for. See
`17-user-input-is-injected-into-five-syntaxes-unescaped.md`, which is the general
form of this — `run.conf` is one of five files it reaches.

## Test

The shell checks are CI and `make check`. The `run.conf` quoting fix belongs in
`internal/generate` (render with a hostile name, assert `bash -n` accepts the
result and that the value survives intact) rather than in a validation rule: it
tests the property that matters instead of the rule that happens to protect it.
