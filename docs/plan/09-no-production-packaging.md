# The project is opinionated up to `mvn package` and silent after it

## Where it stands

The generated project has a considered development environment — `environment/dev`
with compose or podman quadlets, healthchecks on every service so nothing starts
talking to a database that is not ready, `./run.sh env` bringing it up with
`--wait` — and a `./run.sh package` that produces a jar.

Between that jar and anything running anywhere, there is nothing. No container
image, no `environment/prod`, nothing that says how the traceable build's commit
SHA is supposed to reach a build that happens somewhere other than a laptop. The
pom's own comment on the enforcer plugin points at the gap while stepping over it:
"a container build should pass it as a build arg, and CI from its own commit
variable" — a sentence describing a build the project does not have.

## What to do

Not a deployment story; a single artifact and the sentence that goes with it.
Generate a `Dockerfile` at the project root, unconditionally:

- **Multi-stage**, so the produced image carries a JRE and the jar rather than
  Maven, a JDK and a `node_modules`. A Vaadin production build downloads a
  frontend toolchain, and shipping it would multiply the image size for nothing.
- **Production mode.** `-Pproduction` or `-Dvaadin.productionMode=true` — the same
  distinction the `it` profile exists for. An image built without it serves a dev
  bundle and starts a frontend dev server it has no toolchain for.
- **`ARG BUILD_COMMIT`, forwarded as `-D{{.CommitProperty}}`**, when `--traceable`
  is on. This is where that pom comment cashes out: the image build is the one
  place where `git rev-parse HEAD` is not available, because the build context
  usually has no `.git`. Fail loudly with no arg rather than defaulting, the way
  the enforcer already does.
- **A `./run.sh image` task**, so the invocation with its build arg lives beside
  every other task rather than in a README paragraph people retype wrongly.

`.dockerignore` matters as much as the Dockerfile: without one, `target/` and
`node_modules/` are uploaded as build context on every build.

## What to leave out

No production compose file, no Kubernetes manifests, no registry. Those depend on
where the project is going, which the tool cannot know, and a wrong guess is worse
than an absence — the dev stack's `dev-secret` in the Keycloak realm is committed
precisely because it is a development value, and any generated production artifact
risks that convention being copied where it does not belong.

State that boundary in the generated README, in the file's own voice: the project
ships a way to build one image; where it runs is the project's business.

## Test

`internal/generate`: the Dockerfile is generated for every combination, and it
names `BUILD_COMMIT` exactly when `.Traceable` is on. Building the image in CI is
minutes per run and duplicates what the `generated-project` job already proves
about the Maven build — leave it out unless the image build starts breaking.
