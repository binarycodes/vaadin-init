// Package prompt is the TUI: it asks the questions and returns the answers.
//
// It owns no policy. Every value it offers arrives already decided — the defaults
// file supplies the starting point and the version lookup supplies the lists — so
// this package is only the conversation, and the same Config can be produced with
// no terminal at all by setting flags instead.
package prompt

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// ErrCancelled means the user backed out. It is not a failure, and the caller
// says so rather than printing an error.
var ErrCancelled = errors.New("cancelled")

// VersionSource hands over the looked-up releases, blocking if the lookup is
// still in flight.
//
// A function rather than a value because the lookup runs while the first
// questions are being answered: by the time the version prompts are reached the
// answer is almost always already there, and nobody has waited on the network to
// be asked their group id.
type VersionSource func() versions.Available

// custom is the sentinel a version select uses for "let me type one".
//
// Deliberately not the empty string. huh matches the bound value against the
// options to decide where to put the cursor, so a sentinel equal to a string's
// zero value is the option it lands on whenever that value is not yet set — which
// opens the list at "type one myself" with every version scrolled out of sight
// above it. It is also not a shape ValidVersion accepts, so it cannot survive to
// a pom.xml even if the resolving below were ever skipped.
const custom = "\x00type-one-myself"

// Options are the ways the conversation itself can be run.
type Options struct {
	// Accessible replaces the full-screen form with plain sequential prompts
	// read line by line. Screen readers cannot follow a redrawing terminal UI,
	// so without this the tool is unusable with one — and the same mode is what
	// lets the prompt flow be driven from a script or a test.
	Accessible bool

	// Input and Output override the terminal. Left nil they are the real one;
	// a test sets them, which is the only way to drive this flow without a
	// terminal to type into.
	Input  io.Reader
	Output io.Writer
}

// validator adapts a validation rule to how the answer arrives.
//
// In accessible mode an empty answer means "keep what you offered me", but huh
// validates the raw input before it substitutes the default — so a rule that
// rejects the empty string rejects pressing enter, and the field asks again for
// something the user has already accepted. Allowing empty through hands the
// default to huh, which then fills it in.
//
// The full-screen form needs no such allowance: the field is pre-filled, so an
// empty value there means the user cleared it on purpose and should be told.
func (o Options) validator(validate func(string) error) func(string) error {
	if !o.Accessible {
		return validate
	}
	return func(input string) error {
		if input == "" {
			return nil
		}
		return validate(input)
	}
}

// prepared settles the input the whole conversation will read from.
//
// Once, not per form: the conversation is two forms, and wrapping the same
// reader twice would give the second form a wrapper whose buffer the first one
// had already drained.
//
// The wrapping is for accessible mode only. The full-screen form needs the
// terminal itself — it reads escape sequences, not lines — so wrapping it there
// would break the very thing it is meant to help.
func (o Options) prepared() Options {
	if !o.Accessible {
		return o
	}
	input := o.Input
	if input == nil {
		input = os.Stdin
	}
	o.Input = newLineReader(input)
	return o
}

// apply puts the options onto a built form.
func (o Options) apply(form *huh.Form) *huh.Form {
	form = form.WithAccessible(o.Accessible)
	if o.Input != nil {
		form = form.WithInput(o.Input)
	}
	if o.Output != nil {
		form = form.WithOutput(o.Output)
	}
	return form
}

// lineReader hands over one line per Read.
//
// huh builds a fresh bufio.Scanner for each field it asks in accessible mode,
// and a scanner reads ahead: given a reader that returns more than one line at a
// time, the first field's scanner swallows the rest and every later field sees
// EOF and silently keeps its default. A terminal never does that — it delivers a
// line when enter is pressed — so answers piped in from a file or a script would
// behave differently from answers typed in, and only the first one would count.
//
// Reading ahead into this reader's own buffer is safe, because unlike the
// scanners it outlives the field that asked.
type lineReader struct {
	buffered *bufio.Reader
	pending  []byte
	err      error
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{buffered: bufio.NewReader(r)}
}

func (r *lineReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		line, err := r.buffered.ReadBytes('\n')
		r.err = err
		if len(line) == 0 {
			return 0, err
		}
		r.pending = line
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

// typedVersions holds a version the user typed because the lookup did not offer
// it. Kept apart from the Config until the form is done, so that "the select was
// left on the sentinel" and "here is the version" stay two separate facts.
type typedVersions struct {
	vaadin string
	boot   string
}

// resolve replaces a sentinel left by a select with the version typed in the
// group that followed it.
//
// A sentinel that somehow reaches here with nothing typed is dropped back to the
// fallback rather than carried forward: it is not a version, and Validate would
// reject it in a message about a value the user never entered.
func (t typedVersions) resolve(c *config.Config, vaadinFallback, bootFallback string) {
	if c.VaadinVersion == custom {
		c.VaadinVersion = cmp.Or(t.vaadin, vaadinFallback)
	}
	if c.BootVersion == custom {
		c.BootVersion = cmp.Or(t.boot, bootFallback)
	}
}

// Run asks the questions, seeded from c, and returns the answers.
//
// The questions come in two forms rather than one because the second form's
// defaults are derived from the first form's answers: the project name, the
// package and the output directory all follow from the coordinates, and huh
// binds a field's initial value when the form is built, not when the field is
// reached.
func Run(c config.Config, lookup VersionSource, options Options) (config.Config, error) {
	options = options.prepared()
	theme := ui.Theme()

	coordinates := coordinatesForm(&c, theme, options)

	if err := coordinates.Run(); err != nil {
		return c, cancelled(err)
	}

	// Re-derive now that the coordinates are real, so the next form opens on
	// answers that follow from them instead of on the defaults file's example.
	c.ProjectName = config.DeriveProjectName(c.ArtifactID)
	c.Package = config.DerivePackage(c.GroupID, c.ArtifactID)
	c.OutputDir = c.ArtifactID

	available := lookup()
	vaadinList := withFallback(available.Vaadin, c.VaadinVersion)
	bootList := withFallback(available.Boot, c.BootVersion)
	c.VaadinVersion = vaadinList[0]
	c.BootVersion = bootList[0]

	features := selectedFeatures(c)
	confirmed := true
	var typed typedVersions

	rest := restForm(&c, &features, &confirmed, &typed, available, vaadinList, bootList, theme, options)

	if err := rest.Run(); err != nil {
		return c, cancelled(err)
	}
	if !confirmed {
		return c, ErrCancelled
	}

	typed.resolve(&c, vaadinList[0], bootList[0])
	applyFeatures(&c, features)
	return c, nil
}

// cancelled maps huh's abort to this package's own, so the caller does not have
// to know which TUI library is behind the prompts to tell a cancel from a crash.
func cancelled(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	return err
}

func notEmpty(s string) error {
	if s == "" {
		return errors.New("must not be empty")
	}
	return nil
}

// withFallback guarantees a non-empty list. Offline, the only option is whatever
// the defaults file named — which is exactly the case the fallback exists for,
// and it still beats an empty select the user cannot get past.
func withFallback(list []string, fallback string) []string {
	if len(list) > 0 {
		return list
	}
	return []string{fallback}
}

// versionNote says where the list came from. Worth a line of screen: a default
// taken from an offline fallback may be months old, and silently offering it as
// though it were the current release is the failure this whole lookup exists to
// avoid.
func versionNote(fetched []string) string {
	if len(fetched) > 0 {
		return "Released versions, newest first, from Maven Central."
	}
	return "Maven Central could not be reached — this is the built-in default, which may be out of date."
}

// versionGroups asks for the two framework versions.
//
// A list to pick from in the normal case, with an escape hatch for a version the
// lookup did not offer: choosing "type one myself" leaves the value empty, and a
// group that is hidden unless the value is empty then asks for it.
//
// Accessible mode gets plain inputs pre-filled with the newest release instead.
// Not a simplification for its own sake: huh asks a hidden group's questions
// anyway in accessible mode, so the select-plus-follow-up shape would ask for
// each version twice. An input already offers a default to accept or type over,
// which is the whole of what the select was buying.
func versionGroups(
	c *config.Config,
	typed *typedVersions,
	available versions.Available,
	vaadinList, bootList []string,
	options Options,
) []*huh.Group {
	if options.Accessible {
		return []*huh.Group{
			huh.NewGroup(
				huh.NewInput().
					Prompt(ui.Caret).
					Title("Vaadin version").
					Description(versionNote(available.Vaadin)).
					Value(&c.VaadinVersion).
					Validate(options.validator(config.ValidVersion)),
				huh.NewInput().
					Prompt(ui.Caret).
					Title("Spring Boot version").
					Description(versionNote(available.Boot)).
					Value(&c.BootVersion).
					Validate(options.validator(config.ValidVersion)),
				javaVersionInput(c, options),
			).Title("Versions").
				Description("Pinned in pom.xml, and in run.conf for the task runner."),
		}
	}

	// A group is given one height for the whole form and its fields divide it, so
	// each list gets a group of its own. Together they left the second list
	// showing two of its options on a 24-row terminal, and a list nobody can see
	// is a list nobody can choose from.
	return []*huh.Group{
		// One question each, and all three under the same section heading: with a
		// single field per group, a group title would only repeat the field's own.
		huh.NewGroup(
			versionSelect("Vaadin version", versionNote(available.Vaadin), vaadinList, &c.VaadinVersion),
		).Title("Versions"),

		huh.NewGroup(
			versionSelect("Spring Boot version", versionNote(available.Boot), bootList, &c.BootVersion),
		).Title("Versions"),

		huh.NewGroup(
			javaVersionInput(c, options),
		).Title("Versions"),

		// Reached only when a select was left on "type one myself". Bound to its
		// own field rather than to the version itself, so it opens empty and the
		// select's answer stays readable as the sentinel it is.
		huh.NewGroup(
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Vaadin version").
				Description("A version Maven Central did not offer.").
				Value(&typed.vaadin).
				Validate(config.ValidVersion),
		).WithHideFunc(func() bool { return c.VaadinVersion != custom }),
		huh.NewGroup(
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Spring Boot version").
				Description("A version Maven Central did not offer.").
				Value(&typed.boot).
				Validate(config.ValidVersion),
		).WithHideFunc(func() bool { return c.BootVersion != custom }),
	}
}

func javaVersionInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Java version").
		Description(fmt.Sprintf("The JDK the build pins. Spring Boot %d needs 17 or newer.", versions.BootMajor)).
		Value(&c.JavaVersion).
		Validate(options.validator(config.ValidJavaVersion))
}

func versionSelect(title, description string, list []string, value *string) *huh.Select[string] {
	options := make([]huh.Option[string], 0, len(list)+1)
	for _, v := range list {
		options = append(options, huh.NewOption(v, v))
	}
	options = append(options, huh.NewOption("type one myself…", custom))

	// Value before Options, not after: Options is what scans for the bound value
	// to decide which line the cursor opens on, so a value set afterwards arrives
	// too late and the list opens wherever that scan happened to stop.
	return huh.NewSelect[string]().
		Title(title).
		Description(description).
		Value(value).
		Options(options...)
}

// The optional stack pieces, in the order they are offered. Held as data so the
// prompt, the pre-selection and the mapping back onto the Config cannot drift
// into disagreeing about what exists.
var featureList = []struct {
	key   string
	label string
	get   func(config.Config) bool
	set   func(*config.Config, bool)
}{
	{
		key:   "database",
		label: "Database — PostgreSQL, Flyway, JPA, Testcontainers, dev compose",
		get:   func(c config.Config) bool { return c.Database },
		set:   func(c *config.Config, v bool) { c.Database = v },
	},
	{
		key:   "auth",
		label: "Auth — OIDC login against Keycloak in the dev stack",
		get:   func(c config.Config) bool { return c.Auth },
		set:   func(c *config.Config, v bool) { c.Auth = v },
	},
	{
		key:   "e2e",
		label: "End-to-end tests — Playwright, behind an it profile",
		get:   func(c config.Config) bool { return c.E2E },
		set:   func(c *config.Config, v bool) { c.E2E = v },
	},
	{
		key:   "coverage",
		label: "Coverage gate — JaCoCo, 80% on service and presenter packages",
		get:   func(c config.Config) bool { return c.Coverage },
		set:   func(c *config.Config, v bool) { c.Coverage = v },
	},
	{
		key:   "traceable",
		label: "Traceable builds — every build must carry its commit SHA",
		get:   func(c config.Config) bool { return c.Traceable },
		set:   func(c *config.Config, v bool) { c.Traceable = v },
	},
}

// featureOptions is the stack list, with everything already on listed first.
//
// The order is not presentation for its own sake. huh opens a multi-select with
// both the cursor and the viewport on the first *selected* option, so a list whose
// first entries are off opens already scrolled past them — and the options nobody
// can see are exactly the ones nobody thought to turn on. Selected first means
// the first option is always selected whenever any is, which pins the viewport to
// the top.
//
// A stable partition, so the order within each half is still the order declared
// in featureList, and the list reads as "on, then off" rather than as shuffled.
func featureOptions(c config.Config) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(featureList))
	for _, wanted := range []bool{true, false} {
		for _, f := range featureList {
			if f.get(c) == wanted {
				options = append(options, huh.NewOption(f.label, f.key).Selected(wanted))
			}
		}
	}
	return options
}

func selectedFeatures(c config.Config) []string {
	var keys []string
	for _, f := range featureList {
		if f.get(c) {
			keys = append(keys, f.key)
		}
	}
	return keys
}

// applyFeatures writes the multi-select's answer back. Every feature is set from
// the selection, the absent ones to false: the answer is the complete list of
// what is wanted, so anything missing from it was deselected.
func applyFeatures(c *config.Config, keys []string) {
	chosen := make(map[string]bool, len(keys))
	for _, key := range keys {
		chosen[key] = true
	}
	for _, f := range featureList {
		f.set(c, chosen[f.key])
	}
}

// coordinatesForm is the first form: the two answers everything else is derived
// from.
//
// Built apart from being run so that the appearance can be rendered — and
// reviewed — without a terminal to run it in.
func coordinatesForm(c *config.Config, theme *huh.Theme, options Options) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Group ID").
				Description("Maven group, in reverse-DNS form.").
				Value(&c.GroupID).
				Validate(options.validator(config.ValidGroupID)),
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Artifact ID").
				Description("Maven artifact. Also names the directory and the containers.").
				Value(&c.ArtifactID).
				Validate(options.validator(config.ValidArtifactID)),
		).Title("Coordinates").
			Description("What this project is called to Maven."),
	)
	return options.apply(form.WithTheme(theme).WithShowHelp(true))
}

// restForm is everything after the coordinates, in the order it is asked.
func restForm(
	c *config.Config,
	features *[]string,
	confirmed *bool,
	typed *typedVersions,
	available versions.Available,
	vaadinList, bootList []string,
	theme *huh.Theme,
	options Options,
) *huh.Form {
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Project name").
				Description("The name that appears in the UI and in the task runner's output.").
				Value(&c.ProjectName).
				Validate(options.validator(config.ValidProjectName)),
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Description").
				Value(&c.Description),
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Base package").
				Description("Where the generated Java sources live.").
				Value(&c.Package).
				Validate(options.validator(config.ValidPackage)),
		).Title("Identity").
			Description("What this project is called to people."),
	}

	groups = append(groups, versionGroups(c, typed, available, vaadinList, bootList, options)...)

	groups = append(groups,
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Stack").
				// Value before Options, for the reason given in versionSelect.
				Value(features).
				Options(featureOptions(*c)...).
				// Every option, plus the row the field's title takes out of the
				// same budget. This height is the whole list's window, not a
				// minimum: one row short and the last option is only reachable by
				// scrolling, with nothing on screen to say it is there.
				//
				// The field carries no description for the same reason — a line of
				// prose here is a line the list does not get — and the help footer
				// already says "x toggle • enter confirm".
				Height(len(featureList)+1),
		).Title("Stack").
			Description("The core is always generated. These are the rest."),

		huh.NewGroup(
			huh.NewInput().
				Prompt(ui.Caret).
				Title("Directory").
				Description("Created if it does not exist. Must be empty.").
				Value(&c.OutputDir).
				Validate(options.validator(notEmpty)),
			huh.NewConfirm().
				Title("Generate?").
				Value(confirmed),
		).Title("Output"),
	)

	form := huh.NewForm(groups...)
	return options.apply(form.WithTheme(theme).WithShowHelp(true))
}
