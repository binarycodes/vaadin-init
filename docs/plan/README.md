# Plans

One file per improvement idea. Each says where the code stands today, with file
and line, what to do about it, and how it would be tested. Nothing here has been
implemented; a file is a proposal, not a record.

## The generated project

| | | |
| --- | --- | --- |
| 01 | [Theme choice: Aura or Lumo](01-theme-choice-lumo-or-aura.md) | Hard-wired to Lumo. Aura is Vaadin 25's default and should be this tool's. |
| 03 | [Color scheme](03-color-scheme-light-dark.md) | Nothing generated says anything about light or dark. Follows from 01. |
| 06 | [The coverage gate covers nothing](06-coverage-gate-is-vacuous-in-most-projects.md) | Its includes match no package in half the configurations, and a JaCoCo rule that matches nothing passes. |
| 07 | [No CI](07-generated-project-gets-no-ci.md) | The hook and the coverage gate are conventions that evaporate on a pull request. |
| 09 | [No production packaging](09-no-production-packaging.md) | A considered dev environment, a jar, and then nothing — including for the commit SHA a traceable build demands. |
| 13 | [No `.gitattributes`](13-generated-projects-have-no-gitattributes.md) | The CRLF breakage this repository protects itself from is passed straight on. |
| 16 | [Container images pinned twice](16-container-image-versions-are-duplicated.md) | Only a javadoc keeps `compose.yaml` and Testcontainers on the same PostgreSQL. |
| 18 | [Login without logout](18-auth-projects-can-log-in-but-never-log-out.md) | `--auth` generates a complete sign-in and no way out of it. |
| 19 | [The slice has no validation or error handling](19-the-worked-slice-has-no-validation-or-error-handling.md) | 255 lives in one place, furthest from the user, and nothing shows what a failure looks like. |
| 20 | [One properties file for every environment](20-one-properties-file-for-development-and-production.md) | Dev credentials and DEBUG logging in the file that ships. |
| 21 | [No dependency updates](21-generated-projects-get-no-dependency-updates.md) | Started on the newest release, then nothing keeps it there. |
| 30 | [No format or static-analysis gate](30-no-format-or-static-analysis-gate-in-the-generated-project.md) | The tool gates its own formatting and hands the project nothing. |

## The task runner

| | | |
| --- | --- | --- |
| 11 | [bash 4 syntax, and macOS ships 3.2](11-run-sh-needs-bash-4-and-macos-ships-3-2.md) | `./run.sh` fails on line 47 on a stock Mac, and no generated project is ever run on macOS in CI. |
| 12 | [Bash-only on Windows](12-the-generated-project-is-bash-only-on-windows.md) | The tool ships a Windows binary and generates a project PowerShell cannot drive. |
| 14 | [It describes things the project does not have](14-run-sh-describes-things-the-generated-project-does-not-have.md) | Lombok, quadlets and a preview pane that is not wired up. |
| 15 | [Nothing checks it](15-nothing-checks-the-shell-script-that-ships-everywhere.md) | 551 lines of bash in every project, and no shellcheck, no parse check, one task ever run in CI. |

## The tool

| | | |
| --- | --- | --- |
| 02 | [Room for answers that are not yes-or-no](02-non-boolean-answers-in-the-tui.md) | Every optional decision is a boolean in four places at once. |
| 04 | [Reject an unsupported framework version](04-version-compatibility-validation.md) | `--vaadin-version 24.4.0` renders a project that cannot build, silently. |
| 05 | [Hand-pinned tool versions go stale](05-pinned-tool-versions-go-stale.md) | Playwright, JaCoCo and the surefire report plugin are watched by nobody. |
| 08 | [Compiling a generated project without pushing](08-verifying-generated-projects-outside-ci.md) | The likeliest class of mistake is caught only by CI. |
| 10 | [The summary promises a build that cannot run](10-the-summary-promises-a-build-that-cannot-run.md) | `--traceable --no-git` prints next steps whose first command exits 1. |
| 17 | [Input reaches five syntaxes unescaped](17-user-input-is-injected-into-five-syntaxes-unescaped.md) | `R&D` in a description produces a `pom.xml` Maven refuses to read. |
| 25 | [The logic with no tests](25-main-go-has-the-logic-and-none-of-the-tests.md) | Every `internal` package is tested; `main.go`, which decides what gets generated, is not. |
| 26 | [The lookup is not cached, and its fallback is silent](26-the-version-lookup-is-not-cached-between-runs.md) | A scripted offline run pins months-old versions and says nothing. |
| 27 | [`~/projects/app` creates a directory called `~`](27-a-typed-path-with-a-tilde-creates-a-directory-called-tilde.md) | A shell would have expanded it; a prompt does not. |
| 28 | [No way back, no way to replay](28-there-is-no-way-back-and-no-way-to-replay.md) | A wrong group id means starting over, and a good run leaves no record of itself. |
| 29 | [A half-written project blocks its own retry](29-a-half-written-project-blocks-its-own-retry.md) | Rendering is atomic; writing is not, which is the case the package comment warns about. |

## The repository

| | | |
| --- | --- | --- |
| 22 | [No licence anywhere](22-no-license-anywhere.md) | The README points at downloads nobody is licensed to use. |
| 23 | [The hook cites a file that is not there](23-the-commit-hook-cites-a-file-that-is-not-there.md) | And its near-identical twin in `templates/` can drift with nothing checking. |
| 24 | [There is no release](24-there-is-no-release.md) | "Download the binary from the releases" — there are none, and no workflow makes any. |

## If only a few get done

01 is the one with a user waiting for it. 11, 17 and 13 are live breakage:
a generated project that does not start on macOS, an ampersand in a description
that stops Maven, and a Windows clone that grows carriage returns. 04, 06, 10 and
23 are quietly wrong today. The rest are additions.
