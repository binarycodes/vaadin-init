# You cannot go back to a wrong answer, and you cannot replay a good run

## Where it stands

The conversation is two `huh` forms, run one after another, for a reason the code
states clearly: the second form's defaults are derived from the first form's
answers, and `huh` binds a field's initial value when the form is built.

The consequence is that `huh`'s own backwards navigation stops at the seam. Inside
the second form, shift-tab walks back through Identity, the versions, the stack
and the output directory. It cannot reach the Coordinates form. So a group id
noticed to be wrong at the "Generate?" confirmation has exactly one remedy:
answer no, and start the whole conversation again from the top.

The other end has the same shape. A carefully answered interactive run produces a
project and no record of what was asked for. To generate the same project again —
a second service with the same conventions, the same thing after a `rm -rf`, or
the same project in CI — the user reconstructs the flags by hand from the summary
box, which does not show most of them.

## What to do

**The replay is the cheap half and worth more.** Print the equivalent
non-interactive command as the last line of the summary:

```
  Generated with
    vaadin-init --yes --group-id io.binarycodes --artifact-id book-shelf \
        --database --e2e --coverage --traceable
```

Every answer is already a flag — that is a stated design property — so this is a
formatting function over the final `Config` and nothing more. It makes an
interactive run scriptable, it documents the flag names at the moment someone
first wants them, and it is the thing to paste into a project's own README to say
how it was made. Emit only what differs from the defaults, or emit everything;
prefer everything, because a defaults file that changes later should not change
what the recorded command means.

**Going back is the expensive half.** Three options, and the third is the honest
one:

1. Rebuild the second form after the first, in a loop, so "back" from its first
   field re-runs the coordinates form and then rebuilds. Possible, and the
   rebuild-on-every-back is fiddly enough to earn its own layout tests.
2. One form, with the derived defaults filled in by a hidden group's callback
   rather than at build time. This fights the framework in the place the code
   already documents as a losing fight.
3. Leave the seam, and let the confirmation carry an edit path: make "Generate?"
   a three-way — generate, start over, cancel — so the answer at the end of the
   conversation is not a dead end. Cheap, honest about what it does, and removes
   the actual annoyance, which is not "I cannot go back" but "my only way out is
   to kill the program".

Prefer (3), and do (the replay line) first regardless — it is useful on its own
and it is what makes starting over cost a paste rather than a retype.

## Test

`internal/prompt`'s accessible-mode tests already drive the whole conversation
from a scripted input, which is where a three-way confirmation would be checked.
The replay line is a pure function of a `Config` and belongs in a table test
wherever `printResult` ends up living.
