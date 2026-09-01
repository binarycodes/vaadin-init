# Offer a color scheme, once the theme is a choice

## Where it stands

Nothing generated says anything about light or dark. The application takes
whatever the theme's default is — light, for both Aura and Lumo — and a user
whose operating system is set to dark gets a light application with no hint in
the project about where that was decided.

This only becomes worth acting on after `01-theme-choice-lumo-or-aura.md`, because
the mechanism differs by theme and hard-coding one of them is what that plan is
removing.

## What to do

Aura exposes the scheme through the standard CSS `color-scheme` property, with
`@ColorScheme` and `Page::setColorScheme()` as the Flow-side API, and computes
every text, border and surface color for the active scheme. So the whole feature,
for the generated project, is one line in a stylesheet or one annotation:

```css
html {
  /* Follow the operating system's preference */
  color-scheme: light dark;
}
```

Three defensible options, in order of increasing cost:

1. **Document it, generate nothing.** A short paragraph in the generated README's
   theme section saying which property to set and what the three values mean.
   Costs one paragraph, and is a real improvement over the current silence.
2. **Generate the `light dark` line, commented out**, in the entry stylesheet's
   partial, next to the paragraph. The comment is where the reader is already
   looking when they ask the question.
3. **Ask.** A third option on the theme select, or a select of its own —
   light / dark / follow the system.

Prefer (2). The answer is one line, it is not a decision the tool is better placed
to make than the user, and a fourth screen in the conversation for one CSS
property is a poor trade against the tool's eight-question promise.

Whichever is chosen, do not put a `color-scheme` rule in the entry stylesheet
itself: `templates/styles.css` is `@import` statements only, and that rule is
load-bearing for the browser caching that `./run.sh styles` works around.

## Note for Lumo

Lumo's dark variant is not the `color-scheme` property — it is a theme variant
applied to the document. If the generated README grows a paragraph about this, it
has to be rendered per theme rather than written once, or it will be wrong for
one of the two.
