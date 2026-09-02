# The README points at releases that nothing produces

## Where it stands

The README's install section opens with:

> Download the binary for your platform from the releases, or build it.

There is no release. `.github/workflows/build.yml` is the only workflow; its
`cross-compile` job runs `make dist` and uploads the six binaries as *workflow
artifacts*, which expire, require a GitHub login to download, and are not what
anyone means by "the releases". No workflow is triggered by a tag, nothing
publishes, and `make dist` produces no checksums.

The Makefile even stamps a version for this — `VERSION ?= $(shell git describe
--tags --always --dirty)`, linked in as `main.version` so `--version` names a
commit — which only pays off if tags exist and correspond to something
downloadable.

For a tool whose entire distribution argument is "one static binary, nothing to
install first", the download being unavailable is the gap that matters most.

## What to do

Add `.github/workflows/release.yml`, triggered on `v*` tags:

1. `make dist` — it already cross-compiles all six targets, checks formatting,
   vetting and tests first, and builds with `-trimpath`.
2. Generate `SHA256SUMS` over `dist/` and publish it alongside the binaries. A
   static binary downloaded over HTTPS from a redirect chain is exactly the thing
   people want to verify, and it costs one `sha256sum` line.
3. Create the GitHub release with the binaries and the checksum file attached.
4. `permissions: contents: write` scoped to that job alone — the existing
   workflow's `contents: read` default is right and should stay.

Two things worth deciding while writing it:

- **Archives or bare binaries.** Bare binaries are simpler to `curl` and are what
  the README implies. If archives are used, the platform belongs in the file name
  either way — `make dist` already names them
  `vaadin-init-<os>-<arch>[.exe]`.
- **Provenance.** GitHub's artifact attestation is a few lines and gives a
  verifiable statement that a binary came from this repository's workflow. For a
  tool that people will run without reading, it is worth more than it costs.

## Then, and only then, the packaging question

A Homebrew tap, a Scoop manifest, `go install` instructions — all of them are
downstream of releases existing. `go install github.com/binarycodes/vaadin-init@latest`
already works today for anyone with a Go toolchain and deserves a line in the
README regardless, since it is the one install path that needs no release at all.

## Test

The release path is exercised by tagging, which cannot be tested before it is
used. What can be done cheaply: have the `cross-compile` job produce the checksum
file too, so the only untested step at tag time is the publish itself.
