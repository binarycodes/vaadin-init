# Nothing keeps a generated project's dependencies current after day one

## Where it stands

The tool works hard to start a project on the current release: the Vaadin and
Spring Boot versions are looked up from Maven Central at startup rather than
hard-coded, because "a bootstrap tool's headline default is the framework
version, and a hard-coded one is wrong the week after it ships".

That care stops at generation. The project is created on the newest release and
then has nothing to keep it there — no Dependabot configuration, no Renovate
configuration, and no CI to run their pull requests against (see
`07-generated-project-gets-no-ci.md`). Six months later it is a project pinned to
whatever was current the day it was made, which is the exact state the version
lookup exists to prevent.

This repository now has `.github/dependabot.yml` for its own actions. Extending
the same courtesy to what it generates is a small template.

## What to do

Generate `.github/dependabot.yml` alongside the workflow from
`07-generated-project-gets-no-ci.md` — the two are worth doing together, because
dependency pull requests with no CI to verify them are noise:

```yaml
version: 2
updates:
  - package-ecosystem: maven
    directory: /
    schedule:
      interval: monthly
    groups:
      spring-and-vaadin:
        patterns: ['com.vaadin:*', 'org.springframework.boot:*']
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: monthly
```

Two decisions worth making deliberately rather than inheriting:

- **Grouping.** Vaadin and Spring Boot both arrive as a BOM plus a parent, and
  ungrouped updates produce a pull request per artifact that cannot be merged
  independently. Grouping them is what makes the updates reviewable.
- **Cadence.** Monthly, not weekly. A generated project has one developer at
  first, and a weekly pull request nobody merges trains them to ignore the whole
  channel.

Ship it unconditionally, like the `.gitignore`, and say in the generated README
that it assumes GitHub — the same caveat the CI workflow needs, in the same
paragraph.

## Test

`internal/generate`: the file is generated for every combination and parses as
YAML. Whether the ecosystems are right is not something a test can know; the
grouping patterns matching the artifacts the pom actually declares is, and is
worth an assertion if the patterns ever get more specific than the two above.
