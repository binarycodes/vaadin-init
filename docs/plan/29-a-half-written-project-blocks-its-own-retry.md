# An interrupted write leaves a directory that stops the retry

## Where it stands

`generate.Render` builds the whole tree in memory before anything is written, and
the package comment explains why:

> A generator that leaves half a project behind is worse than one that refuses:
> the user cannot tell what is missing, and the obvious recovery — run it again —
> is the one thing the half-written directory now blocks.

The reasoning is right and the protection is partial. Rendering is atomic;
*writing* is not. `Write` creates the target directory and then loops over the
files, calling `MkdirAll`, `WriteFile` and `Chmod` for each, returning on the first
error. A disk that fills up, a permission problem three files in, or a Ctrl-C
mid-loop leaves exactly the half-written directory the comment describes — and
the next run hits the "already exists and is not empty; pass `--force`" guard,
which is precisely the blocked recovery it warns about.

The git step has the same shape: `initRepository` runs after the files are
written, so an interrupt between the two leaves a complete project that is not a
repository and, with `--traceable`, cannot build until someone commits it.

## What to do

Write to a sibling temporary directory and rename it into place:

1. `os.MkdirTemp(filepath.Dir(root), ".vaadin-init-*")` — a sibling, so the rename
   stays on one filesystem;
2. write every file into it, and run the git initialisation there too, so the
   result being renamed in is the finished thing rather than a stage of it;
3. `os.Rename` onto the target;
4. remove the temporary directory on any failure, in a `defer` that knows whether
   the rename happened.

The rename makes success and failure the only two outcomes for a *new* directory,
which is the overwhelmingly common case.

**`--force` cannot have this.** Renaming a directory onto a non-empty one does not
merge, and merging file-by-file is precisely what has no atomic form. So `--force`
keeps today's behaviour, and should say so: its flag help ("write into the target
directory even if it is not empty") can carry the caveat that an interrupted
`--force` run leaves whatever it had written.

**Interrupts are the other half.** A Ctrl-C during the write is a signal Go
delivers by killing the process, so the `defer` never runs unless the tool installs
a handler. A minimal `signal.Notify` that removes the temporary directory and
exits 130 — the same code the cancelled-prompt path already uses — closes that,
and is about ten lines.

## Worth stating plainly

This is a small-probability failure with a good error message today. It is on the
list because the package's own comment holds it to a standard it does not quite
meet, not because anyone has hit it. Weigh it accordingly against everything else
here.

## Test

`internal/generate`: a write that fails partway — a file whose parent is made
read-only, or a manifest entry pointed at an unwritable path — leaves the target
directory absent, and a second run then succeeds without `--force`. That is the
property, and it is testable without simulating an interrupt.
