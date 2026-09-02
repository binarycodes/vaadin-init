# The shared task runner talks about things a generated project does not have

## Where it stands

`templates/run.sh` is copied verbatim into every project, deliberately: it names
no project, which is what lets every project share it and take a newer copy
without a merge. But it does name several things that no generated project
contains, because it grew up in a project that had them:

| Where | What it says | In a generated project |
| --- | --- | --- |
| header comment | pins `JAVA_HOME` because "under a newer JDK than Lombok supports it silently generates no getters" | there is no Lombok; the pom does not mention it |
| `task_preview` | "named for the Claude Code preview pane, which launches the app through this task (see `.claude/launch.json`)" | no `.claude/` directory is generated |
| `usage`, the `env` line | "quadlets under podman, compose under docker" | no quadlets are generated, so podman uses compose too |
| `quadlet_*`, `install_quadlets` | ~80 lines driving systemd units | dead in every generated project |

None of it is broken. `has_quadlets` correctly falls through to compose, and the
Lombok sentence is only a comment. But the file is the first thing a new project's
author reads when they want to know how their build works, and a third of what it
tells them is about a project that is not theirs. The `usage` function is
explicit that this is the standard to hold: it "describes only what this runner
does, never what a project's build is configured to do with it", because "shared
text that claims a coverage gate or a browser test is wrong for the first project
that has neither".

The same argument applies to a task named after a preview pane that is not wired
up, and to a help line promising a quadlet path that does not exist.

## What to do

Decide, per item, whether the generated project should grow the thing or the
runner should stop mentioning it:

- **Lombok.** Drop it from the comment. The reasoning survives without the
  example — a bare `mvn` picking the machine's default JDK is bad on its own
  terms, and the generated pom's `-Werror` makes it worse, which is a live
  example rather than a remembered one.
- **The preview pane.** Either generate `.claude/launch.json` pointing at
  `./run.sh preview` — one small template, and the task then means something — or
  make `preview` an undocumented alias and drop the line from `usage`. Generating
  it is the better answer: the file is three lines and the task already exists.
- **Quadlets.** Either generate the quadlet units for a project whose stack has
  services (they buy systemd ordering and `Notify=healthy` readiness, which the
  compose path gets from `--wait`), or reword the `env` help to describe what the
  project actually ships and leave the quadlet code as the runner's support for
  projects that add their own. Rewording is the cheap correct move; generating
  units is a feature, and should be argued on its own.

## While there: say which runner this is

The runner's whole design is "a project can take a newer `run.sh` without a
merge", and there is no way to tell which one a project has. A
`RUNNER_VERSION="…"` near the top, printed by `./run.sh help`, is what makes that
promise checkable — and would let a future `vaadin-init upgrade` compare rather
than guess.

## Test

`internal/generate` already asserts `run.sh` is copied byte-for-byte
(`TestSharedFilesAreCopiedVerbatim`); that is the property to keep. If
`.claude/launch.json` is generated, add it to the manifest and to the
always-generated list.
