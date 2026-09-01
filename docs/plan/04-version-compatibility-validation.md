# Reject a Vaadin or Spring Boot version this tool's templates cannot support

## Where it stands

The tool targets exactly one generation of each framework, and says so:
`internal/versions/versions.go` declares `VaadinMajor = 25` and `BootMajor = 4`,
the lookup filters Maven Central down to those major lines, and the repository
README's Scope section explains that Vaadin 24 would mean a second set of
templates rather than a conditional.

Nothing enforces it. `config.ValidVersion` checks only the *shape* of a version
string:

```go
versionPattern = regexp.MustCompile(`^\d+\.\d+(\.\d+)?(-[A-Za-z0-9.]+)?$`)
```

So both entry points let a wrong generation straight through:

- the TUI's "type one myself…" escape hatch accepts `24.4.0` or `3.3.5`, and
- `--yes --vaadin-version 24.4.0` never passes through a prompt at all.

The result is a project that renders perfectly and then fails somewhere in Maven,
in a message about a starter artifact that does not exist — with nothing pointing
back at the answer that caused it. The escape hatch exists for a real case (a
release Maven Central had not yet listed, a pre-release), so it should stay; it
just should not be the way to a project that cannot build.

## What to do

Two validators alongside the existing ones in `internal/config/config.go`, taking
the expected major as an argument so `internal/versions`' constants stay the one
place the generation is declared:

```go
// ValidFrameworkVersion is ValidVersion plus the major line this tool's
// templates target. A version from another generation renders a project that
// cannot build, and the failure surfaces in Maven rather than here.
func ValidFrameworkVersion(s string, major int, name string) error
```

Wire it in three places:

- `Config.Validate`, which both entry points already run — this is what catches
  `--yes --vaadin-version 24.4.0`;
- the two `huh.NewInput().Validate(...)` calls for the typed-version escape hatch
  in `versionGroups`, so the TUI says so at the field rather than after the form;
- the accessible-mode version inputs, which use the same rule.

The message should name the constraint and the way out, in the style of the
enforcer plugin's message in `templates/pom.xml.tmpl`:

```
vaadin version: this tool generates Vaadin 25 projects; got 24.4.0. Vaadin 24 sits
on Spring Boot 3 and needs a different pom — see the Scope section of the README.
```

## Test

`internal/config`: accepts `25.2.6` and `25.3.0-beta1`, rejects `24.4.0` and `26.0.0`
for Vaadin; the same for Boot. `internal/prompt`: a typed version from the wrong
generation is refused at the field rather than returned in the `Config`. One
end-to-end assertion that `--yes` with a wrong version fails before anything is
written — `Validate` runs before `generator.Write`, and that ordering is the thing
worth pinning.
