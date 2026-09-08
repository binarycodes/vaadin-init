# Keep one set of templates per Vaadin line, chosen by the version being generated

## Where it stands

The tool has one template tree and generates one Vaadin line from it.

- `templates/` is flat: `pom.xml.tmpl`, `run.sh`, `application.properties.tmpl`
  and the rest at the top, the Java sources under `templates/java/`, and nothing in
  any path that says which Vaadin they are for. `main.go:44` embeds the whole
  directory and `main.go:185` hands `fs.Sub(templateFS, "templates")` to
  `generate.New`.
- The manifest (`internal/generate/generate.go:64-96`) names each template by a
  path relative to that root, and `Render` (`generate.go:117-150`) reads it with
  one `fs.ReadFile(g.templates, f.src)` at line 124. The `Config` it renders with
  carries `VaadinVersion` (`internal/config/config.go:37`), and nothing in the
  package looks at it.
- Which lines the tool generates is a flag in data: `compat.json` marks line 25
  `supported: true` and 24 `supported: false`, its `$comment` says *a Vaadin line
  is `supported` when this tool has templates for it*, and `Rules.Vaadin`
  (`internal/compat/compat.go:264-283`) refuses any version outside a supported
  line. `Rules.Supported()` (`compat.go:250-262`) is what the lookup fetches
  (`main.go:420`), what the banner says, and what `internal/checkcompat` checks
  against Maven Central.
- The README's Scope section (`README.md:192-199`) is honest about why: Boot 4
  splits auto-configuration into a module per technology and renames starters,
  so *Vaadin 24 would mean a second set of templates rather than another
  conditional*. The tree cannot hold a second set. Flipping 24 to `supported:
  true` today would render a Vaadin 24 project from a Boot 4 pom.

So the flag promises something the layout cannot deliver, and the only test of
the promise is a sentence in a JSON comment. The same is true of the next major:
the day Vaadin 26 changes anything a starter project contains, the choice is an
`if` in every affected template or a copy of the tree under another name with a
hand-written switch.

Two things the tool already has make this cheap to do properly.
`internal/version` compares versions (`version.go:51`, `:90`) and knows what a
line is (`InLine`, `:66`); `compat.json` already names lines the same way (`25`,
`4.1`). And the manifest is one Go list with `src` paths that never say where the
root is, so a root that depends on the version costs the manifest nothing.

## What to do

### 1. A directory per version floor, at the top of `templates/`

```
templates/
  24/                 everything a Vaadin 24 project needs
    pom.xml.tmpl
    run.sh
    java/Application.java.tmpl
    …
  25/                 only what a Vaadin 25 project needs *differently*
    pom.xml.tmpl
    application.properties.tmpl
    java/Application.java.tmpl
    …
  25.2/               only what changed again at 25.2
    pom.xml.tmpl
```

**Named by the version the contents are true from.** A directory's name is a
version prefix — `25`, `25.2`, in principle `25.2.3` — read as the lowest version
it stands for: `25` is 25.0.0, `25.2` is 25.2.0. Generating for version X uses
the directories whose floor is at or below X. `25.2.6` uses `25.2`, `25`, `24`;
`25.1.0` uses `25` and `24`; `24.10.9` uses `24` alone. A directory with a floor
above X is never consulted, so `25.3/` can be committed before 25.3 is GA without
touching a 25.2 project.

**Why not one directory per minor, and why not majors only.** What a starter
project contains changes at a major (the Boot generation, the theme, the
starters), and now and then at a minor — `compat.json` already records Boot
tightening at 25.1 and 25.2, and a Boot minor can rename a starter the pom names.
A directory per minor would be eleven near-empty directories for the 24 line; a
directory per major could not express a change at 25.2 without an `if` in the
template, which is the thing being removed. With the floor rule the granularity
is decided per directory, not once: as coarse as is true, a new directory only
when something actually differs. The same vocabulary as `compat.json`'s `line`
and `since`, and the same as the version the user types, so nobody translates.

**Why at the top and not under `templates/java/`.** The file that differs most
between two lines is `pom.xml.tmpl` — the README's Scope section says so — and
`application.properties.tmpl` and `README.md.tmpl` follow it. A version directory
under `java/` only would leave those three to conditionals, and one rule for
every file is one lookup, one test and one thing to explain. Manifest `src` paths
stay as they are (`java/Application.java.tmpl`), resolved against a layer rather
than the root.

### 2. Overlay lookup, not complete copies

Each directory holds only the files that differ from the layers below it. A
template is read from the first directory in the chain — newest floor first —
that has it. `25.2/pom.xml.tmpl` shadows `25/pom.xml.tmpl`, which shadows
`24/pom.xml.tmpl`; `run.sh`, which names no project and no version, lives once in
`24/` and every line gets it.

The alternative is a complete tree per directory. It reads well — open `25.2/`
and the whole project is there — and it is what makes a base-layer edit unable to
reach a line it was not meant for. It costs a copy of twenty-odd files per
directory, and a fix to `run.sh` made in one and not the others. Plan 23 already
documents that failure for a file duplicated *twice* (`templates/commit-msg`
against `.githooks/commit-msg`); this would be the same file duplicated once per
line, with a test asserting the copies agree as the only guard. Overlay has the
opposite failure mode — an edit low in the chain reaches every line above it that
has not shadowed the file — and that is usually the point (a `run.sh` fix) and
occasionally not (a workaround for one line). The discipline for the second case
is copy-up: put the file in the layer the change is for, then edit it there. The
tests in section 6 render every supported line and CI compiles each, so an edit
to `24/` that breaks 25 fails before it merges, and an override identical to the
file below it is refused as the drift-in-waiting it is.

Presence is not part of the overlay. Whether a file exists at all is the
manifest's `when`, as it is today, so a template wanted by one line only gets a
predicate rather than a tombstone file in the directory above:

```go
// atLeast reports whether the project's Vaadin is the given version or newer,
// for a manifest entry one line has and another does not.
func atLeast(vaadin string) func(config.Config) bool {
	return func(c config.Config) bool { return version.Compare(c.VaadinVersion, vaadin) >= 0 }
}
```

Nothing in today's manifest needs it. It is here so the answer to "how do I drop
a file in 26" is a line in the manifest and not a convention.

### 3. `internal/version`: a prefix as a floor

`version.Parse` (`version.go:22`) requires at least `major.minor`, so `25` on its
own does not parse. One function beside it:

```go
// ParseFloor reads a version prefix — "25", "25.2", "25.2.3" — as the lowest
// version it names, so that a template directory called 25.2 sorts after 25.1.9
// and before 25.2.6. Qualifiers are refused: a floor is a number.
func ParseFloor(name string) (Version, bool)
```

`Compare` ignores qualifiers, so `25.3.0-beta1` sits at or above a `25.3` floor,
which is right: a pre-release of 25.3 wants 25.3's templates.

### 4. `internal/generate`: layers

`New` reads the version directories once and refuses a tree it cannot make
sense of; an embedded tree is fixed at build time, so this is a build-time
assertion with a runtime signature.

```go
// layer is one template directory: its name, and the version it is true from.
type layer struct {
	name  string
	floor version.Version
}

// New reads the template root. Every entry there is a version directory; a name
// that is not a version floor, or a file at the top, is an error naming it.
func New(templates fs.FS) (*Generator, error)

// Layers lists the directories consulted for a Vaadin version, newest floor
// first: every directory whose floor is at or below it. An empty chain is an
// error naming the oldest floor there is — a version the rules allowed and the
// tree has nothing for, which the tests in 6 exist to prevent.
func (g *Generator) Layers(vaadin string) ([]string, error)

// open reads a template from the first layer that has it, and says which.
func (g *Generator) open(chain []string, src string) (body []byte, from string, err error)
```

`Render` calls `Layers(c.VaadinVersion)` once and `open` per manifest entry in
place of the `fs.ReadFile` at `generate.go:124`. The error for an empty chain:

```
no templates for Vaadin 23.3.0: the oldest set is for 24
```

It is an error and not a silent fall-back to the oldest directory because the
case only arises when the rules and the tree disagree — a `--compat` override
marking a line supported that this binary has no templates for — and a project
rendered from another line's templates is the outcome plan 04 was written to
stop. `Validate` runs before `Write` on both paths (`main.go:247`, `:300`), so
for the embedded rules the message is unreachable once the agreement test below
is green.

`File` (`generate.go:107-112`) gains `Source string`, the layer-qualified template
path (`25.2/pom.xml.tmpl`). It is what the tests assert on, and what `--dry-run`
(`main.go`, `printDryRun`) can print under its heading — `templates 25.2 → 25 →
24` — so that "which pom did I get" is answered without opening three files.

`main.go:189` handles the new error the way it handles `fs.Sub`'s.

### 5. `compat.json` and the tree agree, and a test says so

Keep the flag; do not derive it from the directories. Two reasons. The rules file
is overridable (`--compat`, the per-user path) and is checked offline by
`checkcompat` against Maven Central — a `supported` that meant "whatever
directories this binary happened to embed" could not be written in the file,
overridden, or checked. And the flag is the deliberate switch: a `24/` tree can
be built across several pull requests behind `supported: false`, and the day it
is flipped is a one-line, reviewable change.

What replaces the sentence in the `$comment` is a test in `internal/generate`,
reading `../../compat.json` with `compat.Parse` the way the tests already read
`../../templates`:

- every supported line has a directory whose floor is in it (`24` or `24.x` for
  line 24) — without one, generating that line is the error in 4;
- every directory's floor is in a supported line — a `26/` with 26 unsupported is
  templates nobody can reach, and the flag should be flipped or the directory
  should not be there yet.

Both directions, so that adding a line is two edits that fail separately when
one is forgotten. `$comment` in `compat.json` points at the test.

### 6. The first move

`git mv templates/* templates/25/` — everything, since the current tree is true
for the whole 25 line (`compat.json` supports it from 25.0.0, and the Lumo
utility stylesheet the templates load is a 25.0 fact). Nothing else changes and
every existing test passes with the `templates(t)` helper taking the new `New`.

When 24 is added, the files it shares with 25 move *down* — `git mv
templates/25/run.sh templates/24/run.sh` and so on — and `25/` keeps only what
differs. That is the one-time cost of overlay when the older line arrives second;
git follows a move, so the history of `run.sh` does not restart. `24/`'s
`Application.java.tmpl` has no Aura branch, and needs none: `Rules.CheckTheme`
(`compat.go:483`) already refuses `aura` on a line whose `themes` does not list
it.

### 7. Documentation

`README.md` Scope (`:192-199`) and Layout (`:203-226`) describe the tree and
the rule; `REQUIREMENTS.md` 7.2 says templates are resolved through the layers
for the project's Vaadin version, and 13.1 gains the tests below. The README's
"a new template is a template plus a line there" stays true, with "in the
directory of the version it is true from" added.

**Order:** 3, 4 and 6 together — they are one change that leaves behaviour
identical. Then 5. Then the test and CI changes below, which are what make a
second line safe to add. Nothing here adds a line; it makes adding one a
directory and a flag.

## Test

- `internal/version`: `ParseFloor("25")` is 25.0.0, `("25.2")` is 25.2.0,
  `("25.2.3")` is 25.2.3; `("v25")`, `("latest")` and `("25.2-beta1")` are
  refused. `Compare` of a floor against a release: 25.2.0 ≤ 25.2.6, 25.2.0 ≤
  25.2.0, 25.2.0 > 25.1.9.
- `internal/generate`, on an `fstest.MapFS` with `24`, `25` and `25.2`:
  `Layers("25.2.6")` is `[25.2 25 24]`, `("25.1.0")` is `[25 24]`, `("24.10.9")`
  is `[24]`, `("25.3.0-beta1")` is `[25.2 25 24]`; `("23.3.0")` errors naming 24
  as the oldest set. A file in all three renders from `25.2` with `Source`
  saying so; a file only in `24` renders from `24` for a 25.2 project. `New`
  refuses a root with a file at the top or a directory called `v25`.
- `internal/generate`, on the real tree:
  - the combination matrix runs per supported line. `everyCombination`
    (`generate_test.go:45-60`) crosses the 32 option combinations with the themes
    the line ships (`rules.Themes`) rather than a fixed pair, so 25 renders 64
    configurations and a 24 line renders 32 without an Aura it does not have.
    The version pair per line is a small table in the test — `25` → 25.2.6 on
    4.1.1, Java 21 — with an assertion that every supported line has a row and
    that `rules.Check` accepts each row, so the examples stay true. `describe`
    (`:63-69`) puts the line first: `25/aura/database+auth`.
  - agreement, both directions, as in 5.
  - every file in every layer is named by a manifest `src` — a file no entry
    reaches is a copy left behind when the manifest moved on, and overlay is
    what makes that easy to miss.
  - no file in a layer is byte-identical to the same path in the layer below it
    for the versions that reach it — an override that changes nothing is the
    twin plan 23 warns about, with a test instead of a comment.
  - `TestSharedFilesAreCopiedVerbatim` (`:325-343`) reads the original through
    `File.Source` rather than from `../../templates/run.sh`, which no longer
    exists.
- `internal/config`: nothing new. The line check stays where it is; this plan
  makes it true rather than changing it.
- CI, `.github/workflows/build.yml` `generated-project` (`:97-114`): the matrix
  gains a line, so `core` and `full` run once per supported line — four legs with
  24 added, in parallel, and the only place a Boot 3 pom meets Maven. Each leg
  carries its `--vaadin-version` and the JDK for `setup-java` (`:158-163`), which
  has to match the pom the leg generates: 21 for 25, 17 for 24 so the older
  line's floor is compiled at least once. The Vaadin version is a bare line —
  `--vaadin-version 24` — resolved on the scripted path (`main.go:277-297`) to
  the newest release the lookup found in that line, the way `--vaadin-version`
  omitted resolves to the newest overall (`:280`); `ValidVersion`
  (`config.go:363`) accepts the line shape for that flag only, and the resolved
  release is what `Validate` sees. That keeps a version out of the workflow file,
  where plan 05 says pinned numbers go to die. If that resolution is not wanted,
  the leg pins a release and plan 05's job watches it — but a line is what a
  person typing `--vaadin-version 24` means, and it is one filter over a list
  the tool already has.
- The `tool` job's smoke test (`build.yml:79`) is unchanged: the default line is
  one of the supported ones, and rendering all of them dry is what `go test`
  does the step before.

## Sources

- `README.md`, Scope, on why Boot 3 and Boot 4 do not share a pom.
- `compat.json`, `$comment`, on what `supported` is meant to mean.
- Plan 23, on what happens to a file kept in two places.
