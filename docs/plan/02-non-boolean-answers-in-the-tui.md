# Make room for answers that are not yes-or-no

## Where it stands

Every optional decision the tool takes is a boolean, and that assumption is baked
into four places at once:

- `defaults.toml` has a `[features]` table of five booleans, decoded by the nested
  `Features` struct in `internal/config/defaults.go`;
- `internal/prompt/prompt.go` holds `featureList` — key, label, getter, setter —
  and turns it into one multi-select;
- `main.go` defines one `flags.Bool` per feature;
- `internal/config/config.go` has one `bool` field per feature and lists them
  again in `Selected()`.

Adding a boolean means five edits in agreement. Adding anything else — the theme
choice, a color scheme, a build tool, a database other than PostgreSQL — has no
place to go at all: the multi-select cannot express it, `[features]` cannot type
it, and `Selected()` cannot name it.

The theme choice (`01-theme-choice-lumo-or-aura.md`) is the first answer that does
not fit, and it can be threaded through by hand. The second one is where this
starts to hurt.

## What to do

Not a framework. Two narrow changes that keep the current shape honest:

1. **Give non-boolean answers a home in the defaults file** — top-level keys, as
   the versions already are, rather than a `[choices]` table that would only
   invite a second decoder. `theme = "aura"` is the first.

2. **Split the prompt's `featureList` from the rest of the conversation.** It is
   the multi-select's data and should be named as such (`stackFeatures` or the
   like), so a reader does not read it as "the list of everything optional" and
   file the next choice into it because it looked like the register.

Then, when a third non-boolean answer arrives, the question of whether the four
parallel lists want unifying can be asked with two examples in hand instead of
none. Doing it now, on one example, would produce an abstraction shaped entirely
by the theme flag.

## What not to do

Do not move the version answers or the coordinates into a generic table. They are
validated individually, derived from one another, and asked in a specific order
for reasons `internal/prompt/prompt.go` documents at length. A generic mechanism
would have to grow back every one of those exceptions.

## Test

`internal/config`'s `TestDefaultsProduceAValidConfig` already guards the
round-trip from the embedded TOML to a valid `Config`. Extend it as each
non-boolean key lands, so that a key added to `defaults.toml` and forgotten in
`ToConfig` fails a test rather than silently defaulting to the zero value.
