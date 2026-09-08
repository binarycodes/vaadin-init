# Requirements

What `vaadin-init` is and does, as implemented. Every statement here is a
requirement: an implementation that satisfies all of them is this tool.

## 1. Purpose and shape

1.1 `vaadin-init` bootstraps an opinionated Vaadin and Spring Boot project.

1.2 It is a single Go binary with no runtime dependencies. Templates and the
default answers are embedded in the binary (`//go:embed all:templates`,
`//go:embed defaults.toml`); nothing is read from beside the executable.

1.3 It runs in three modes: a full-screen TUI (the default when attached to a
terminal), plain sequential prompts (`--accessible`), and no prompts at all
(`--yes`).

1.4 Packages:

| Package | Holds |
| --- | --- |
| `main` | flags, embedding, process control, the non-interactive path |
| `internal/config` | the answers, and what counts as a valid answer |
| `internal/compat` | the rules in `compat.json`: which Spring Boot and JDK go with a Vaadin release |
| `internal/version` | parsing and comparing Maven version numbers |
| `internal/versions` | the Maven Central lookup, the pinned Spring Boot, and the fallback |
| `internal/prompt` | the TUI |
| `internal/generate` | the manifest, and rendering it to disk |
| `internal/ui` | the palette, the huh theme, and every renderer |
| `internal/checkcompat` | the CI check of `compat.json` against Maven Central |
| `templates/` | what gets generated |

1.5 `internal/prompt` never writes files and never starts processes. Writing the
project and running a task are functions passed in through `prompt.Options`.

## 2. Command line

2.1 Usage:

```
vaadin-init                     ask, then generate
vaadin-init --yes [flags]       generate without asking
```

2.2 Flags. Defaults shown by `--help` are this machine's, after the defaults file
has been read.

| Flag | Default | Effect |
| --- | --- | --- |
| `--defaults <path>` | — | read defaults from this file instead of the per-user one |
| `--compat <path>` | — | read the compatibility rules from this file instead of the per-user one |
| `--version` | — | print `vaadin-init <version>` and exit 0 |
| `--group-id` | defaults file | Maven group id |
| `--artifact-id` | defaults file | Maven artifact id |
| `--name` | derived | project name |
| `--package` | derived | base Java package |
| `--description` | defaults file | project description |
| `--vaadin-version` | newest found | Vaadin version |
| `--boot-version` | derived (§2.6) | Spring Boot version |
| `--java-version` | defaults file, moved into range (§2.6) | JDK major version the build pins |
| `--theme` | defaults file | Vaadin theme the project loads: `aura` or `lumo` |
| `--dir` | the artifact id | where to write the project |
| `--app-port` | drawn from the range | port the application listens on |
| `--db-port` | drawn from the range | host port the dev stack's PostgreSQL is published on |
| `--auth-port` | drawn from the range | host port the dev stack's Keycloak is published on |
| `--author-name` | what git has configured | name for the first commit, kept in the new repository |
| `--author-email` | what git has configured | email for the first commit, kept in the new repository |
| `--database` | defaults file | PostgreSQL, Flyway, JPA, Testcontainers, dev compose |
| `--auth` | defaults file | OIDC login against Keycloak in the dev stack |
| `--e2e` | defaults file | Playwright browser tests behind an `it` profile |
| `--coverage` | defaults file | a JaCoCo coverage gate |
| `--traceable` | defaults file | require every build to carry its commit SHA |
| `--yes` | false | skip the prompts, generate from flags and defaults |
| `--force` | false | write into a target directory that is not empty |
| `--no-git` | false | do not touch git at all |
| `--no-commit` | false | set the repository up, make no commit |
| `--dry-run` | false | list what would be written, write nothing |
| `--accessible` | `ACCESSIBLE` env set | plain sequential prompts, for screen readers |

2.3 `--defaults`, `--compat` and `--version` are parsed before the other flags are
defined, because the defaults file supplies their default values. That pre-parse
ignores unknown flags; the real parse reports them.

2.4 A positional argument is an error: `unexpected argument %q; every setting is
a flag (see --help)`.

2.5 `--name`, `--package` and `--dir` are derived from the coordinates unless the
user typed them. Whether a flag was typed is read from `flag.FlagSet.Visit`, not
from comparing values.

2.6 A version flag is used as typed. Left out, the three are derived in order, in
the TUI and in `--yes` alike: Vaadin is the newest release the lookup found;
Spring Boot is the release that Vaadin was built with (§5.6) when the rules allow
it, otherwise the newest release they allow; Java is the defaults file's when the
pair allows it, otherwise the newest LTS release the pair allows; the theme is the
defaults file's when the Vaadin line ships it, otherwise the line's own default.
A `--yes` run with all three versions typed fetches nothing, and the theme then
stays as named.

2.7 `--author-name` and `--author-email` are for a machine whose git has no
identity to commit with. Given, they are written to the new repository's own
config before the first commit; not given, `generate.CurrentAuthor(dir)` asks git
who it would commit as in the output directory, and the TUI asks for whichever
half is missing (§6.2.6). Git's
global configuration is never written.

2.8 `--app-port`, `--db-port` and `--auth-port` default to three ports drawn from
the defaults file's range (§3.4) when the defaults are loaded, so `--help` shows
this run's draw; a flag replaces one draw.

2.9 Exit codes: `0` success; `130` after a cancelled conversation, with
`Cancelled. Nothing was written.` on stderr; `1` on any other error, with
`✗ <error>` on stderr.

## 3. Defaults file

3.1 The embedded `defaults.toml` is decoded first, then a user file is layered
over it, so a personal file names only what it changes.

3.2 The user file is `<os.UserConfigDir>/vaadin-init/defaults.toml`. A missing
file there is normal and silent. A file named by `--defaults` is an instruction:
missing or unparsable is an error naming the path.

3.3 Shape and shipped values:

```toml
group_id = "com.example"
artifact_id = "my-app"
description = "A Vaadin application"
java_version = "21"
vaadin_version = "25.2.6"
boot_version = "4.1.1"
theme = "aura"

[ports]
from = 49000
to = 51000

[features]
database = true
auth = true
e2e = true
coverage = true
traceable = false
```

3.4 `Defaults.ToConfig()` fills in the derived fields, so `--yes` produces exactly
what the TUI would have offered.

3.5 `[ports]` is the range each project's three ports are drawn from.
`ToConfig()` calls `PickPorts(from, to, 3)`: a random permutation of the range,
taking ports nothing on this machine is listening on (`net.Listen` on
`127.0.0.1`) and falling back to busy ones only when fewer than three are free.
Random rather than lowest-free so that projects generated while the stack is down
do not all land on the same port. A range that is not within 1024–65535, or has
room for fewer than three ports, is refused when the file is loaded, naming the
file.

3.5 `compat.json`, embedded beside `defaults.toml`, holds the compatibility rules
(§4.5). It is layered the same way — a file at
`<os.UserConfigDir>/vaadin-init/compat.json`, or `--compat <path>` — except that
the unit of replacement is a line: a `vaadin` or `boot` entry in the user file
replaces the embedded entry with the same `line` and other entries are kept, and
`java.lts` replaces the list whole when given. Shape and shipped values:

```json
{
  "vaadin": [
    { "line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "4.0.0",
      "since": [ { "vaadin": "25.1.0", "boot_min": "4.0.4" }, { "vaadin": "25.2.0", "boot_min": "4.1.0" } ],
      "themes": ["aura", "lumo"],
      "source": "https://github.com/vaadin/platform/releases/tag/25.2.0" },
    { "line": "24", "supported": false, "java_min": 17, "boot_line": "3", "boot_min": "3.5.0",
      "themes": ["lumo"],
      "source": "https://vaadin.com/docs/v24/compatibility" }
  ],
  "boot": [
    { "line": "4.1", "java_min": 17, "java_max": 26, "eol": "2027-07-31", "source": "…" },
    { "line": "4.0", "java_min": 17, "java_max": 26, "eol": "2026-12-31", "source": "…" },
    { "line": "3.5", "java_min": 17, "java_max": 25, "eol": "2026-06-30", "source": "…" }
  ],
  "java": { "lts": [17, 21, 25] }
}
```

`supported` marks the lines this tool has templates for. `since` tightens
`boot_min` from a Vaadin release onwards. `themes` lists the themes the line
ships, its default first. `java_max` is inclusive. A `$comment` key carries the
editing procedure. A file is decoded strictly — an unknown key is an error — and
every entry is checked: lines look like `25` or `4.1`, `boot_min` and each
`since` are versions inside their line, `themes` is not empty, Java ranges are
ranges, `eol` is a date, no line is listed twice, `lts` is not empty. Each
message names the entry.

## 4. The answers

4.1 `config.Config` is the only value templates are rendered against:
`GroupID`, `ArtifactID`, `ProjectName`, `Description`, `Package`, `JavaVersion`,
`VaadinVersion`, `BootVersion`, `Theme`, `Database`, `Auth`, `E2E`, `Coverage`,
`Traceable`, `OutputDir`, `AppPort`, `DatabasePort`, `AuthPort`, `AuthorName`,
`AuthorEmail`. `Theme` is `aura` or `lumo` (`config.ThemeAura`, `config.ThemeLumo`)
and always one of them; it is not an optional piece and `Selected()` does not
list it. The three ports are always set, whether or not the stack piece is
generated. The author fields are empty unless git had no identity of its own;
empty, nothing is written to the repository.

4.2 Derivations:

- `DeriveProjectName("book-shelf")` → `Book Shelf`. Split on `-`, `_` and `.`,
  each word's first rune upper-cased, joined with spaces.
- `DerivePackage(group, artifact)` → group + `.` + the artifact lower-cased with
  every non-letter, non-digit removed. If the artifact cleans to nothing, or the
  group already ends in that suffix, the group is returned unchanged.
- `OutputDir` defaults to the artifact id.

4.3 Validation, applied field by field in the TUI and over the whole `Config`
before generating:

| Field | Rule |
| --- | --- |
| group id | `^[a-z][a-z0-9]*(\.[a-z0-9][a-z0-9_]*)*$`, non-empty |
| artifact id | `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`, non-empty |
| project name | non-empty after trimming |
| package | every `.`-separated segment matches `^[a-z_][a-z0-9_]*$` and is not one of the 50 Java keywords (`_` included) |
| java version | a positive integer; the range is §4.5's question |
| vaadin / boot version | `^\d+\.\d+(\.\d+)?(-[A-Za-z0-9.]+)?$`, non-empty |
| theme | exactly `aura` or `lumo` |
| output directory | non-empty |
| app / database / auth port | each 1024–65535, and the three distinct |
| author name | when set, non-empty after trimming |
| author email | when set, `^[^\s@<>]+@[^\s@<>]+$` |

4.4 Values derived for the templates:

| Method | Value |
| --- | --- |
| `PackagePath()` | the package with `.` replaced by `/` |
| `Aura()`, `Lumo()` | `Theme == "aura"`, `Theme == "lumo"` |
| `ThemeName()` | `Aura` or `Lumo`, for the summary and the generated README |
| `ContainerRequired()` | `Database \|\| Auth` |
| `ITProfile()` | `it` when `E2E`, otherwise empty |
| `CommitProperty()` | `build.commit` when `Traceable`, otherwise empty |
| `BrowserTests()` | `E2E && !Auth` |
| `ProtectedRootTest()` | `E2E && Auth` |
| `ContainerPrefix()` | artifact id lower-cased, `-dev` appended |
| `DatabaseName()` | artifact id lower-cased, `-` replaced by `_` |
| `Selected()` | the options that are on, named `database`, `auth`, `e2e`, `coverage`, `traceable builds` |

4.5 The versions are checked together, after the per-field rules, by
`compat.Rules.Check`; `Config.Validate` takes the rules. A version's line is the
longest `line` it is in. In order:

| Rule | Message |
| --- | --- |
| the Vaadin line has a rule and is `supported` | `vaadin version: this tool generates Vaadin 25 projects; got 24.10.9, and Vaadin 24 sits on Spring Boot 3` |
| the Boot is in the line's `boot_line` | `spring boot version: Vaadin 25 sits on Spring Boot 4; got 3.5.15` |
| the Boot is at least `boot_min`, with every `since` at or below the Vaadin applied | `spring boot version: Vaadin 25.2.6 needs Spring Boot 4.1.0 or newer; got 4.0.8` |
| Java is at least the Vaadin line's `java_min` | `java version: Vaadin 25 needs Java 21 or newer; got 17` |
| Java is within the Boot line's range, when that line has a rule | `java version: Spring Boot 4.1 supports Java up to 26; got 27` |
| the theme is one the Vaadin line ships | `theme: Vaadin 25 ships aura and lumo, not material` |

Each message ends with the rule's `source` on a second line, in parentheses. A
Boot minor with no rule has no Java ceiling: nobody has written one down, and the
check in §11.4 is what notices.

The same rules give the derivations: `BootDefault` (the pin if the rules allow
it, else the newest allowed candidate, else nothing), `JavaRange` (the higher of
the two floors, up to the Boot ceiling), `JavaDefault` (the preferred release if
in range, else the newest LTS in range, else the floor), `Describe` (*21 or
newer for Vaadin 25, up to 26 for Spring Boot 4.1.*), `Themes` (the line's
themes, default first) and `ThemeDefault` (the preferred theme if the line ships
it, else the line's default).

## 5. Version lookup

5.1 At startup, in the background, the tool reads two `maven-metadata.xml`
documents:

- `https://repo1.maven.org/maven2/com/vaadin/vaadin-bom/maven-metadata.xml`
- `https://repo1.maven.org/maven2/org/springframework/boot/spring-boot-starter-parent/maven-metadata.xml`

Both requests run concurrently. The response body is read through a 4 MiB limit.

5.2 Only releases of the lines the rules mark `supported`, and the Boot lines
those sit on, are kept — `25` and `4` as shipped. A version with a qualifier is a
pre-release and is skipped; a version that does not parse is skipped rather than
guessed at. An empty list of lines keeps every release, for §11.4.

5.3 Kept versions are sorted newest first by major, minor, patch as numbers, and
all of them are returned; the prompt offers the first `offered` = 5 that the
rules allow.

5.4 The whole lookup has 5 seconds (`lookupTimeout`, also the HTTP client
timeout). It never fails: an empty list is a normal outcome and means the caller
keeps the version the defaults file named.

5.5 The lookup is started before the questions and waited on once, whichever path
asks for it.

5.6 `PinnedBoot(vaadin)` reads
`https://repo1.maven.org/maven2/com/vaadin/vaadin-spring-boot-starter/<v>/vaadin-spring-boot-starter-<v>.pom`
and returns the version of its `org.springframework.boot` `spring-boot-starter-*`
dependency — `-web` on Boot 3, `-webmvc` on Boot 4 — which is the Boot the release
was built with. A 404 is `""` and no error: a typed release has no pom. Asked once
per release, when the Vaadin answer is known, with the same timeout as the lookup;
the answer is remembered for the run, `""` included.

## 6. The full-screen TUI

### 6.1 The screen

6.1.1 The form runs in the alternate screen. Its own bubbletea model owns a `huh`
form, so that the screen can lay every section out at once, jump between them,
keep derived answers following the coordinates, write the project, and run its
tasks — none of which a form on its own does.

6.1.2 Three phases, one screen. The banner, the boxes and the bottom bar stay
where they are; only what is inside them changes.

| Phase | Middle of the screen | Bar |
| --- | --- | --- |
| asking | the sections | huh's help for the focused field, then the jump keys |
| writing | a `Writing` box naming the directory | `writing the project…` |
| written | the summary, `Next`, and the log | the command bar |

6.1.3 The banner (`ui.Banner`) is passed in by `main` and drawn inside the screen,
trimmed of the blank lines it is printed with. It is not printed in accessible
mode.

### 6.2 Sections

6.2.1 In order, with the fields they ask:

| # | Section | Description | Fields |
| --- | --- | --- | --- |
| 1 | Coordinates | What this project is called to Maven. | Group ID, Artifact ID |
| 2 | Identity | What this project is called to people. | Project name, Description, Base package |
| 3 | Versions | Newest first, from Maven Central. | Vaadin version, Spring Boot version, Java version — three selects (§6.6) |
| 4 | Stack | The core is always generated. Choose its theme, and the rest. | Theme (a select of the themes the chosen Vaadin ships, the line's default first, described *Aura is the Vaadin 25 default.* — §6.6.8), then Features (one multi-select of five options) |
| — | Author | Git has no identity for the first commit. Kept in this repository only; git config --global sets one everywhere. | Name (inline), Email (inline) — a `span` row above Output, only when `Options.AskAuthor` (§6.2.6) |
| 5 | Output | Created if it does not exist. Must be empty. | Directory (inline), Generate |

6.2.2 No version can be typed on the screen. Each version list offers what the
lookup found and the rules allow, and nothing else; a release the lookup did not
offer is what `--vaadin-version`, `--boot-version` and `--java-version` are for,
and a set given that way is checked by `Validate` (§4.5) before anything is
written.

6.2.3 The stack options, in declaration order, each mapping to one `Config` flag:

```
database   Database — PostgreSQL, Flyway, JPA, Testcontainers, dev compose
auth       Auth — OIDC login against Keycloak in the dev stack
e2e        End-to-end tests — Playwright, behind an it profile
coverage   Coverage gate — JaCoCo, 80% on service and presenter packages
traceable  Traceable builds — every build must carry its commit SHA
```

The list is presented with everything already on first, so it opens at the top
whatever the defaults select. Its height is the number of options plus its
title row; it is titled `Features` in both forms and carries no description. The
theme select above it is titled `Theme` in both forms, offers Aura then Lumo, and
opens on the `Config`'s theme.

6.2.4 The Output section declares `span`: it gets a row of its own the width of
the terminal, under the columns. Its directory field is inline (question and
answer on one line) and its description is carried by the section.

6.2.5 Generating is one button — `Affirmative("Generate")`, `Negative("")` — and
the confirm's toggle, accept and reject keys are disabled so the bar offers no key
that does nothing.

6.2.6 The Author section declares `span` like Output and is drawn directly above
it, its two fields inline; it is there only when `Options.AskAuthor` is set. `main` sets it when there is a
conversation to ask in (`--yes` is left with the flags and the summary), a commit
is coming (`--no-git` and `--no-commit` both mean it is not) and
`generate.CurrentAuthor(dir)` reports that git would refuse one. It runs
`git -c user.useConfigOnly=true var GIT_COMMITTER_IDENT` where the commit will
happen: in the output directory when that is already a repository, otherwise in a
throwaway repository created beside it (`.vaadin-init-*` under the nearest
existing ancestor) and removed at once, falling back to a directory that is no
repository when that cannot be made. This is what makes an `includeIf
"gitdir:…"` identity count and an identity local to the repository the user is
standing in — which a repository created inside it does not inherit — not count.
The name and email come from the ident git prints. When git refuses, `git config --get user.name` and `user.email` supply whichever half is
known, and the fields open on it. Both fields are required: a field with nothing
offered rejects the empty answer in accessible mode too, where an empty answer
normally means "keep what you offered". A machine with no git is not asked.

### 6.3 Layout

6.3.1 Each section is a box: a rounded border with the section's name set into the
top edge, one space of padding either side, the description on the first line
inside, then the questions. The active section's border and name are accented; the
rest are drawn in the border colour.

6.3.2 Boxes stand side by side with one column between them. Sections are packed
into columns in reading order, each column taking as near as possible an equal
share of the height; the slack goes to the last box in each column so every column
ends on the same line. Spanning sections are drawn full width underneath.

6.3.3 The number of columns starts as the number of non-spanning sections and is
reduced until each column is at least 26 wide. Group width is the column width
less the box frame (4) and the field's own left bar (2), so nothing inside a box
is re-wrapped by the box.

6.3.4 Whether to tile is decided by rendering and measuring, not by estimating:
the tiled view must be no taller and no wider than the space available. When it is
not, the form falls back to huh's own layout, which asks one section at a time,
and the height budget drops by one row for the blank line huh puts above its help.
Tiling needs roughly 150×36, or 160×34.

6.3.5 The screen is always exactly as tall as the terminal. The bar is held against
the bottom row: a rule the width of the screen, then one line of help and jump
keys. Blank rows go between the content and the rule, never below the bar.

### 6.4 Moving around

6.4.1 `alt+1` … `alt+N` jump to the *n*th section on screen. The keys are listed in
the bar with the current section lit up.

6.4.2 A jump blurs the focused field first. If that raises a validation error in
the section being left, the field is re-focused and the jump is refused.

6.4.3 A jump walks: back to the first section, then forward. Every step blurs
before it moves, so no section is left with a lit-up field behind it. The walk
never steps past the target, so jumping never submits the form.

6.4.4 A jump key past the last section on screen does nothing.

6.4.5 `tab`/`shift+tab` and `enter` behave as huh defines them. `ctrl+c` aborts.

### 6.5 Answers that follow

6.5.1 Project name, package and directory follow the coordinates as they are
typed, since both are on screen at the same time.

6.5.2 A derived answer is replaced only while it still holds what was last derived
for it. Once the user has typed their own, it is never taken back.

6.5.3 Replacing the text means re-binding the field's value: huh reads a bound
value when the field is built and not again. The value is bound through an empty
string first, which is the one case the text input moves its cursor to the end for;
otherwise a name derived letter by letter is reached with the cursor after its
first letter, where `ctrl+u` deletes one letter and typing lands inside the old
answer.

### 6.6 Versions arriving

6.6.1 The version selects are built with whatever the defaults named, and the
lookup's result arrives as a message. The screen never blocks on it.

6.6.2 When it arrives, the Vaadin list is replaced and its bound value set to the
newest release, so the cursor opens there — unless that select has already been
focused, in which case it is left alone.

6.6.3 A list that was fetched carries no description. An empty list keeps the
built-in default and says so: *Maven Central could not be reached — this is the
built-in default, which may be out of date.*

6.6.4 The Boot and theme lists follow the Vaadin answer and the Java list follows
both versions, the way the derived answers follow the coordinates: each is
re-derived when what it follows changes, and left alone otherwise. Nothing on the
screen can be chosen from a list that was not derived for the answers above it,
so a set the rules refuse cannot be assembled there.

6.6.5 The Boot list is The Boot list is the fetched releases
the rules allow for the Vaadin, newest first, at most `offered` of them, plus the
release the Vaadin was built with if it is not among them, labelled `4.1.0 · built
with 25.2.6`. It opens on that release, or on the newest allowed. The pin is asked
for from a command when a Vaadin release is first chosen, and the list re-derived
when it lands; offline the list is the defaults file's Boot alone.

6.6.6 The Java list holds the LTS releases in the range the pair allows and the
newest release in it, LTS ones labelled `21 · LTS`, described by `Describe`
(§4.5). It opens on the defaults file's Java when the range allows it, and on the
newest LTS in range when it does not, the description then adding *The default,
17, is outside that.* A Boot with no rule has no ceiling; the list then runs to
the newest Java any Boot rule names. A pair the rules refuse — possible only with
the offline fallback — leaves the list as it was and puts the refusal in the
description.

6.6.7 A re-derived list keeps a choice already made in it when the new list still
offers it; otherwise the cursor moves to where the list opens. The list is set with its first option bound and the cursor then
walked down to the answer, because huh scrolls a list so that the bound value is
its first row, which hides the newer releases above the pin.

6.6.8 The theme list holds the themes the chosen Vaadin's line ships, in the
rules' order, named for a person (`Aura`, `Lumo`), described *<default> is the
Vaadin <line> default.* It opens on the defaults file's theme when the line ships
it, and on the line's own default otherwise. The select validates its answer
against the Vaadin as answered, for accessible mode, where the list is built
before the Vaadin is typed.

### 6.7 After Generate

6.7.1 Pressing Generate does not end the screen. The screen calls the injected
`Options.Generate` with the completed answers (stack applied),
shows the `Writing` box while it runs, and then the result.

6.7.2 A generation failure ends the screen and is returned to `main`, which prints
it.

6.7.3 The result is two boxes across the top — `✓ <name> is ready` with the same
rows the summary prints, and `Next` with the same commands — drawn to the same
height, and a `Log` box under them taking every row that is left.

6.7.4 A git message from the generator is shown inside the first box in the danger
colour.

### 6.8 The command bar and the log

6.8.1 The bar becomes `❯ run.sh ❯ ` with the placeholder `a task name`, and
`quit to finish` at the end of the line. The input is sized to the bar less that
reminder.

6.8.2 `enter` on an empty bar does nothing. `quit` or `exit` — any case, any
surrounding space — leaves. Anything else is a task and is run. `esc` and `ctrl+c`
also leave.

6.8.3 Running a task echoes `❯ run.sh <task>` into the log, clears the input, and
calls the injected `Options.Task` with a cancellable context and a writer.

6.8.4 Output arrives as whole lines, in order, and is drained in batches of up to
512 so a build of thousands of lines is not a redraw per line.

6.8.5 The log keeps the last 2000 lines. It shows the tail that fits its box, tabs
expanded to four spaces, each line truncated to the width rather than wrapped.
Empty, it reads *output from a task appears here*.

6.8.6 A finished task closes its output with `· done`, `· stopped`, or `· <error>`
in the danger colour, and the bar comes back.

6.8.7 While a task runs the bar reads `running run.sh <task>… · ctrl+c to stop`,
and the log box is drawn as the active one. Keys go nowhere except `ctrl+c`, which
cancels the context; the bar then reads `stopping run.sh <task>…`.

6.8.8 Leaving the screen prints nothing. The terminal is handed back as it was
found.

### 6.9 Accessible mode

6.9.1 `--accessible`, or `ACCESSIBLE` in the environment, replaces the screen with
huh's accessible prompts: no alternate screen, no banner, no boxes.

6.9.2 The conversation is two forms. The first asks the coordinates; the project
name, package and directory are re-derived from the answers; the second asks the
rest. This is what the screen's live derivation replaces. The Author group is
appended to the second form only when it is asked for — left out rather than
hidden, because of 6.9.3.

6.9.3 The version questions are plain inputs pre-filled with the derived answers
(§2.6) — the pin fetched once, between the two forms — not selects with a
follow-up: huh asks a hidden group's questions anyway in accessible mode, which
would ask for each version twice. Their validators apply the rules (§4.5) to the
answers as typed so far, so a Boot typed over the default is checked against the
Vaadin typed before it.

6.9.4 An empty answer means "keep what you offered me", so validation lets the
empty string through and huh substitutes the default. The full-screen form
validates the empty string, because a cleared field there was cleared on purpose.

6.9.5 Input is read through a reader that hands over one line per `Read`, because
huh builds a fresh scanner per field and a scanner reads ahead: without it,
answers piped from a file would all be swallowed by the first question.

6.9.6 In this mode `main` generates the project, prints the summary, and then asks
for one task with the same `❯ run.sh ❯` prompt; `quit`, `exit` and an empty answer
all mean none.

## 7. Generating

7.1 The whole tree is rendered into memory before anything is written, so a broken
template leaves nothing on disk.

7.2 The manifest is the generated project, declared once. `dst` is itself a
template, which is what places Java sources under the chosen package.

| Template | Destination | When | Mode |
| --- | --- | --- | --- |
| `pom.xml.tmpl` | `pom.xml` | always | 644 |
| `run.sh` | `run.sh` | always, verbatim | 755 |
| `run.conf.tmpl` | `run.conf` | always | 644 |
| `run.tasks.sh.tmpl` | `run.tasks.sh` | always | 644 |
| `commit-msg` | `.githooks/commit-msg` | always, verbatim | 755 |
| `gitignore.tmpl` | `.gitignore` | always | 644 |
| `README.md.tmpl` | `README.md` | always | 644 |
| `application.properties.tmpl` | `src/main/resources/application.properties` | always | 644 |
| `styles.css` | `src/main/resources/META-INF/resources/styles.css` | always, verbatim | 644 |
| `styles/main-view.css` | `src/main/resources/META-INF/resources/styles/main-view.css` | always, verbatim | 644 |
| `java/Application.java.tmpl` | `src/main/java/{{.PackagePath}}/Application.java` | always | 644 |
| `java/MainView.java.tmpl` | `src/main/java/{{.PackagePath}}/ui/view/MainView.java` | always | 644 |
| `java/MainViewTest.java.tmpl` | `src/test/java/{{.PackagePath}}/ui/view/MainViewTest.java` | always | 644 |
| `compose.yaml.tmpl` | `environment/dev/compose.yaml` | `ContainerRequired` | 644 |
| `sql/V1__init_schema.sql.tmpl` | `src/main/resources/db/migration/V1__init_schema.sql` | database | 644 |
| `java/Note.java.tmpl` | `src/main/java/{{.PackagePath}}/notes/Note.java` | database | 644 |
| `java/NoteRepository.java.tmpl` | `src/main/java/{{.PackagePath}}/notes/NoteRepository.java` | database | 644 |
| `java/NoteService.java.tmpl` | `src/main/java/{{.PackagePath}}/notes/service/NoteService.java` | database | 644 |
| `java/TestcontainersConfiguration.java.tmpl` | `src/test/java/{{.PackagePath}}/TestcontainersConfiguration.java` | database | 644 |
| `java/SecurityConfig.java.tmpl` | `src/main/java/{{.PackagePath}}/config/SecurityConfig.java` | auth | 644 |
| `java/TestSecurityConfiguration.java.tmpl` | `src/test/java/{{.PackagePath}}/TestSecurityConfiguration.java` | auth | 644 |
| `keycloak-realm.json.tmpl` | `environment/dev/keycloak/realm.json` | auth | 644 |
| `java/MainViewIT.java.tmpl` | `src/test/java/{{.PackagePath}}/ui/view/MainViewIT.java` | `BrowserTests` | 644 |
| `java/ProtectedRootIT.java.tmpl` | `src/test/java/{{.PackagePath}}/ProtectedRootIT.java` | `ProtectedRootTest` | 644 |

7.3 Verbatim files are copied byte for byte. They name no project, which is what
lets every generated project share them.

7.4 Templates are `text/template` with `missingkey=error`; an unknown field is a
parse or execute error naming the template.

7.5 Rendered paths are relative and forward-slashed, sorted, and become platform
paths only when written.

7.6 A target directory that exists and is not empty stops the run:
`<root> already exists and is not empty; pass --force to write into it anyway`.

7.7 Every file is written with its mode and then `chmod`ed to it, so `--force`
onto a tree where `run.sh` lost its executable bit sets it again.

7.8 Unless `--no-git`, the generator then:

1. `git init --quiet`, unless `.git` already exists;
2. `git config core.hooksPath .githooks`, so the hook the project ships is live;
3. `git config user.name` and `git config user.email` for whichever of
   `AuthorName` and `AuthorEmail` is set — the repository's own config, never
   `--global`;
4. unless `--no-commit`, `git add -A` and `git commit --message "chore: initial commit"`.

7.9 The commit is only ever the first one. A repository that already has a `HEAD`
is left alone and says so: *this repository already has commits, so none was
made*.

7.10 Failure here is reported, never fatal — the project is written and usable:

- git not on the `PATH` → *git is not on the PATH: run `git init` yourself, then
  `git config core.hooksPath .githooks`*
- a failed commit → *nothing was committed: `<git's first line>`. Commit yourself
  with: git add -A && git commit -m 'chore: initial commit'*

7.11 `--dry-run` renders, prints `<n> files would be written` and the file tree,
and writes nothing.

## 8. What is printed

8.1 `ui` owns every colour and glyph. One palette, one huh theme, one set of
renderers, so the banner, the form and the summary cannot look like three tools.

8.2 Every colour is a `lipgloss.CompleteAdaptiveColor`: a light and a dark value,
each stating its TrueColor, ANSI256 and ANSI form rather than letting a
nearest-match choose. Roles: `Accent` (identity, the active field, commands to
type), `Success` (what happened), `Danger` (only ever a failure), `Muted`
(supporting prose), `Faint` (legible only when looked for), `Border` (structure).

8.3 Glyphs: `❯ ` prompt, `✓ ` selected, `· ` unselected, ` ✗` error indicator,
` · ` between things on one line.

8.4 The theme is built on `huh.ThemeBase`, which sets the structural styles and
leaves the colours unset. The active field wears a thick accent left bar, the same
mark as the banner. Blurred styles are copied from focused first, so a style added
to one is inherited rather than silently missing from the other. A blurred select
keeps a faint caret on its chosen option — the only mark of the answer once the
cursor has moved on — while a blurred multi-select drops its cursor, its options
carrying their own ✓ and ·.

8.5 Renderers: `Banner`, `Summary` (a rounded box, labels aligned, values wrapped
to 72 columns with a hanging indent), `NextSteps`, `FileTree` (a tree, chains of
single-child directories collapsed onto one line), `SectionBox`, `Fields`,
`Commands`, `Shortcuts`, `Bar`, `Rule`, `Heading`, `Warning`, `Working`, `Echoed`,
`Finished`, `Failed`, `CommandInput`, `Cancelled`, `Error`, `Join`.

8.6 The summary rows are `where` (path relative to the working directory when it
is under it, absolute otherwise, and the file count), `stack` (the three
versions, then the theme's name; and under them, when the Boot is the release the
Vaadin was built with, *Spring Boot 4.1.0 is the release Vaadin 25.2.6 was built
with* — on the scripted path only when the Boot was derived, since a typed pair
has had no pin fetched and a summary is not worth a request), `options` (`Selected()`, or `none — core only`), `ports` (`app <n>`,
then `postgres <n>` with the database and `keycloak <n>` with auth), `git` (what
was done —
`initialised`, `commit-msg hook wired up`, `author kept in this repository`, `first
commit made` — or `not initialised`).

8.7 The next steps are `cd <dir>`, then `./run.sh env` when a container runtime is
required, `./run.sh run` (*start the application on http://localhost:<app port>*),
then `./run.sh verify` when the project has integration
tests or `./run.sh test` when it does not, then `./run.sh help`.

8.8 The full-screen path prints none of this on exit; it showed it on the screen.
`--yes` and `--accessible` print it, because they have no screen to show it on.

## 9. Running a task

9.1 A task named in the command bar runs `./run.sh <fields of the input>` with the
project root as its working directory, stdout and stderr going to the log.

9.2 The task gets a process group of its own, so the script, Maven and anything
they start can be stopped by one signal.

9.3 Cancelling interrupts that whole group. The command is given 2 seconds
(`WaitDelay`) before Go kills the process and closes the pipes, and whatever is
still standing when the run returns is killed outright — a script's background
children ignore an interrupt when there is no job control, and one that keeps hold
of the output pipe holds the terminal.

9.4 A task the user stopped is not a task that failed.

9.5 Process-group handling is per platform: `process_unix.go` sets `Setpgid` and
signals `-pid`; `process_windows.go` compiles to a no-op group and a plain kill.

9.6 The task asked for after the summary in the non-screen path is not run on
Windows.

## 10. Build and distribution

10.1 `make build` builds `vaadin-init` with
`-ldflags '-s -w -X main.version=$(VERSION)'`, where `VERSION` is
`git describe --tags --always --dirty`, or `dev`.

10.2 `make check` is `gofmt -l` (checked, never applied), `go vet ./...` and
`go test ./...`.

10.3 `make dist` runs `check`, then cross-compiles with `CGO_ENABLED=0
-trimpath` into `dist/vaadin-init-<os>-<arch>[.exe]` for linux/amd64,
linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 and windows/arm64.

10.4 `make install` and `make clean` exist and do what their names say.

10.5 `.gitattributes` sets `* text=auto eol=lf`, because `templates/run.sh` and
the commit-msg hook are copied verbatim into every generated project and a CRLF
checkout would generate projects whose task runner dies on its first line.

10.6 The repository's own `.githooks/commit-msg` is the same hook the generated
projects get.

## 11. Continuous integration

`.github/workflows/build.yml`, on push to `main`, on pull request, and on demand;
in-flight runs of the same ref are cancelled; every action pinned to a commit SHA.

11.1 `tool` — on ubuntu, macos and windows: gofmt, `go vet`, `go test`, then a
smoke test that builds the binary and runs `--version` and
`--dry-run --yes --group-id com.example --artifact-id smoke-app`.

11.2 `cross-compile` — `make dist`, uploaded as artifacts.

11.3 `generated project` — twice, once with everything off on the Aura default
and once with everything on and `--theme lumo`, so both themes are compiled by a
real JDK: generate with `--yes`, assert the project committed itself and has
a clean working tree, then install Temurin 21 with the Maven cache keyed on the
generated pom, and build and test it through `./run.sh test`. Surefire reports are
kept on failure.

11.4 `.github/workflows/versions.yml`, weekly, on demand, and on a pull request
touching `compat.json` or the check, runs `go run ./internal/checkcompat
compat.json`. It parses the file (the schema check), reads every Vaadin release
and the Boot releases of the supported lines from Maven Central, and the starter
pom of each supported line's newest release, and fails naming each of: a newest
release built with a Boot the rules refuse for it; a Vaadin major, from the oldest
ruled line up, with no rule; a Boot line a supported Vaadin line sits on that is
past `eol`; a Boot minor with no rule. Every message quotes the rule's source.
The decision is a pure function over the fetched lists, tested offline.

## 12. The generated project

12.1 `pom.xml`: `jar` packaging, version `1.0.0-SNAPSHOT`, parent
`spring-boot-starter-parent` at the chosen Boot version, `vaadin-bom` imported at
the chosen Vaadin version, `java.version` as chosen, and an empty `argLine`
property so `@{argLine}` resolves when JaCoCo does not run.

12.2 Dependencies follow the options: `vaadin-core`,
`vaadin-spring-boot-starter`, `vaadin-dev` and `spring-boot-devtools` always,
security and `oauth2-client` with auth, `data-jpa` + `postgresql` + Flyway
(`flyway-core`, `flyway-database-postgresql`, `spring-boot-flyway`) with the
database, `spring-boot-starter-test` always, the browserless test libraries,
Playwright `1.59.0` with browser tests, `spring-security-test` with auth, and
`testcontainers-postgresql` + `spring-boot-testcontainers` with the database.

12.3 Plugins always present: compiler, `maven-dependency-plugin` (resolving agent
paths), `spring-boot-maven-plugin`, `vaadin-maven-plugin` (`build-frontend`), and
surefire configured with `@{argLine}` and Mockito as a `-javaagent`. Then:

- with traceable builds, `maven-enforcer-plugin` execution `require-build-commit`,
  which fails a build that carries no commit SHA;
- with coverage, `jacoco-maven-plugin` `0.8.13`: `prepare-agent`, a `report` and a
  `check` bound to `test`, with a `PACKAGE` rule over `<package>.*.service` and
  `<package>.*.ui.presenter` requiring 0.80 covered instructions;
- with end-to-end tests, `maven-failsafe-plugin` bound to `integration-test` and
  `verify`, and `maven-surefire-report-plugin` `3.5.5` rendering the results into
  `target/reports/` at `post-integration-test`.

12.4 Profiles, when the project has end-to-end tests: `it` sets
`vaadin.productionMode=true` (and `headless` where there are browser tests),
because integration tests against a dev-mode page find an empty screen; and, with
browser tests, `debug-ui` sets `headless=false`, for watching the browser drive
itself.

12.5 `run.sh` is the task runner, shared and verbatim, driven by `run.conf`. Its
`setup` resolves a JDK — by prefix under `~/.sdkman/candidates/java`, falling back
to `JAVA_HOME` — then Maven, and then, only when `CONTAINER_REQUIRED` is true, a
container runtime: docker if it is there, otherwise podman, and an error if
neither is. Tasks: `env [up|down|logs|reset]`, `deps`, `compile`, `bundle`,
`styles`, `test`, `verify`, `run`, `preview`, `package`, `clean`, `help`. `env` is
listed, and will run, only when the project needs containers. The task list
describes the runner and never what a project's build is configured to do.

12.6 `run.conf` carries `PROJECT_NAME`, `JAVA_VERSION`, `ENV_DIR`,
`CONTAINER_REQUIRED`, and — commented out unless the option is on — `IT_PROFILE`
and `COMMIT_PROPERTY`. `CONTAINER_PREFIX`, `BUNDLE_DIR` and `STYLES_CSS` are shown
commented, as the defaults `run.sh` applies.

12.7 `run.tasks.sh` is sourced after the runner's helpers; a `task_<name>`
function there becomes `./run.sh <name>`, and `project_usage` is printed under the
built-in list. It ships as a worked example, commented out.

12.8 `application.properties` sets the application name, `server.port` to the
app port, opens a browser in dev mode, shows `vaadin.allowed-packages` commented
out, and sets `logging.level.<package>=DEBUG`. With the database it points at the
dev PostgreSQL on the database port, sets `ddl-auto=validate` and
`open-in-view=false`. With auth it configures the Keycloak registration
(`client-id` = artifact id, secret `dev-secret`, scopes `openid,profile,email`) and
an issuer the browser can reach, on the auth port.

12.9 `environment/dev/compose.yaml` is named after the container prefix and
declares a healthcheck per service: `postgres:18-alpine` with the derived database
name, user and password `dev`, published on the database port with a named
volume mounted at `/var/lib/postgresql` (the 18+ image's data root), and `quay.io/keycloak/keycloak:26.0` in `start-dev --import-realm`
published on the auth port (its management port 9000 is reached by the healthcheck
inside the container and not published), importing
`environment/dev/keycloak/realm.json` — a realm named after the artifact id,
holding a confidential client of the same name with the secret `dev-secret`,
redirect URIs and web origin on the app port, a `user` realm role, and one user
`dev` / `dev`. `TestSecurityConfiguration` names the same issuer as the
properties, on the auth port.

12.10 Java sources: `Application` (a `@SpringBootApplication` and
`AppShellConfigurator` loading `styles.css` after the theme: `Aura.STYLESHEET`
under Aura, or `Lumo.STYLESHEET` and `Lumo.UTILITY_STYLESHEET` under Lumo —
nothing else loads the Lumo Utility Classes, and a layout built from
`LumoUtility` constants without them renders unstyled with no error), `MainView` (`@Route("")`, titled with the project
name, a text field and a button, a `Grid` of notes when there is a database,
`@PermitAll` under auth), `MainViewTest` (a browserless `@SpringBootTest` that
adds a message and refuses an empty one). With the database: `Note` (`@Entity`,
`note` table, `body` and `created_at`), `NoteRepository`, `NoteService`
(`@Transactional`, `findAllNewestFirst`, `add`) and `TestcontainersConfiguration`.
With auth: `SecurityConfig`, and
`TestSecurityConfiguration` — a `@TestConfiguration` declaring the OIDC client
registration in memory, which is what stops Spring Security resolving the issuer
over the network while a test context starts, and so what lets the tests run with
no Keycloak up. The test classes import it. With browser tests: `MainViewIT` driving Playwright
against a random port. With auth and e2e: `ProtectedRootIT`, which asserts that an
anonymous request to `/` answers 302 to `/oauth2/authorization/keycloak`, and
needs neither a browser nor a running Keycloak.

12.11 `README.md` documents the project it was generated for: how to run it, what
is in the tree, the opinions behind it, and the git conventions the hook enforces.
`.gitignore` covers `target/`, the frontend build's generated files and bundles,
`environment/dev/data/` when there is a database, and editor droppings.

12.12 The commit-msg hook enforces a Conventional Commits subject
(`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`, optional scope,
optional `!`), a subject of at most 100 characters, a single line with no body,
and no `Co-Authored-By` trailer. Merge, revert, fixup and squash messages pass
through.

## 13. Verification

13.1 `internal/generate` renders all 32 combinations of the five options, under
each of the two themes, and
checks each: the pom is well-formed XML, the realm is valid JSON, no file carries
an unresolved template value, every Java file declares the package its path
implies, each optional file appears exactly when its option is on, the pom names a
dependency only when it is used, `run.sh` and the hook are executable, and the
shared files are byte-identical to their templates, `Application` names the
chosen theme's stylesheets and nothing of the other theme (both Lumo stylesheets
under Lumo), no generated stylesheet uses a `--lumo-*` or `--aura-*` property,
and the three ports appear
wherever the project names a host port with no fixed 8080, 8081 or 5432 left
beside them. It also covers the git
behaviour: the project commits itself, that commit satisfies the hook the project
ships, existing history is untouched, and both `--no-commit` and `--no-git` are
honoured. With every place git looks for an identity pointed at nothing, it checks
that the commit is reported rather than made, that an author given goes into the
repository's config and no further and is who the commit is by, and that
`CurrentAuthor(dir)` reports what git has — nothing, one half, both — including
an identity from the output directory's own repository or from a conditional
include keyed on its parent, and not the identity of the repository the test is
standing in; and that asking leaves nothing beside the project.

13.2 `internal/prompt` drives the conversation two ways: through huh's accessible
mode, and by stepping the full-screen model at a range of terminal sizes. The
screen's assertions are that every section, every version and every stack option
is on screen at once; that the screen fits the terminal at every size or falls
back to one section at a time; that the bar is on the bottom row whatever is above
it; that only one question is ever active after a jump; that a derived answer
follows the coordinates but a typed one is never taken back; that the Vaadin
list opens on the newest release and the Boot list on the release it was built
with, labelled, with the newer releases still in view; that the Boot list follows
the Vaadin answer and keeps a choice while it is still allowed; that the Java list
holds the LTS releases and the newest in range, opens on the default or snaps to
the newest LTS and says so; that the theme list follows the Vaadin, offering only
what the line ships and naming its default; that no version list offers typing
and no section is hidden; that the author section is a row above the output only when it is asked for, and that
its questions are asked in accessible mode, refuse an empty answer with nothing
offered, and keep an offered half; that the output section is the only one with a row of its own and is drawn
to the whole width; that generating is one button and the bar advertises no key
that does nothing; that the project is written without leaving the screen; that a
task runs into the log and comes back, a failure included; that ctrl+c stops the
task and not the screen; and that the log takes the room that is left.

In accessible mode: that the derived Boot is the pin and the derived Java the
default, that a Vaadin, Boot or Java refused by the rules is asked again at its
field, and that an out-of-range Java default snaps.

An empty `compat.Rules` refuses every theme, and huh's accessible select re-asks a
refused answer until one is accepted — with the input exhausted, forever. Every
test conversation therefore starts from `conversation()`, which carries the
shipped rules.

13.4 `internal/compat` table-tests the rules over the shipped file: an
unsupported line is refused naming its Boot line and source, `since` tightens the
Boot minimum per release, `CompatibleBoot` keeps the line and the minimum,
`JavaRange` refuses the pair before Java and has no ceiling for an unruled Boot
minor, `java_max` is inclusive, `BootDefault` prefers a pin the rules allow, a
theme the line does not ship is refused and `ThemeDefault` falls back to the
line's own, an
override wins per line and leaves the other lines alone, and a malformed file is
refused naming the entry. `internal/config` checks `Validate` accepts 25.2.6 with
4.1.0 on 21 and 25 and refuses 4.0.8, Java 17 and 27, and 24.10.9, each with the
field and the rule named. `internal/versions` reads the pinned Boot from a `-web`
and a `-webmvc` starter pom served locally, and a 404 as `""` with no error.
`main` asserts a `--yes` run with a refused set fails naming the field and writes
nothing, and a compatible fully pinned set is accepted with no network.
`internal/checkcompat` tests its findings over the releases Maven Central listed
the day the file was written — quiet — and with a pin moved ahead of the rules, a
Vaadin major with no rule, a Boot line past its end of support, and a Boot minor
with no rule.

13.5 `TestPreview` renders the screen at several terminal sizes without a
terminal, for reviewing how it looks:

```sh
PREVIEW=1 go test ./internal/prompt/ -run TestPreview -v
SIZE=150x36 PREVIEW=1 ...
PROFILE=ansi PREVIEW=1 ...
LIGHT=1 PREVIEW=1 ...
```
