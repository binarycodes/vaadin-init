# Let the user choose between Aura and Lumo, with Aura as the default

## Where it stands

The generated project is hard-wired to Lumo:

- `templates/java/Application.java.tmpl:5,23` — `import com.vaadin.flow.theme.lumo.Lumo;` and
  `@StyleSheet(Lumo.STYLESHEET)`.
- `templates/styles/main-view.css:8` — `padding: var(--lumo-space-l);`, a Lumo-only
  custom property.

Aura is the default theme in Vaadin 25 and Lumo is the alternative kept for
continuity. A tool whose whole reason for existing is to start a project on the
current generation of the framework should therefore start it on Aura, and Lumo
should be a choice the user makes rather than one the templates made for them.

The choice is not cosmetic for the generated code. The two themes have disjoint
style-property namespaces — `--lumo-*` against `--aura-*` — and the Lumo Utility
Classes only work under Lumo, so a project that starts on the wrong one pays for
it in every stylesheet written afterwards.

## What to do

Add one answer, `Theme`, taking `aura` or `lumo`, defaulting to `aura`.

**Config** — `internal/config/config.go`

```go
// Theme is the Vaadin theme the generated project loads: "aura" or "lumo".
Theme string
```

with `ValidTheme` alongside the other rules, and a `Validate` entry. Two helpers
keep the conditionals out of the templates:

```go
func (c Config) Aura() bool { return c.Theme == ThemeAura }
func (c Config) Lumo() bool { return c.Theme == ThemeLumo }
```

Add `Theme` to `Selected()`? No — `Selected` lists optional pieces that are on,
and a theme is always on. It belongs in the summary as a row of its own (see
below), not in the options list.

**Defaults** — `defaults.toml` and `internal/config/defaults.go`

```toml
# The Vaadin theme the generated project loads. Aura is Vaadin 25's default and
# the one new work should start on; lumo is the earlier theme, kept for projects
# that have to match an existing Lumo-based design.
theme = "aura"
```

A top-level key, not one under `[features]`: `[features]` is a table of booleans
mapped from a multi-select, and a theme is neither.

**Flag** — `main.go`

`--theme aura|lumo`, defaulting to `cfg.Theme` so `--help` shows this machine's
default like every other flag. An invalid value is caught by `Validate`, which
already runs on both paths.

**Prompt** — `internal/prompt/prompt.go`

A `huh.NewSelect[string]` in a group of its own, between Identity and Versions:

```go
huh.NewGroup(
    huh.NewSelect[string]().
        Title("Theme").
        Description("Aura is Vaadin 25's default. Lumo is the earlier theme.").
        Value(&c.Theme).                      // before Options, as versionSelect explains
        Options(
            huh.NewOption("Aura — the Vaadin 25 default", config.ThemeAura),
            huh.NewOption("Lumo — the earlier theme", config.ThemeLumo),
        ),
).Title("Theme"),
```

`Value` before `Options` for the reason `versionSelect` already documents: `huh`
scans for the bound value inside `Options(...)` to decide which line to open on.
Two options and a one-line description fit any terminal, so no `Height` juggling
is needed — but the layout test should cover it anyway, since that is the class
of fault `internal/prompt/layout_test.go` exists to catch.

Accessible mode needs nothing special: a select renders as a numbered list there.

**Templates**

`templates/java/Application.java.tmpl`:

```
{{- if .Aura}}
import com.vaadin.flow.theme.aura.Aura;
{{- else}}
import com.vaadin.flow.theme.lumo.Lumo;
{{- end}}
...
{{- if .Aura}}
@StyleSheet(Aura.STYLESHEET)
{{- else}}
@StyleSheet(Lumo.STYLESHEET)
@StyleSheet(Lumo.UTILITY_STYLESHEET)
{{- end}}
@StyleSheet("styles.css")
```

A Lumo project loads the Lumo Utility Classes too. In Vaadin 25 they are no
longer loaded through `theme.json`; the only way in is
`@StyleSheet(Lumo.UTILITY_STYLESHEET)`, and the symptom of not having it is a
layout built from `LumoUtility` constants rendering with no styles at all and no
error anywhere. Anyone who picks Lumo over the Vaadin 25 default is almost
certainly matching an existing Lumo codebase, and those codebases use the
utility classes, so the annotation belongs in the template rather than in a
README note the user finds after the layout has already broken. Aura has no
counterpart, so nothing is added on that side.

The class comment there still says `@Theme` "is deprecated in Vaadin 25"; while
editing it, make it say which theme is loaded, that the other is one flag away,
and why the Lumo branch carries two stylesheets.

`templates/styles/main-view.css` is currently copied verbatim
(`internal/generate/generate.go`, the `styles/main-view.css` entry). Two ways out,
in order of preference:

1. **Write it against the base style properties.** Vaadin 25's base styles expose
   `--vaadin-*` properties in every theme, so `padding: var(--vaadin-padding)`
   would be correct under both and the file could stay verbatim. Verify this
   against the base-style reference before committing to it — if it holds, this is
   the whole change and nothing else in the CSS has to be conditional.
2. **Render it.** Drop `verbatim: true` and template the one declaration. Cheap
   today, and it puts a theme conditional in a place a later reader has to
   remember to keep in step with the Java.

Prefer (1). It is the only option that keeps a new view's stylesheet
theme-agnostic by default, which is the property that matters as the project
grows past one view.

**Generated README** — `templates/README.md.tmpl`

A short section saying which theme this project loads, where the constant lives,
and the edit that swaps it — the file is the answer to "why is it like this?",
and "why does my `--lumo-*` property do nothing?" is exactly the question an Aura
project will produce. For a Lumo project the section also names the utility
stylesheet, so a reader swapping to Aura knows to drop it along with the theme
and that `LumoUtility` constants stop working when they do.

**Summary** — `main.go`, `printResult`

The stack row already reads `Vaadin 25.2.6 · Spring Boot 4.1.1 · Java 21`. Append
the theme to it rather than adding a row: it is part of what was pinned, and a
five-row box for a four-fact result reads as padding.

## Also worth deciding while here

- **Lumo Utility Classes.** Decided above: loaded whenever Lumo is chosen, never
  under Aura. Nothing generated uses them yet, so the cost is one extra stylesheet
  request on a Lumo project; the alternative is a failure mode with no error
  message.
- **Color scheme.** Aura ships light and dark schemes and an `@ColorScheme`
  annotation. That is a separate question and has its own file:
  `03-color-scheme-light-dark.md`.

## Tests

- `internal/generate`: the combination matrix (`everyCombination`, currently 32
  configs over five booleans) has to run under both themes. Cross the existing
  matrix with the two themes rather than adding a third dimension of booleans —
  64 renders, still milliseconds.
- A direct assertion in the same package: an Aura config's `Application.java`
  names `Aura.STYLESHEET` and no `Lumo`; a Lumo config's names both
  `Lumo.STYLESHEET` and `Lumo.UTILITY_STYLESHEET` and no `Aura`. The existing
  `TestPomNamesADependencyOnlyWhenItIsUsed` is the shape to copy.
- `internal/config`: `ValidTheme` accepts both values and rejects anything else;
  the embedded defaults produce `aura`.
- `internal/prompt`: the theme select appears in the accessible flow and its
  answer reaches the `Config`; a layout assertion that both options are on screen.
- CI (`.github/workflows/build.yml`, the `generated-project` job): the "everything
  on" project should be generated with `--theme lumo` and the bare one left on the
  Aura default, so both themes are compiled by a real JDK on every push without a
  third job.

## Risk

The Aura artifacts ship in `vaadin-core`, so no new dependency — confirm against
the Vaadin version the tool defaults to before merging, since the pom's comment
about staying on the free components is a promise this must not quietly break.
