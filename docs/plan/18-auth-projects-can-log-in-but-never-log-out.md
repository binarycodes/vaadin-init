# A project with auth can log in, and has no way to log out

## Where it stands

`--auth` generates a complete login: `SecurityConfig` with
`VaadinSecurityConfigurer`, the OIDC client registration in
`application.properties`, a Keycloak in the dev stack with a realm imported and a
`dev`/`dev` user, `@PermitAll` on the root view, and an integration test asserting
an anonymous visitor is redirected.

There is no logout anywhere. `MainView` gains an annotation and nothing else — no
logout button, no indication of who is signed in, and no handler for OIDC
RP-initiated logout, so nothing ends the session at the identity provider either.
The generated README's auth section documents the login and stops.

For a template project this matters more than a missing feature usually would. The
first thing anyone does with a generated auth project is log in as `dev`; the
second is want to be somebody else. Without a logout, that means clearing cookies
or an incognito window — and the person doing it is left assuming the framework
makes logout hard, which it does not.

There is also a correctness trap waiting: logging out of a Vaadin application by
invalidating the HTTP session alone leaves the client with a stale UI and, against
an OIDC provider, leaves the provider's own session intact — so the next login
silently succeeds without asking for credentials, which looks exactly like a
broken logout.

## What to do

Add the smallest honest version to the `--auth` project:

- **Show who is signed in and offer a way out.** The current user's name from the
  OIDC principal, and a logout control beside it, in `MainView` (or in a small
  header component the view uses). Guarded by `{{- if .Auth}}` like the rest of
  the auth-conditional code in that template.
- **Use the framework's logout rather than a hand-rolled one**, and say why in a
  comment: Vaadin's Spring security support has a logout handler that closes the
  UI down cleanly, and the OIDC case needs the provider's end-session endpoint or
  the next login skips the password prompt. That comment is worth as much as the
  code — it is the trap above, written down at the place someone would otherwise
  fall into it.
- **A line in the generated README's auth section**, next to "this application
  never sees a password": where the logout is, and what it ends.

## Scope

Resist growing this into a login page, a user menu, or roles. `--auth` is a worked
example of "this application is protected and knows who you are"; logout is the
missing half of the smallest complete version of that. `@RolesAllowed`, a second
view with a different rule, and a real login test all belong in the README's "to
grow this" paragraph, where `ProtectedRootIT`'s javadoc already puts its own.

## Test

`MainViewTest` runs `@WithMockUser`, so it can assert the logout control is
present and enabled — a browserless assertion, no browser needed. Asserting the
session actually ends is an integration test with a running Keycloak, which is
exactly the test `ProtectedRootIT` explains the project deliberately does not
have; leave it out and say so.
