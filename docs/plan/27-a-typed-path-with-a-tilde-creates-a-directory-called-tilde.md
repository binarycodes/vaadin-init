# A directory typed as `~/projects/app` becomes a directory named `~`

## Where it stands

The output directory is asked for as free text — "Created if it does not exist.
Must be empty." — validated only as non-empty, and then handed to
`filepath.Abs(c.OutputDir)` in `generate.Write`.

Go does not expand `~`. A shell does, so `--dir ~/code/app` on a command line
works and hides the problem entirely; typing `~/code/app` at the TUI's Directory
prompt does not, and the tool creates a literal directory named `~` under the
current working directory, with `code/app` inside it. The summary box then prints
that path back — as `~/code/app`, which is exactly what the user typed and reads
as success.

It is a small bug with an annoying tail: the stray `~` directory is easy to create
repeatedly and easy to miss, and on the second attempt the "already exists and is
not empty" guard fires against a path the user does not believe they created.

The same applies to `$HOME/code/app` and to any other shell syntax typed into a
prompt that is not a shell.

## What to do

Expand a leading `~` (and `~/`) when the answer comes from a prompt rather than
from a shell:

```go
// Expand is what a shell would have done to a path typed at a prompt. Only a
// leading ~ or ~/: a tilde anywhere else is a legal character in a directory
// name, and expanding it there would break a path that is already correct.
func Expand(path string) (string, error)
```

Where to apply it is the real decision. Applying it in `config` — as part of
normalising `OutputDir` before `Validate` — covers both entry points and means
`--dir '~/code/app'` (quoted, so the shell did not expand it) works too, which is
what a user would expect. That is the better place; expanding only in the prompt
would leave the two paths behaving differently for the same input.

Environment variables are a step further and should not be taken: `$HOME` in a
directory name is rarer, and a tool that expands arbitrary variables in a value it
then writes to disk has taken on a job nobody asked it to do.

## While there

The Directory prompt's description could say what it accepts — "relative to here,
or an absolute path" — which is a one-line fix for the same confusion, and worth
doing whether or not the expansion lands.

## Test

`internal/config`: `~` alone, `~/x`, `~x` (a real directory name, unchanged),
`x/~/y` (unchanged), an absolute path (unchanged), and the `HOME`-unset case,
which should leave the path alone rather than fail — the user gets the "already
exists" or "cannot create" error from the real path, which is still better than an
error about their environment.
