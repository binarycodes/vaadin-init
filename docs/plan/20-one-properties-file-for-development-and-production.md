# Development values ship to production in the only properties file there is

## Where it stands

`application.properties` is generated once and carries everything:

```properties
vaadin.launch-browser=true
spring.datasource.password=dev
spring.security.oauth2.client.registration.keycloak.client-secret=dev-secret
spring.security.oauth2.client.provider.keycloak.issuer-uri=http://localhost:8081/realms/…
logging.level.{{.Package}}=DEBUG
```

The file is honest about what these are. Two comments say the credentials are
development values and that "a deployment supplies its own through the
environment", and `vaadin.launch-browser` carries a note that it is read only in
development.

But the mechanism does not match the intention. There is one profile, so a
deployment does not *supply* its own values — it has to *override* each one, and
the failure mode of forgetting is not an error. A deployment that misses the
issuer URI points at `localhost:8081` and fails at login; one that misses the
password fails at start-up, which is the good case; one that misses
`logging.level.<package>=DEBUG` logs every statement of a production application
at DEBUG forever, and nobody notices because nothing is broken.

## What to do

Split along the line the comments already draw:

- `application.properties` — what is true everywhere: the application name, the
  JPA settings, `open-in-view=false`, the OIDC *registration* minus its secret.
- `application-dev.properties` — the dev stack: local datasource credentials,
  `launch-browser`, the `localhost:8081` issuer, `DEBUG` logging.
- Start `dev` by default where it belongs: `spring.profiles.active=dev` is one
  option; `./run.sh run` passing `-Dspring-boot.run.profiles=dev` is another and
  is better, because the profile is then a property of *how you ran it* rather
  than of the artefact. The task runner already exists to hold exactly that kind
  of decision.

Then a deployment that supplies nothing gets a missing-property failure at
start-up instead of a silent connection to a database that is not there — which
is the same argument `ddl-auto=validate` and the `build.commit` enforcer already
win elsewhere in this project.

## The secret

`dev-secret` in a committed file is fine and the README says why. Worth adding
while splitting: a one-line comment in the production-facing file naming the
environment variable that supplies the real one
(`SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_KEYCLOAK_CLIENT_SECRET`), because the
relabelling from dotted property to environment variable is the step people get
wrong.

## Test

`internal/generate`: with `--auth`, no production-facing properties file contains
`dev-secret`, and the dev profile does. That assertion is short, and it is the one
that stops the split from quietly regressing the first time a property is added to
the wrong file.
