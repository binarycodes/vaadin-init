# The coverage gate passes by covering nothing

## Where it stands

`--coverage` adds JaCoCo with a rule that is deliberately narrow
(`templates/pom.xml.tmpl:330-350`):

```xml
<element>PACKAGE</element>
<includes>
    <include>{{.Package}}.*.service</include>
    <include>{{.Package}}.*.ui.presenter</include>
</includes>
```

The narrowness is right, and the generated README argues for it well: a
project-wide percentage moves when a view gains a layout and can be met by testing
the easy half.

But a JaCoCo rule whose includes match no package is satisfied, not violated. And
in a generated project:

- `{{.Package}}.*.ui.presenter` matches nothing **ever**. No template generates a
  presenter package; `MainView` is a view with its logic inline, and
  `templates/java/MainView.java.tmpl` documents that as the design.
- `{{.Package}}.*.service` matches exactly one package, `…notes.service`, and only
  when `--database` is on. `--coverage` without `--database` — a combination the
  test matrix generates and CI compiles — produces a build with a coverage gate
  that can never fail.

So the option delivers, in half its configurations, a plugin, a report, an
`argLine` interaction documented at length in the pom, and no gate. Someone reads
"coverage · 80%" in the summary box and believes something is being enforced.

## What to do

Decide which of these the option is meant to be, and make the generated project
say so:

1. **Fail when the gate covers nothing.** JaCoCo will not do this for you. A
   second rule, or the enforcer, would have to assert that at least one included
   package exists. Correct, and heavy for what it buys.
2. **Generate the packages the gate names.** Give the core project a real service
   with a test, so `--coverage` alone gates something. This changes the core
   project's shape to satisfy an option, which is backwards.
3. **Say it, in the generated README.** State plainly that the gate covers
   `*.service` and `*.ui.presenter`, that a project with neither is not gated by
   it, and that the includes are the line to edit as the project grows its own
   packages. One paragraph, no build complexity, and it converts a silent false
   assurance into a documented starting point.
4. **Drop `*.ui.presenter` from the includes** until something generates one, or
   keep it and say in the pom comment that it is there for the packages the
   project will grow, not for the ones it has.

Prefer (3) plus (4). The failure here is a false impression, and the fix for a
false impression is words, not a plugin.

Also worth checking while in there: the summary box in `main.go` prints `coverage`
in its options list. If the gate is documented as conditional, the word in the box
is still an overstatement for a core-only project — `coverage (no gated packages
yet)` is longer but true.

## Test

`internal/generate`: for a `--coverage --database` config, assert the rendered
pom's include patterns actually match the package path of the generated
`NoteService` — a string comparison against `{{.Package}}.notes.service`, not a
regex over the pom. That is the assertion that would have caught this, and it
stays useful when the includes are edited.
