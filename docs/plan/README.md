# Plans

One file per improvement idea. Each says where the code stands today, with file
and line, what to do about it, and how it would be tested. Nothing here has been
implemented; a file is a proposal, not a record.

| | | |
| --- | --- | --- |
| 01 | [Theme choice: Aura or Lumo](01-theme-choice-lumo-or-aura.md) | The generated project is hard-wired to Lumo. Aura is Vaadin 25's default and should be this tool's. |
| 02 | [Room for answers that are not yes-or-no](02-non-boolean-answers-in-the-tui.md) | Every optional decision is a boolean in four places at once. The theme choice is the first that does not fit. |
| 03 | [Color scheme](03-color-scheme-light-dark.md) | Nothing generated says anything about light or dark. Follows from 01. |
| 04 | [Reject an unsupported framework version](04-version-compatibility-validation.md) | `--vaadin-version 24.4.0` renders a project that cannot build, and nothing says so until Maven does. |
| 05 | [Hand-pinned tool versions go stale](05-pinned-tool-versions-go-stale.md) | Playwright, JaCoCo and the surefire report plugin are pinned by hand and watched by nobody. |
| 06 | [The coverage gate covers nothing](06-coverage-gate-is-vacuous-in-most-projects.md) | Its include patterns match no package in half the configurations, and a JaCoCo rule that matches nothing passes. |
| 07 | [The generated project ships no CI](07-generated-project-gets-no-ci.md) | The hook and the coverage gate are conventions that evaporate on a pull request. |
| 08 | [Compiling a generated project without pushing](08-verifying-generated-projects-outside-ci.md) | The likeliest class of mistake — a wrong name in a Java template — is caught only by CI. |
| 09 | [No production packaging](09-no-production-packaging.md) | Considered dev environment, a jar, and then nothing — including for the commit SHA a traceable build demands. |
| 10 | [The summary promises a build that cannot run](10-the-summary-promises-a-build-that-cannot-run.md) | `--traceable --no-git` prints next steps whose first command exits 1. |

01 is the one with a user waiting for it; 04, 06 and 10 are corrections to things
that are quietly wrong today; the rest are additions.
