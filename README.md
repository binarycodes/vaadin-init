# vaadin-init

Bootstraps an opinionated Vaadin and Spring Boot project. One binary, no
toolchain to install first, and a full-screen TUI that asks about eight things on
one page and then writes a project that builds.

```
$ vaadin-init

┃ vaadin-init v0.2.0
┃ An opinionated Vaadin 25 and Spring Boot 4 project.

╭─ Coordinates ────────────────────╮ ╭─ Identity ───────────────────────╮ ╭─ Versions ───────────────────────╮ ╭─ Stack ──────────────────────────╮
│ What this project is called to   │ │ What this project is called to   │ │ Newest first, from Maven         │ │ The core is always generated.    │
│ Maven.                           │ │ people.                          │ │ Central.                         │ │ These are the rest.              │
│ ┃ Group ID                       │ │   Project name                   │ │   Vaadin version                 │ │     ✓ Database — PostgreSQL,     │
│ ┃ Maven group, in reverse-DNS    │ │   The name that appears in the   │ │     25.2.6                       │ │   Flyway, JPA, Testcontainers,   │
│ ┃ form.                          │ │   UI and in the task runner's    │ │     25.2.5                       │ │   dev compose                    │
│ ┃ ❯ io.binarycodes               │ │   output.                        │ │     25.2.4                       │ │     ✓ End-to-end tests —         │
│                                  │ │   ❯ Book Shelf                   │ │     25.2.3                       │ │   Playwright, behind an it       │
│   Artifact ID                    │ │                                  │ │     25.2.2                       │ │   profile                        │
│   Maven artifact. Also names     │ │   Description                    │ │     type one myself…             │ │     ✓ Coverage gate — JaCoCo,    │
│   the directory and the          │ │   ❯ A small library              │ │                                  │ │   80% on service and presenter   │
│   containers.                    │ │                                  │ │   Spring Boot version            │ │   packages                       │
│   ❯ book-shelf                   │ │   Base package                   │ │     4.1.1                        │ │     ✓ Traceable builds — every   │
│                                  │ │   Where the generated Java       │ │     4.1.0                        │ │   build must carry its commit    │
│                                  │ │   sources live.                  │ │     4.0.8                        │ │   SHA                            │
│                                  │ │   ❯ io.binarycodes.bookshelf     │ │     4.0.7                        │ │     · Auth — OIDC login against  │
│                                  │ │                                  │ │     4.0.6                        │ │   Keycloak in the dev stack      │
│                                  │ │                                  │ │     type one myself…             │ │                                  │
│                                  │ │                                  │ │                                  │ │                                  │
│                                  │ │                                  │ │   Java version                   │ │                                  │
│                                  │ │                                  │ │   Spring Boot 4 needs 17 or      │ │                                  │
│                                  │ │                                  │ │   newer.                         │ │                                  │
│                                  │ │                                  │ │   ❯ 21                           │ │                                  │
╰──────────────────────────────────╯ ╰──────────────────────────────────╯ ╰──────────────────────────────────╯ ╰──────────────────────────────────╯
╭─ Output ───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ Created if it does not exist. Must be empty.                                                                                                       │
│   Directory ❯ book-shelf                                                                                                                           │
│                                                                                                                                                    │
│     Generate                                                                                                                                       │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯

──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
enter next · alt+1 coordinates · alt+2 identity · alt+3 versions · alt+4 stack · alt+5 output
```

One screen, not a queue of questions: the whole of what is about to be generated
is in front of you before you agree to it. `alt+1` … `alt+5` jump straight to a
section rather than walking back through the ones in between, and the bar along
the bottom stays put — what the keys under your fingers do, then where each jump
key goes, with the section you are in lit up.

The answers derived from the coordinates — the project name, the package, the
directory — follow them as they are typed, since both are on the screen at the
same time and one of them being stale would be visible.

A terminal too small for the boxes — roughly under 150 by 36, or 160 by 34 —
asks one section at a time instead, with the same bar and the same jump keys, and
`--accessible` replaces the screen entirely with plain sequential prompts for a
screen reader.

Agreeing to generate does not end the screen. The project is written from inside
it, and the same boxes report what happened — with a log under them that takes
whatever room is left:

```
┃ vaadin-init v0.2.0
┃ An opinionated Vaadin 25 and Spring Boot 4 project.

╭─ ✓ Book Shelf is ready ────────────────────────────────────────────────╮ ╭─ Next ─────────────────────────────────────────────────────────────────╮
│ where    book-shelf  (22 files)                                        │ │ cd book-shelf                                                          │
│ stack    Vaadin 25.2.6 · Spring Boot 4.1.1 · Java 21                   │ │ ./run.sh env     bring up the development stack                        │
│ options  database · auth · e2e · coverage · traceable                  │ │ ./run.sh run     start the application                                 │
│ git      initialised, commit-msg hook wired up                         │ │ ./run.sh verify  unit tests and integration tests                      │
│                                                                        │ │ ./run.sh help    every task                                            │
╰────────────────────────────────────────────────────────────────────────╯ ╰────────────────────────────────────────────────────────────────────────╯
╭─ Log ──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ❯ run.sh test                                                                                                                                      │
│ [INFO] Scanning for projects...                                                                                                                    │
│ [INFO] Building book-shelf 0.0.1-SNAPSHOT                                                                                                          │
│ [INFO] Tests run: 4, Failures: 0, Errors: 0, Skipped: 0                                                                                            │
│ [INFO] BUILD SUCCESS                                                                                                                               │
│ · done                                                                                                                                             │
│                                                                                                                                                    │
│                                                                                                                                                    │
╰────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
❯ run.sh ❯ a task name                                                                                                                · quit to finish
```

The bar has become a command bar, and the log is what it writes into. Name one of
the tasks beside it and it runs in the new project, right there: its output
arrives in the log as it is produced, `ctrl+c` stops it — the task, not the tool —
and the bar comes back for the next one. Starting the application is not the last
thing this tool does; it is one of the things it does.

Type `quit` — or `exit` — to finish. Not a bare enter: that is the key every
answer on this screen was given with, and a way out that close to hand is a way
out taken by accident.

What it leaves behind is nothing. The screen goes when the program does and the
terminal comes back exactly as it was found — no summary printed under the command
that started it, no scrollback to clear. The modes with no screen to say it on —
`--yes` and `--accessible` — print the summary instead, and offer the same one
task before they go:

```
╭────────────────────────────────────────────────────────────╮
│  ✓ Book Shelf is ready                                     │
│                                                            │
│  where    book-shelf  (22 files)                           │
│  stack    Vaadin 25.2.6 · Spring Boot 4.1.1 · Java 21      │
│  options  database · auth · e2e · coverage · traceable      │
│  git      initialised, commit-msg hook wired up            │
╰────────────────────────────────────────────────────────────╯

  Next
    cd book-shelf
    ./run.sh env     bring up the development stack
    ./run.sh run     start the application
    ./run.sh verify  unit tests and integration tests
    ./run.sh help    every task
```

Every question is also a flag, so the same project can be generated from a
script:

```sh
vaadin-init --yes \
    --group-id io.binarycodes \
    --artifact-id book-shelf \
    --database --e2e --coverage --traceable
```

`--dry-run` lists what would be written and writes nothing. `--help` lists every
flag, with this machine's defaults filled in.

The generated project is a git repository with the commit-message hook wired up
and the first commit made, so it builds immediately. That last part is not a
courtesy: with `--traceable` on, the generated build requires the commit SHA it
was built from and refuses to run until a commit exists. `--no-commit` sets the
repository up and leaves the commit to you; `--no-git` touches git not at all.

On a machine where git has no name and email configured, the commit would fail,
so the tool asks for them first — an Author section on the screen, or
`--author-name` and `--author-email` from a script — and keeps them in the new
repository's own config. Your global git configuration is never written; the
section says how to set one everywhere yourself.

## Installing

Download the binary for your platform from the releases, or build it:

```sh
make build           # ./vaadin-init
make install         # into $GOBIN
make dist            # every platform, into dist/
```

`make dist` cross-compiles for linux, macOS and Windows on amd64 and arm64. The
binaries are static: the templates and the defaults are embedded, so a binary
carries the whole tool.

## What it generates

Every project gets the core — Vaadin, Spring Boot, the task runner, the
commit-message hook, one view and one browserless test for it. The rest is
chosen:

| Option | What it adds |
| --- | --- |
| `--database` | PostgreSQL, Flyway, JPA, Testcontainers, a dev compose file, and a worked vertical slice: an entity, a repository, a service and a grid |
| `--auth` | OIDC login against a Keycloak in the dev stack, with a realm imported and a `dev` / `dev` user |
| `--e2e` | Integration tests behind an `it` profile that puts the app in production mode — Playwright driving a browser, or, for a project with auth, an assertion that the root is protected |
| `--coverage` | JaCoCo with an 80% gate on the service and presenter packages |
| `--traceable` | Every build must carry the commit SHA it was built from |

The generated project's own README explains each opinion and how to back out of
it. That file is the answer to "why is it like this?", and it ships in the project
rather than living here, because that is where the question gets asked.

## Defaults

The framework versions are looked up from Maven Central when the tool starts, so
the default is the current release rather than whatever was current when the
binary was built. The lookup runs while the first questions are being answered and
gives up after five seconds; `defaults.toml` is the fallback, not the source.

To change what the prompts start on, drop a file shaped like `defaults.toml` at:

| | |
| --- | --- |
| Linux | `~/.config/vaadin-init/defaults.toml` |
| macOS | `~/Library/Application Support/vaadin-init/defaults.toml` |
| Windows | `%AppData%\vaadin-init\defaults.toml` |

Keys left out keep their built-in value, so a personal file only names what it
changes. `--defaults <path>` overrides both.

## Scope

Vaadin 25 on Spring Boot 4, and nothing else. The two generations differ in ways
one pom template cannot straddle honestly — Boot 4 splits auto-configuration into
a module per technology and renames several starters — so Vaadin 24 would mean a
second set of templates rather than another conditional.

## Layout

```
main.go                     flags, embedding, and the non-interactive path
internal/config/            the answers, and what counts as a valid answer
internal/versions/          the Maven Central lookup, and its fallback
internal/prompt/            the TUI
internal/generate/          the manifest, and rendering it to disk
templates/                  what gets generated
defaults.toml               what the prompts start on
```

`internal/generate/generate.go` holds the manifest: one line per generated file,
saying where it goes and which option asks for it. A new template is a template
plus a line there.

`templates/run.sh` and `templates/commit-msg` are copied verbatim, never rendered.
They name no project, which is what lets every generated project share them — and
`run.sh` contains `{{.Endpoints.docker.Host}}` for `docker context inspect`, so
rendering it would not survive first contact anyway.

## Continuous integration

`.github/workflows/build.yml` verifies three things on every push and pull request:

| Job | |
| --- | --- |
| `tool` | gofmt, vet, tests and a binary smoke test on Linux, macOS and Windows — the tool ships a binary for each, so each has to run one |
| `cross-compile` | `make dist` for all six targets, uploaded as artifacts |
| `generated-project` | generates a project and builds it with a real JDK — once with nothing optional, once with everything |

That last job is the one that matters most, and the only place a generated project
is ever compiled: the tests in this repository render the templates and check their
shape, but well-formed XML is not the same as Maven accepting it, and a wrong class
name in a Java template is invisible until `javac` sees it. It builds through
`./run.sh`, so the task runner every generated project ships is verified too.

It runs `./run.sh test` rather than `verify` — that compiles every source and test,
integration tests included, and runs the unit tests, without the production
frontend build and browser download that `verify` adds.

## Development

```sh
make check     # gofmt, go vet, go test
```

`internal/generate`'s tests render all 32 combinations of the five options and
check each one: the pom is well-formed XML, the realm is valid JSON, no file
carries an unresolved template value, every Java file declares the package its
path implies, and each optional file appears exactly when its option is on.

`internal/prompt`'s tests drive the conversation two ways: through huh's accessible
mode, which is also what `--accessible` gives a screen reader, and by stepping the
full-screen form as a `tea.Model` at a range of terminal sizes. The second set
exists because this class of fault is silent and specific:

- everything has to be on the screen at once — a section, a version or a stack
  option that is not visible is one nobody can check before pressing generate,
  which is the same as never having asked;
- the screen has to fit the terminal it is drawn in, at every size, or the layout
  it falls back to has to be the one asking a section at a time;
- the project has to be written without leaving the screen — same banner, same
  boxes, same bar — since handing the terminal back mid-flow reads as the program
  having ended and something else having started;
- a task named in the bar has to run into the log and come back, however it ends:
  a failure is a line in the log, and ctrl+c stops the task rather than the tool,
  which is the difference between a screen that runs commands and one that runs
  a command;
- the bar has to be on the bottom row whatever is above it, and only one question
  is ever the one being answered — a jump walks past sections, and arriving in one
  focuses the question waiting there, so a walk that does not blur as it goes
  lights up a field in every section it passed;
- `huh` decides which line a list opens on inside `Options(...)`, by scanning for
  the bound value — so `Value(...)` must be chained *before* `Options(...)` or the
  list opens wherever that scan stopped;
- `huh` opens a multi-select on the first *selected* option, so the stack list puts
  everything already on at the top; otherwise a defaults file that turns the first
  entries off would open the list scrolled past them;
- a derived answer follows the coordinates only while it still holds what was
  derived for it, so a name someone typed is never taken back.

To look at the screen without running it — it is a `tea.Model`, so a frame of it
can be rendered anywhere:

```sh
PREVIEW=1 go test ./internal/prompt/ -run TestPreview -v   # a frame at each size, plus the summary
SIZE=132x30 PREVIEW=1 ...                                  # one terminal size
PROFILE=ansi PREVIEW=1 ...                                 # the sixteen-colour fallback
LIGHT=1 PREVIEW=1 ...                                      # on a light background
```

`internal/ui` holds the palette, the huh theme and the renderers for everything
printed around the form. Colours are `CompleteAdaptiveColor`: adaptive for light
and dark backgrounds, and with the sixteen-colour value stated rather than
nearest-matched — left to a nearest match, every grey in this palette lands on
bright blue and the form loses all hierarchy.
