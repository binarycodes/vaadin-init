# A generated project can only be compiled by pushing

## Where it stands

The split is stated plainly in the repository README: the tests in this repository
render the templates and check their shape — well-formed XML, valid JSON, no
unresolved template values, every Java file declaring the package its path implies
— and the `generated-project` CI job is "the only place a generated project is ever
compiled".

That is the right division. The consequence is that a one-character mistake in a
Java template — a wrong class name, a missing import, a stray brace — passes
`make check` on a laptop and is caught minutes later by a push. For a repository
whose entire output is Java and XML that this repository never compiles, the
feedback loop for the most likely class of mistake is a git push.

## What to do

Add a make target that does locally what CI does, and skips itself when the
machine cannot:

```make
# Compiles a generated project with a real JDK — the only check that catches a
# wrong class name in a Java template, which every test here is blind to.
# Skipped, not failed, without a JDK: the tool itself needs no Java to build.
check-generated:
```

Shape it after the CI job so there is one procedure, not two: build the binary,
generate into a temporary directory twice — once bare, once with every option —
and run `./run.sh test` in each.

Two things make this worth the target rather than a paragraph in the README:

- **It should skip, not fail.** The tool is a Go program; requiring a JDK to run
  `make check` would be a new prerequisite for contributors who are only touching
  the TUI.
- **It should not be in `check`.** `check` is `fmt vet test` and runs in seconds;
  `dist` depends on it. A Maven build with a frontend compile does not belong in
  that path.

## Worth considering alongside

**Golden files for one or two configurations.** The current tests assert
properties — parses, package matches path, option implies file. They cannot catch
a change that is well-formed and wrong: a dependency accidentally deleted, an
`argLine` that loses `@{argLine}`, a comment block that silently swallowed a
conditional. A committed rendering of the bare config and the everything-on
config, refreshed by `make golden`, makes every such change visible in review as a
diff.

The cost is real — a golden file is a second place to update on purpose, and a
noisy one when versions change. Mitigate it by rendering with fixed versions
rather than looked-up ones, so the golden files only move when a template does.
