# vaadin-init

Bootstraps an opinionated Vaadin and Spring Boot project. One binary, no
toolchain to install first, and a TUI that asks about eight things and then writes
a project that builds.

```
$ vaadin-init

  Coordinates
  What this project is called to Maven.

  Group ID
  > io.binarycodes

  Artifact ID
  > book-shelf
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

## Development

```sh
make check     # gofmt, go vet, go test
```

`internal/generate`'s tests render all 32 combinations of the five options and
check each one: the pom is well-formed XML, the realm is valid JSON, no file
carries an unresolved template value, every Java file declares the package its
path implies, and each optional file appears exactly when its option is on.

`internal/prompt`'s tests drive the whole conversation through huh's accessible
mode, which is also what `--accessible` gives a screen reader.
