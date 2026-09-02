# Every run asks Maven Central again, and an offline run silently uses a stale answer

## Where it stands

`startLookup` fires two HTTPS requests at Maven Central on every invocation of the
tool, with a five-second budget, overlapped with the first questions. The design
is careful — it degrades rather than failing, the timeout is short on purpose, and
`versionNote` tells the user in the TUI when the list came from the built-in
fallback instead.

Two consequences of having no cache:

- **A `--yes` run in a script pays the network cost every time**, and on a slow or
  filtered network pays the full five seconds before generating anything. There is
  no `--offline` to skip it either.
- **The fallback is silent on the scripted path.** `versionNote`'s warning only
  exists inside the TUI. `vaadin-init --yes` on a machine with no network prints a
  summary saying `stack Vaadin 25.2.6 · Spring Boot 4.1.1` with nothing to
  indicate those came from a `defaults.toml` written months ago rather than from
  the current release. The generated project pins them, and nobody finds out until
  someone wonders why a two-week-old project is three patch releases behind.

## What to do

Two independent changes; the second is the important one.

**Cache the lookup.** A small JSON file under `os.UserCacheDir()/vaadin-init/`,
holding the two version lists and when they were fetched. Fresh within a day,
used and refreshed in the background after that. It makes the common case
instant, and it makes an offline run fall back to *yesterday's real answer*
rather than to a constant in a file — which is a much better last resort than the
one the tool has now. Keep the network path exactly as it is; the cache is a
layer, not a replacement, and a corrupt or unreadable cache file must be ignored
rather than reported.

**Say where the numbers came from, on every path.** Add the provenance to the
summary's stack row when it is not the live answer:

```
stack    Vaadin 25.2.6 · Spring Boot 4.1.1 · Java 21
         versions from the built-in defaults — Maven Central was not reachable
```

Two lines of code, and it closes the gap between what the TUI is honest about and
what a scripted run hides. Do this one first: it needs no cache, and it is the
part that prevents a wrong project rather than a slow one.

**While there:** an explicit `--offline` that skips the lookup entirely. It makes
the scripted path deterministic and fast for anyone who has already decided which
versions they want, and it removes the only reason a CI job that passes
`--vaadin-version` explicitly should touch the network at all.

## Test

`internal/versions` already drives `stableVersions` against a local test server,
so the cache is testable without a network: a cold read fetches and writes, a warm
read inside the freshness window does not fetch, and a corrupt file behaves as a
cold read. The provenance line belongs to whichever package ends up owning
`printResult` — see `25-main-go-has-the-logic-and-none-of-the-tests.md`.
