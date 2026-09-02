# Two files pin the PostgreSQL image, and only a comment keeps them in step

## Where it stands

`TestcontainersConfiguration` says it best, in its own javadoc:

> The image is pinned to the one environment/dev/compose.yaml runs, so a test
> cannot pass against a version the application is never deployed on.

That is the right rule, and nothing enforces it. The image name appears as a
literal in two templates that know nothing about each other:

- `templates/compose.yaml.tmpl` — `image: postgres:18-alpine`
- `templates/java/TestcontainersConfiguration.java.tmpl` —
  `new PostgreSQLContainer<>("postgres:18-alpine")`

Change one and the javadoc becomes a false statement, with no test and no reader
positioned to notice. Keycloak has the same shape — `quay.io/keycloak/keycloak:26.0`
in the compose file — though it appears only once today.

## What to do

Make the images `Config`-derived, the way every other cross-file agreement in this
tool already is. `ContainerRequired`, `ITProfile` and `CommitProperty` all exist
precisely so that two generated files cannot disagree; images belong in the same
list:

```go
// PostgresImage is the PostgreSQL the dev stack runs and the tests run against.
// One value, because a test that passes against a different engine version than
// the application is deployed on has tested the wrong thing.
func (c Config) PostgresImage() string { return "postgres:18-alpine" }

func (c Config) KeycloakImage() string { return "quay.io/keycloak/keycloak:26.0" }
```

Methods rather than fields to start with: they are not answers the user gives, and
a field would appear in the TUI's field list as something to ask about. If the
images ever become a question — a project on an older PostgreSQL is a real case —
promoting a method to a field is a small change and the templates do not move.

Then both templates read `{{.PostgresImage}}`, and the javadoc's claim becomes
structurally true rather than aspirational.

## The other half: staleness

Pinning the two together fixes drift, not age. `postgres:18-alpine` and
`keycloak:26.0` go stale exactly like the hand-pinned Maven versions in
`05-pinned-tool-versions-go-stale.md`, and the same weekly job can watch both —
querying a registry's tag list rather than `maven-metadata.xml`. Do them together;
they are one habit, not two.

## Test

`internal/generate`: for a `--database --e2e` config, assert the image string in
the rendered `compose.yaml` is byte-identical to the one in the rendered
`TestcontainersConfiguration.java`. That is a three-line test that encodes the
javadoc's promise, and it is the reason to prefer this over "just be careful".
