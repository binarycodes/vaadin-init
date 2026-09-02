# The one package with no tests is the one deciding what gets generated

## Where it stands

Every `internal` package is tested — 1,100 lines of test across config, generate,
prompt and versions, including a 32-way combination matrix and a set of TUI layout
assertions. `main.go` has 368 lines and no test file at all.

It is not a thin shell. `main.go` owns:

- `preParse`, which parses a copy of the flag set so `--defaults` can be read
  before the flags it supplies defaults for are defined — subtle enough to need
  eleven lines of comment;
- `setFlags`, and the derivation rules that depend on it: whether `ProjectName`,
  `Package` and `OutputDir` follow the coordinates or the user's explicit answer.
  Getting this wrong means `--artifact-id` alone silently generates a project
  whose package still carries the defaults file's example name — the exact bug the
  comment says it exists to prevent;
- the interactive/non-interactive decision, including the `--accessible` exception
  for pipes;
- the version-lookup fallback on the scripted path, which is supposed to make a
  scripted run and an interactive one start from the same numbers;
- `printResult`, `printDryRun` and `displayPath`.

The only thing standing behind any of it is the CI smoke test, which runs
`--dry-run --yes` with two flags and checks the exit status.

## What to do

Move the logic into `internal/cli` and leave `main.go` as `func main() {
os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }`. Package `main` is
importable by nothing, which is the whole reason it has no tests; everything else
follows from that one move.

Then the tests that matter are cheap:

- **Derivation.** A table over flag combinations asserting the resulting `Config`:
  `--artifact-id book-shelf` alone derives name, package and directory;
  `--package com.acme.thing` wins over derivation; `--dir` likewise. This is the
  highest-value test in the list — it is the behaviour most likely to break and
  the least likely to be noticed.
- **Defaults layering.** `--defaults <path>` with a partial file changes only what
  it names; a missing explicit path is an error and a missing per-user path is
  not. `internal/config` tests the loader; nothing tests that `main` wires it up
  in that order.
- **Version fallback on the scripted path.** With a lookup that returns nothing,
  `--yes` keeps the defaults file's versions; with one that returns a list, it
  takes the newest — and `--vaadin-version` beats both. `startLookup` already
  returns a `prompt.VersionSource` function, so a test injects one directly.
- **Output.** `printResult` with each shape of `generate.Result`, including the
  git-failure notice — which is the branch
  `10-the-summary-promises-a-build-that-cannot-run.md` wants to change and cannot
  currently test.

## Cost

One rename, one import cycle to not create, and `main.go` shrinks to four lines.
The flag definitions move with the logic; `usage` moves with them. Nothing about
the tool's behaviour changes, which is what makes it safe to do before the changes
that will.
