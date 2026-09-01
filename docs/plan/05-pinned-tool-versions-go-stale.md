# The versions the templates pin by hand go stale, and nothing notices

## Where it stands

The tool goes to real lengths to keep the two headline versions current:
`internal/versions` reads them from Maven Central at startup, overlapping the
request with the first questions, precisely because "a hard-coded one is wrong the
week after it ships".

Every other version in the generated build is hard-coded and gets none of that
care:

| Where | What |
| --- | --- |
| `templates/pom.xml.tmpl:19` | `<playwright.version>1.59.0</playwright.version>` |
| `templates/pom.xml.tmpl:23` | `<jacoco.version>0.8.13</jacoco.version>` |
| `templates/pom.xml.tmpl:401` | `maven-surefire-report-plugin` `3.5.5` |

They are pinned for a good reason — none of the three is managed by the Spring
Boot BOM, and an unpinned plugin version makes the build non-reproducible. The
problem is not the pinning; it is that nothing in the repository will ever tell
anyone they have drifted. A project generated two years from now gets a
two-year-old Playwright, and the first sign is a browser that the current
Playwright driver protocol no longer matches.

## What to do

Do not extend the startup lookup to cover them. It is on the user's critical path,
it is allowed five seconds for two requests, and a browser-test dependency is not
worth a third — the tool's own comment about what is worth making the user wait
for applies.

Instead, make drift visible in this repository, where it can be fixed once for
everyone:

1. **Name them in one place.** Move the three into a small block at the top of
   `templates/pom.xml.tmpl`'s `<properties>` with a comment saying they are pinned
   here because nothing manages them, and that a scheduled check watches them.
2. **Add a scheduled CI job** — `.github/workflows/versions.yml`, `schedule:`
   weekly, `workflow_dispatch:` too — that reads each artifact's
   `maven-metadata.xml` from Maven Central (the same documents
   `internal/versions` already parses, so the parsing is a package away) and fails
   with a list of what is behind. A failing scheduled run is the notification;
   opening a pull request automatically is a nicety that can come later.
3. **While there, check the defaults file's fallbacks** — `vaadin_version` and
   `boot_version` in `defaults.toml` are described as "the answer of last resort"
   and go stale the same way. They are the one case where staleness is silent for
   the user too: an offline run generates a project on them, and the prompt's
   `versionNote` says so, but nobody reads that as "this is a year old".

## Test

The check is CI, not a unit test — a test that talks to Maven Central would make
`go test ./...` fail on an aeroplane, which is exactly the failure mode
`internal/versions` was written to avoid. The version-comparison code it needs is
already tested (`internal/versions/versions_test.go` drives `stableVersions` from
a local server); reuse it rather than re-implementing the parse in shell.
