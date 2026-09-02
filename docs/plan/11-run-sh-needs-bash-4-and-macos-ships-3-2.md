# The task runner does not start on a stock macOS

## Where it stands

`templates/run.sh` is `#!/usr/bin/env bash` and uses bash 4 syntax at the top
level, on a line every task executes before anything else happens:

```sh
# templates/run.sh:47
: "${CONTAINER_PREFIX:=${PROJECT_NAME,,}-dev}"
```

`${VAR,,}` — lowercasing expansion — arrived in bash 4.0. macOS ships bash 3.2.57
as `/bin/bash` and has since 2007, because bash moved to GPLv3. On a Mac without
a newer bash on the `PATH`, `env bash` finds that 3.2, and the first `./run.sh`
of a freshly generated project fails on line 47 with `bad substitution` — before
it has said a single useful thing.

`mapfile` (line 337) is bash 4 too, though it sits in the podman/quadlet path that
only runs on Linux.

This matters more than a portability nit because of what the tool claims. It
cross-compiles a `darwin/arm64` binary, CI runs the tool's own tests on
`macos-latest`, and the README's whole pitch is "one binary, no toolchain to
install first". A Mac user follows that to the letter and gets a project whose
first documented command does not run.

Nothing catches it: the `generated-project` CI job is `runs-on: ubuntu-latest`,
so no generated project has ever been executed on macOS.

## What to do

Two changes, both small:

1. **Drop the bash 4 constructs.** The lowercasing has a portable form:

   ```sh
   : "${CONTAINER_PREFIX:=$(printf '%s' "${PROJECT_NAME}" | tr '[:upper:]' '[:lower:]')-dev}"
   ```

   `mapfile` becomes a `while read` loop, or the quadlet services can stay an
   array built by the loop that already exists.

2. **Prove it stays fixed.** Add `macos-latest` to the `generated-project` job's
   matrix. It is the only assertion that survives someone reintroducing a bash 4
   expansion, and macOS runners are the platform this repository already pays for
   in the `tool` job.

If a bash 4 feature is ever genuinely worth it, the honest alternative is to say
so: check `BASH_VERSINFO` at the top and exit with "this runner needs bash 4;
`brew install bash`". A clear refusal beats `bad substitution` on line 47. But
nothing in this file currently needs bash 4 at all.

## Cost

The macOS matrix entry roughly doubles the slowest job's wall-clock — macOS
runners are slower and a Vaadin build is not quick. If that is too much for every
push, run the extra platform on `main` only, or nightly. Some coverage on the
platform the tool ships a binary for is the point; running it on every pull
request is not.

## Test

`internal/generate` cannot catch this — it renders text and never runs it. The
only real check is executing a generated project's `run.sh` on macOS, which is
why this belongs in CI rather than in the Go tests. A cheap complement:
`bash --posix -n` style parse checks are not enough, but `shellcheck` with
`shell=bash` and a declared minimum version does flag some of it — see
`15-nothing-checks-the-shell-script-that-ships-everywhere.md`.
