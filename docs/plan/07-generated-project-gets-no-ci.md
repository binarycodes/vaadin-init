# The generated project ships no continuous integration

## Where it stands

The generated project is opinionated about almost everything a new project needs
to be opinionated about: a task runner, a commit-message hook wired up, a first
commit, a test suite, an integration-test profile, a coverage gate, and a rule
that every build carries its commit SHA.

It has no CI. Nothing runs any of that except a person remembering to.

The gap is sharpest for the two options that exist to be enforced. A commit-msg
hook is local and can be skipped with `--no-verify`; a coverage gate only gates
the builds someone runs. Both are conventions that hold on a laptop and evaporate
on a pull request, which is the only place they were ever going to matter for more
than one person.

This repository knows the argument: its own `.github/workflows/build.yml` opens by
saying the generated-project job "is the point", because nothing else proves the
templates produce something that compiles. The generated project deserves the same
reasoning about its own code.

## What to do

Add one template, `templates/github-workflow.yml.tmpl`, landing at
`.github/workflows/build.yml`, behind no option — every project gets it, the way
every project gets `run.sh`.

It should do the least that is worth doing, through the task runner the project
already ships, so that CI and a laptop cannot disagree about what "the build" is:

```yaml
- run: ./run.sh test          # or verify, when the project has integration tests
```

Details that are decisions, not defaults:

- **JDK.** From `{{.JavaVersion}}`, the same answer `run.conf` pins, so there is
  no second place to bump.
- **`test` or `verify`.** `verify` when `.E2E` is on. The repository's own workflow
  chose `test` for the generated-project job for a stated reason — it skips the
  production frontend build and the browser download — but that reasoning is about
  *this* repository verifying templates. A project's own CI is exactly where the
  browser tests should run, since a laptop is where they get skipped.
- **Traceable builds.** With `--traceable`, the build refuses to run without
  `-Dbuild.commit`. `run.sh` supplies it from `git rev-parse HEAD`, which works on
  a normal checkout — but `actions/checkout` defaults to a shallow fetch, and the
  workflow has to be checked against that rather than assumed to be fine.
- **The dev stack.** A project with `--database` runs its tests against
  Testcontainers, which needs a container runtime; the GitHub-hosted Linux runners
  have one. `--auth` puts Keycloak in the dev *stack*, but its integration test
  (`ProtectedRootIT`) deliberately needs no identity provider — worth confirming
  before the workflow tries to bring anything up.
- **Caching.** `actions/setup-java` caches the Maven repository with one line.
  Without it a Vaadin build's first job is slow enough that people start skipping
  CI, which defeats the exercise.

## Also

A GitHub workflow is a bet that the project lives on GitHub. That is true often
enough to be a defensible default, and the generated README — the file that
answers "why is it like this?" — is where to say the bet was made and that
deleting the file is the whole of backing out of it.

## Test

- `internal/generate`: the workflow is generated for every combination, its YAML
  parses, and it names `verify` exactly when the project has integration tests.
  A YAML parse is the counterpart of the pom's existing well-formed-XML check.
- The repository's own `generated-project` CI job could go one step further and
  run the generated workflow's build command rather than an equivalent one it
  spells out itself — then the generated project's CI is verified by this
  repository's CI, and the two cannot drift.
