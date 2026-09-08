# Map a Vaadin version to the Spring Boot and JDK it goes with, from a data file

## Where it stands

The tool asks for three versions and treats them as three unrelated answers.

- `internal/versions/versions.go` fixes the generation in two constants,
  `VaadinMajor = 25` and `BootMajor = 4`, and runs two independent Maven Central
  lookups filtered by them. Nothing says which Boot release a given Vaadin
  release was built against, so the TUI offers the five newest of each and the
  cursor lands on the newest of both — which happens to be right today and is
  wrong the week Boot releases a minor Vaadin has not caught up with.
- `config.ValidJavaVersion` (`internal/config/config.go:347`) enforces a floor
  of 17 and says "Spring Boot 4 needs Java 17 or newer". That is Boot's floor.
  Vaadin 25's is 21, so `--java-version 17` produces a project whose runtime
  requirement is not met, and the prompt's description
  (`internal/prompt/prompt.go:408`) tells the user the wrong number.
- Nothing ties the pair together. Vaadin 25.2.6 with Boot 4.0.8 passes
  validation, although the 25.2.0 release notes say 4.1 is required. Vaadin
  25.1.x with Boot 4.0.0 passes, although 25.1 needs 4.0.4 for Jackson 3.1.
- No ceiling exists for Java at all: a JDK Boot has not yet been made compatible
  with is accepted without comment.

Plan 04 proposes rejecting a version from the wrong major line. This plan
subsumes it: the same check falls out of the compatibility rules, and the rules
belong in data rather than in a validator with two integers baked into it.

## How start.vaadin.com does it

Read from the served frontend bundle and from what the generator returns, on
2026-09-08.

**The frontend hard-codes its choices at build time.** The Vaadin drop-down is a
literal list of minor lines — `Vaadin 25.2` and `Vaadin 25.3 (pre)`, ids `v25.2`
and `v25.3` — not patch versions. The Java drop-down is a literal map,
`{21: "LTS", 25: "LTS", 26: ""}`, default 25. A `fixJavaVersion` method snaps a
saved configuration with a Java version outside that map back to 17 — a value
the map no longer contains, left over from the Vaadin 24 era. There is no
compatibility API; the UI never asks the server what goes with what.

**The backend resolves the line to a release and picks Boot itself.** A request
for `v25.2` today yields a pom with `vaadin.version` 25.2.6 and
`spring-boot-starter-parent` 4.1.0. The `/skeleton` endpoint ignores a platform
version it does not serve — `platformVersion=24.10.9` still returned 25.2.6 — and
passes the Java version through unchecked: `javaVersion=17` with Vaadin 25 is
written into the pom as asked. So the service validates nothing beyond what the
drop-downs make impossible.

**The Boot number comes from the platform build, not from a table.** The
vaadin/platform `pom.xml` has a single `spring.boot.version` property, and the
published `vaadin-spring-boot-starter-<v>.pom` on Maven Central carries it as a
literal dependency version — of `spring-boot-starter-web` on 24, of
`spring-boot-starter-webmvc` on 25. The generated project's Boot always matched
it:

| Vaadin | Boot the starter pins | Java floor in the release notes | Boot requirement in the release notes |
| --- | --- | --- | --- |
| 24.8.0 | 3.5.0 | 17 | 3.x (docs: 3.5 or later from the 3.x series) |
| 24.9.0 | 3.5.5 | 17 | 3.5.x |
| 24.10.0 | 3.5.11 | 17 | 3.5 or later from the 3.x series |
| 24.10.9 | 3.5.15 | 17 | — |
| 25.0.0 | 4.0.0 | 21 | Spring Boot 4 |
| 25.1.0 | 4.0.4 | 21 | 4.0.4 required, for Jackson 3.1 |
| 25.2.0 | 4.1.0 | 21 | 4.1 required |
| 25.2.6 | 4.1.0 | 21 | — |
| 25.3.0-beta1 | 4.1.1 | — | — |

Boot's own Java range, from its system-requirements page per line: 3.5.x is 17
to 25, 4.0.x and 4.1.x are 17 to 26. Boot support windows (endoflife.date): 3.5
ended 2026-06-30, 4.0 ends 2026-12-31, 4.1 ends 2027-07-31.

**What to take from it.** There are two different kinds of fact here:

1. *Which exact Boot a Vaadin release was built against* is machine-readable and
   versioned — one small pom on Maven Central per Vaadin release. It can be
   looked up, never goes stale, and is exactly the answer start.vaadin.com
   writes into its pom.
2. *The rules* — Java floors, Boot minimums per Vaadin minor, Boot's Java
   ceilings, which JDKs are LTS — exist only as prose in release notes and docs
   pages. Nobody publishes them as data. Someone has to write them down, and
   they change a handful of times a year, at a Vaadin minor or a Boot minor.

start.vaadin.com writes the second kind into its JavaScript and rebuilds. The
leftover snap-to-17 is what that costs. The rules should live in a data file
this repository owns, that a pull request can change without touching Go, and
that a scheduled job checks against the published poms.

## What to do

### 1. `compat.json`, next to `defaults.toml`

Embedded like `defaults.toml`, overridable the same two ways (a file under the
user config dir, `--compat <path>`), and later refreshable from the repository
(see 8). JSON rather than TOML because the same file is read by the CI check
with `jq` and, eventually, fetched over HTTP; the Go side is `encoding/json`
either way.

Rules only. No table of patch versions: that is what the lookup is for, and a
per-patch table is the thing that rots.

```json
{
  "vaadin": [
    {
      "line": "25",
      "supported": true,
      "java_min": 21,
      "boot_line": "4",
      "boot_min": "4.0.0",
      "since": [
        { "vaadin": "25.1.0", "boot_min": "4.0.4" },
        { "vaadin": "25.2.0", "boot_min": "4.1.0" }
      ],
      "source": "https://github.com/vaadin/platform/releases/tag/25.2.0"
    },
    {
      "line": "24",
      "supported": false,
      "java_min": 17,
      "boot_line": "3",
      "boot_min": "3.5.0",
      "source": "https://vaadin.com/docs/v24/compatibility"
    }
  ],
  "boot": [
    { "line": "4.1", "java_min": 17, "java_max": 26, "eol": "2027-07-31" },
    { "line": "4.0", "java_min": 17, "java_max": 26, "eol": "2026-12-31" },
    { "line": "3.5", "java_min": 17, "java_max": 25, "eol": "2026-06-30" }
  ],
  "java": { "lts": [17, 21, 25] }
}
```

`supported` is the line this tool has templates for — the replacement for the
two constants, so the day Vaadin 26 needs a second template set is a data change
plus templates, not a code hunt. `since` is how a mid-line tightening (25.1 → 4.0.4,
25.2 → 4.1) is expressed without splitting the line. `source` is for the person
editing the file, and for the CI job's failure message. `java_max` is inclusive.

### 2. `internal/compat`: the rules as pure functions

```go
type Rules struct{ … }

// Load decodes the embedded file, then layers an override the way
// config.LoadDefaults does. A malformed file is an error naming the field.
func Load(embedded []byte, explicitPath string) (Rules, error)

// Vaadin returns the rule for a version's line, or an error saying the line is
// not one this tool generates — which is plan 04.
func (r Rules) Vaadin(version string) (VaadinRule, error)

// BootMin is the lowest Boot the Vaadin version accepts, `since` applied.
func (r Rules) BootMin(vaadin string) (string, error)

// CompatibleBoot filters a Boot list to the line and minimum for a Vaadin version.
func (r Rules) CompatibleBoot(vaadin string, candidates []string) []string

// JavaRange is the intersection of Vaadin's floor and Boot's floor and ceiling.
func (r Rules) JavaRange(vaadin, boot string) (min, max int, err error)

// Check is every cross-field rule at once, for Config.Validate.
func (r Rules) Check(vaadin, boot string, java int) error
```

No network in this package. The version comparison it needs already exists in
`internal/versions` (`parseVersion`, `after`); export it or move it to a small
shared package rather than writing a second one.

### 3. `internal/versions`: the pinned Boot, and lines from data

- The two lookups filter by the lines `compat` marks `supported`, not by the
  constants. `VaadinMajor` and `BootMajor` go; `ui.Banner` takes its numbers from
  the rules.
- Add `PinnedBoot(ctx, client, vaadinVersion) (string, error)`: fetch
  `com/vaadin/vaadin-spring-boot-starter/<v>/vaadin-spring-boot-starter-<v>.pom`
  and return the version of the `org.springframework.boot` dependency it
  declares — `spring-boot-starter-web` or `-webmvc`, so match on the group and
  take whichever starter is there. The pom is a few kilobytes and the request
  fires when the Vaadin answer is known, so it is not on the critical path the
  package comment guards; it gets the same timeout and the same degrade-to-nothing
  contract. When it fails, the Boot default is the newest release
  `CompatibleBoot` allows.

### 4. The TUI

- **Boot select.** Options are `CompatibleBoot(chosen Vaadin, fetched list)`,
  newest first, and the pinned release is labelled and is where the cursor
  opens — `4.1.0 · built with Vaadin 25.2.6`. Choosing a different Vaadin
  version re-derives the list; today the two selects are independent groups, so
  the Boot group is built after the Vaadin answer, the way derived answers
  already follow the coordinates.
- **Java becomes a select**, not a free input: every major in `JavaRange`, LTS
  releases labelled, plus the existing type-one-myself escape hatch. The cursor
  opens on `defaults.toml`'s `java_version` when it is in range and on the
  newest LTS in range when it is not, with a one-line description saying why.
  start.vaadin.com defaults to 25 and offers 26; this tool keeps 21 as its
  default because the defaults file says so, and the file is where to change it.
- **Accessible mode** keeps inputs, with the same validators and a description
  that names the range for the versions already answered.
- The Java description's "Spring Boot 4 needs 17 or newer" becomes the computed
  range: `Vaadin 25 needs 21 or newer; Spring Boot 4.1 supports up to 26.`

### 5. Validation on both paths

`Config.Validate` gains one cross-field entry, `rules.Check(...)`, after the
per-field ones. It is what catches `--yes --vaadin-version 25.2.6 --boot-version
4.0.8` and `--java-version 17`. Messages name the rule and where it came from,
in the style of the enforcer message in `templates/pom.xml.tmpl`:

```
spring boot version: Vaadin 25.2.6 needs Spring Boot 4.1.0 or newer; got 4.0.8
  (https://github.com/vaadin/platform/releases/tag/25.2.0)
java version: Vaadin 25 needs Java 21 or newer; got 17
```

On the scripted path, when only `--vaadin-version` is given, Boot defaults to the
pinned release and Java to the default-in-range rule above — the same derivation
as the TUI, so `--yes` and the screen agree.

### 6. The summary

Unchanged, except one line under the stack row when a derived Boot was used:

```
stack    Vaadin 25.2.6 · Spring Boot 4.1.0 · Java 21
         Spring Boot 4.1.0 is the release Vaadin 25.2.6 was built with
```

### 7. Keeping the file true: the scheduled check

Extend plan 05's weekly job rather than adding a second one. It reads
`compat.json` — the same file, so adding a line is still one edit — and fails
when:

- the newest release of a `supported` Vaadin line has a starter pom whose Boot
  falls outside `boot_line`/`boot_min` (the pin has moved ahead of the rules,
  which is how a new `since` entry gets noticed);
- Maven Central lists a Vaadin minor or major with no rule (25.3 going GA);
- a `boot` line the rules mention is past `eol`, or a new Boot minor exists
  with no `java_max` recorded;
- the file fails its own schema.

The failure message quotes the `source` URL so the fix is a read and an edit.
Editing procedure, written at the top of the file: read the release notes for
the new line, add or tighten the entry, cite the URL, open a pull request.

### 8. Refreshing at runtime, later

Fetch `compat.json` from this repository's main branch alongside the two Maven
lookups, cache it with plan 26's cache, and keep the embedded copy as the
fallback. This is the step that makes the file maintainable without a release:
a rule added on Monday reaches a binary built in March. It depends on 26 and on
the file being stable enough to trust remotely, so it is last.

**Order:** 1, 2 and 5 first — the data, the rules, and the check that stops a
wrong project. Then 3 and 4, which change what is offered. Then 7, before the
first Vaadin minor lands without anyone noticing. 8 when 26 exists.

## As built

1 to 7 are done; 8 waits for 26. Where the implementation differs from the text
above, and why:

- **The Java list is the LTS releases in the range and the newest release in
  it**, not every major — `21 · LTS`, `25 · LTS`, `26` for 25.2.6 on 4.1.0 — with
  the rest behind the same "type one myself" hatch and accepted by the rules.
  Two reasons. A feature release older than the newest is out of support the day
  the next one ships, so a list of 22, 23 and 24 is a list of JDKs nobody should
  start a project on; and six rows plus a two-line description is what pushed the
  Versions column past a terminal that tiled. start.vaadin.com's own Java list is
  exactly LTS plus newest.
- **The scheduled check is Go, not jq** — `go run ./internal/checkcompat` — for
  the reason plan 05 gives: the version parse and the two Maven Central readers
  already exist and are tested, and the decision is a pure function over the
  fetched lists that is tested offline. JSON stays the format, for the fetch in 8.
  Plan 05's job did not exist yet, so `.github/workflows/versions.yml` is new and
  is where 05 adds its checks.
- **A Boot minor with no rule has no Java ceiling** rather than being refused: a
  new Boot minor is compatible by the Vaadin rule's line and minimum the day it
  ships, and refusing it until the file catches up would be the tool being wrong
  for a week. The list then runs to the newest Java any Boot rule names, and the
  weekly check is what asks for the missing rule.
- **The pin is checked against the rules before it is offered.** A starter pom
  that names a Boot the rules refuse is the rules being stale, which the check
  notices; until then the newest allowed release is offered, because a default
  the validator then refuses is worse than an older one it accepts.
- **The pinned label is `4.1.0 · built with 25.2.6`**, without "Vaadin": the
  longer label wraps in a column, and a wrapped option puts the list's rows and
  huh's viewport out of step.
- **huh scrolls a list so the bound value is its first row**, which hid the
  newer releases above the pin. The list is set with its first option bound and
  the cursor walked down to the answer instead, which scrolls only when the
  cursor leaves the view — and with every option in view it never does.
- **`Config.Validate` takes the rules** as a parameter rather than reaching for
  a package-level value, and the rules are passed to the prompt through
  `prompt.Options`, beside the pin source.
- The version parse moved to `internal/version`, since `internal/versions`
  cannot import the rules that need it and the rules cannot import the lookup.

## Test

- `internal/compat`: table tests over the example file — `Vaadin("24.10.9")`
  errors as unsupported; `BootMin("25.0.5")` is 4.0.0, `("25.1.0")` is 4.0.4,
  `("25.2.6")` is 4.1.0; `JavaRange("25.2.6", "4.1.0")` is 21–26 and
  `("25.2.6", "4.0.8")` errors before Java is looked at; `java_max` is
  inclusive; an override file layered over the embedded one wins per line; a file
  with `boot_min` unparsable is rejected naming the entry.
- `internal/versions`: `PinnedBoot` against the local test server serving two
  captured starter poms, one `-web` (24) and one `-webmvc` (25); a 404 yields
  `""` and no error the caller has to handle differently.
- `internal/config`: `Validate` rejects 25.2.6 + 4.0.8, 25.2.6 + Java 17,
  24.10.9 on any Boot, 4.1.0 + Java 27; accepts 25.2.6 + 4.1.0 with 21 and 25.
  One end-to-end assertion that `--yes` with a rejected pair fails before
  anything is written, as plan 04 already asks for.
- `internal/prompt`: the Boot list opens on the pinned release; the Java list
  holds exactly the range; an out-of-range default snaps and says so.
- The schema and freshness checks are CI, not `go test`, for the reason plan 05
  gives.

## Sources

- start.vaadin.com served bundle, `VAADIN/build/indexhtml-*.js` and
  `preview-source-*.js` — platform lines, Java map, `fixJavaVersion`,
  `/skeleton` parameters.
- `https://start.vaadin.com/skeleton?platformVersion=…&javaVersion=…` and
  `/dl?preset=latest-java` — generated poms.
- `https://github.com/vaadin/platform/blob/<tag>/pom.xml` — `spring.boot.version`.
- `https://repo1.maven.org/maven2/com/vaadin/vaadin-spring-boot-starter/<v>/…pom`.
- Platform release notes, "Supported technologies", tags 24.7.0 to 25.2.0.
- `https://vaadin.com/docs/latest/compatibility`, `https://vaadin.com/docs/v24/compatibility`.
- `https://docs.spring.io/spring-boot/{3.5,4.0,4.1}/system-requirements.html`.
- `https://endoflife.date/api/spring-boot.json`.
