# The tool runs on Windows; the project it generates does not

## Where it stands

The tool takes Windows seriously. `make dist` cross-compiles `windows/amd64` and
`windows/arm64`, the CI `tool` job runs on `windows-latest` with a comment
explaining that a shipped Windows binary has to be run there, and the smoke test
even works around `go build -o vaadin-init` writing a file Windows will not
execute.

What that binary generates on Windows is a project whose every documented
entry point is a bash script:

- `run.sh` — 551 lines of bash, and the only path to a build the README describes;
- `.githooks/commit-msg` — `#!/bin/sh`, wired up by `git config core.hooksPath`.

Git for Windows ships a bash, so both work from Git Bash, and WSL works. From
PowerShell or `cmd` — where a Windows user who just downloaded a `.exe` is
standing — `./run.sh run` is not a command. `os.Chmod(target, 0o755)` in
`generate.Write` is a no-op there too, so the executable bit the manifest is
careful about means nothing.

The gap is not that the scripts are bash. It is that nothing says so. The
generated README documents `./run.sh` as *the* interface, and the tool's summary
box prints `./run.sh env` as the next step, on a platform where that line needs a
shell the user may not have opened.

## What to do

Pick one and commit to it, in the generated README and in the summary:

1. **Say it plainly.** A short "On Windows" section: run these from Git Bash or
   WSL, and `git config core.hooksPath .githooks` needs the same. One paragraph,
   honest, no new code. The commit-msg hook is fine — Git for Windows runs hooks
   through its own shell.
2. **Detect and adapt the closing summary.** `main.go` already branches on the
   config to build its next steps; on `runtime.GOOS == "windows"` it could print
   `bash run.sh run` and a line about where to get a bash. Cheap, and it lands
   exactly where the user is about to type the wrong thing.
3. **Ship a `run.ps1`.** A second task runner is a second thing to keep correct,
   and the reason `run.sh` names no project — so every project can share it — is
   the same reason a divergent PowerShell copy would rot. Only worth it if
   Windows is a first-class target rather than a supported one.

Prefer (1) plus (2). They cost a paragraph and a branch, and they turn a silent
dead end into a documented prerequisite.

## Worth checking while there

Whether `--dry-run` and the file tree render correctly in a Windows console —
the box drawing and the `❯` caret assume a font and a code page. The `tool` CI
job runs `--dry-run` on Windows already, so a broken glyph would be visible in
the log, but nobody has looked at it as output rather than as an exit code.
