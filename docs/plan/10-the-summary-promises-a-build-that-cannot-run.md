# "Next: ./run.sh run" is wrong for a project that cannot build yet

## Where it stands

`printResult` in `main.go` always ends with the same next steps:

```
Next
  cd book-shelf
  ./run.sh env     bring up the development stack
  ./run.sh run     start the application
```

For one combination of answers those instructions cannot work. `--traceable` makes
the generated build require `-Dbuild.commit`, and `run.sh` takes that SHA from
`git rev-parse HEAD`. With `--no-git`, or with `--no-commit`, there is no commit —
so the very first suggested command exits 1.

This is not an oversight in the design; it is the design working, and it is
already handled well at both ends. `initRepository` makes the first commit
specifically so that "generated" and "buildable" are not two different states, and
`run.sh` distinguishes "this repository has no commits yet" from "this is not a
repository" and prints the right fix for each. What is missing is the tool saying
so at the moment the user chose it, rather than the project saying so a minute
later.

The same applies to the softer case: `result.GitMessage` already carries git's
complaint when the commit could not be made — no identity configured, for
instance — and that message is printed under the summary box while the next steps
below it still say `./run.sh run`.

## What to do

Make the next steps follow from what actually happened, which `Result` already
records (`GitInit`, `Committed`, `GitMessage`):

- When the project is traceable and no commit was made, put the commit first:

  ```
  Next
    cd book-shelf
    git add -A && git commit -m 'chore: initial commit'   this build requires a commit SHA
    ./run.sh run
  ```

- When `--no-git` was passed with `--traceable`, say the pair is a deliberate
  combination that leaves the project unbuildable until it is a repository with a
  commit, and name the escape hatch the generated `run.conf` already documents:
  dropping `COMMIT_PROPERTY`.

Both are additions to `printResult`; no new state is needed.

## Worth deciding at the same time

Whether `--yes --traceable --no-git` should warn on the way in rather than only on
the way out. Not an error — the combination is legitimate for someone generating
into an existing tree they will commit themselves — but it is the one flag pair
whose result is a project that does nothing until a further step is taken, and the
tool knows it before it writes a file.

## Test

`internal/generate`'s git tests already cover the states (`TestCommitCanBeDeclined`,
`TestGitCanBeDeclinedEntirely`, `TestExistingHistoryIsNotTouched`). The next-step
selection is presentation and belongs in a small table test beside `printResult`,
which means extracting it from that function — worth doing on its own: it is
currently the only part of the tool's output with a branch in it and no test.
