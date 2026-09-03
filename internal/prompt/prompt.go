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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

// Outcome is what there is to say once a project has been written: the same
// summary the tool leaves in the scrollback, ready to be shown inside the screen
// that asked for it.
type Outcome struct {
	Title  string
	Rows   []ui.Row
	Notice string
	Steps  []ui.Step
}

// Session is what the conversation came to.
type Session struct {
	Config config.Config

	// Written says the project was generated before the screen closed, which is
	// what makes the summary and the command bar part of the same screen rather
	// than something printed after it.
	Written bool

	// Task is the run.sh task named in the command bar, or empty for none.
	Task string
}

// Options are the ways the conversation itself can be run.
type Options struct {
	// Accessible replaces the full-screen form with plain sequential prompts
	// read line by line. Screen readers cannot follow a redrawing terminal UI,
	// so without this the tool is unusable with one — and the same mode is what
	// lets the prompt flow be driven from a script or a test.
	Accessible bool

	// AskAuthor adds the question of who the first commit is by. Asked only when
	// git could not say — the caller has checked — because a machine that knows
	// its user should not have them typed in once per project.
	AskAuthor bool

	// Banner is what the full-screen form prints above the questions. Passed in
	// rather than built here because it names the tool's own version, which this
	// package has no business knowing — and it is drawn inside the form's screen
	// because a full-screen form replaces whatever was printed before it.
	Banner string

	// Generate writes the project, and says what to show about it.
	//
	// Injected rather than called by the caller afterwards, because the screen
	// does not end when the questions do: the answers become a project, and the
	// summary and the command bar that follow are drawn in the same full-screen
	// layout the questions were asked in. Left nil — a dry run, or a screen
	// reader — the conversation ends at the last question, as it used to.
	Generate func(config.Config) (Outcome, error)

	// Task runs one of the generated project's tasks, writing everything it says
	// to out, and stops when the context is cancelled.
	//
	// Injected for the same reason as Generate, and needed for the same reason:
	// the screen does not end when the project is written. A task named in the
	// command bar runs from inside it and its output lands in the log, so the
	// tool is still the thing on the terminal — starting a task cannot be the
	// last thing it does.
	Task func(ctx context.Context, task string, out io.Writer) error

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
// Two conversations, not one. The full-screen form puts every section on one
// screen, where an answer derived from the coordinates can follow them as they
// are typed; a screen reader cannot be shown a screen, so that mode keeps asking
// one question at a time and re-derives between two forms instead.
func Run(c config.Config, lookup VersionSource, options Options) (Session, error) {
	options = options.prepared()
	if options.Accessible {
		c, err := runAccessible(c, lookup, options)
		return Session{Config: c}, err
	}
	return runScreen(c, lookup, options)
}

// runAccessible asks the questions as plain sequential prompts.
//
// The questions come in two forms rather than one because the second form's
// defaults are derived from the first form's answers: the project name, the
// package and the output directory all follow from the coordinates, and huh
// binds a field's initial value when the form is built, not when the field is
// reached.
func runAccessible(c config.Config, lookup VersionSource, options Options) (config.Config, error) {
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
	c.VaadinVersion = withFallback(available.Vaadin, c.VaadinVersion)[0]
	c.BootVersion = withFallback(available.Boot, c.BootVersion)[0]

	features := selectedFeatures(c)
	confirmed := true

	rest := restForm(&c, &features, &confirmed, available, theme, options)

	if err := rest.Run(); err != nil {
		return c, cancelled(err)
	}
	if !confirmed {
		return c, ErrCancelled
	}

	applyFeatures(&c, features)
	return c, nil
}

// Leaving reports whether what was typed into the command bar means "nothing,
// thanks" rather than the name of a task.
//
// A word, not an empty line. The bar is a prompt like any other and enter is the
// key everything else on this screen is agreed to with, so a bare enter meaning
// "we are done here" is a way to leave by accident — and the way back is to run
// the tool again and answer every question a second time.
func Leaving(task string) bool {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case "quit", "exit":
		return true
	}
	return false
}

// Task asks which of the generated project's tasks to run next.
//
// What the bar along the bottom of the screen turns into once there is a project:
// the tool has just listed the tasks, and the next thing anyone does is type one
// of them — so it is asked for here rather than left to be retyped after a `cd`.
//
// Unlike the bar, an empty answer is taken as "none, thanks" here: this is the
// version read out to a screen reader, where every question so far has been
// answered by pressing enter to accept what was offered, and answering one more
// the same way should not be the one that means something else.
func Task(options Options) (string, error) {
	options = options.prepared()

	var task string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				// Two carets, so the line reads as a command being built: the
				// tool's prompt, the script that will run, and the part left to
				// type.
				Prompt(ui.Caret + "run.sh " + ui.Caret).
				Placeholder("a task name, or quit to finish").
				Value(&task),
		),
	).WithTheme(ui.Theme()).WithShowHelp(false)

	if err := options.apply(form).Run(); err != nil {
		return "", cancelled(err)
	}
	if Leaving(task) {
		return "", nil
	}
	return strings.TrimSpace(task), nil
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
		// Nothing: where the list came from is said once, by the section, and a
		// line of prose repeated over each list is a line the lists do not get.
		return ""
	}
	return "Maven Central could not be reached — this is the built-in default, which may be out of date."
}

// versionGroup asks for the versions in accessible mode: plain inputs,
// pre-filled with the newest release.
//
// No select and no escape hatch, because huh asks a hidden group's questions
// anyway in accessible mode, so the select-plus-follow-up shape the screen uses
// would ask for each version twice. An input already offers a default to accept
// or type over, which is the whole of what the select was buying.
func versionGroup(c *config.Config, available versions.Available, options Options) *huh.Group {
	return huh.NewGroup(
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
		Description("Pinned in pom.xml, and in run.conf for the task runner.")
}

func javaVersionInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Java version").
		Description(fmt.Sprintf("Spring Boot %d needs 17 or newer.", versions.BootMajor)).
		Value(&c.JavaVersion).
		Validate(options.validator(config.ValidJavaVersion))
}

// versionOptions is a version list as a select's options, with the escape hatch
// last.
func versionOptions(list []string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(list)+1)
	for _, v := range list {
		options = append(options, huh.NewOption(v, v))
	}
	return append(options, huh.NewOption("type one myself…", custom))
}

func versionSelect(title, description string, list []string, value *string) *huh.Select[string] {
	// Value before Options, not after: Options is what scans for the bound value
	// to decide which line the cursor opens on, so a value set afterwards arrives
	// too late and the list opens wherever that scan happened to stop.
	return huh.NewSelect[string]().
		Title(title).
		Description(description).
		Value(value).
		Options(versionOptions(list)...)
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

// The questions, one constructor each, so that the full-screen form and the
// accessible one ask the same thing in the same words and cannot drift apart.

func groupIDInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Group ID").
		Description("Maven group, in reverse-DNS form.").
		Value(&c.GroupID).
		Validate(options.validator(config.ValidGroupID))
}

func artifactIDInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Artifact ID").
		Description("Maven artifact. Also names the directory and the containers.").
		Value(&c.ArtifactID).
		Validate(options.validator(config.ValidArtifactID))
}

func projectNameInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Project name").
		Description("The name that appears in the UI and in the task runner's output.").
		Value(&c.ProjectName).
		Validate(options.validator(config.ValidProjectName))
}

func descriptionInput(c *config.Config) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Description").
		Value(&c.Description)
}

func packageInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Base package").
		Description("Where the generated Java sources live.").
		Value(&c.Package).
		Validate(options.validator(config.ValidPackage))
}

// stackSelect asks which of the optional pieces to generate.
//
// The title is the caller's because it is only worth a row where the question is
// not already introduced: on the full-screen form the section above it says
// "Stack", and saying it twice costs a line of the list.
func stackSelect(c *config.Config, features *[]string, title string) *huh.MultiSelect[string] {
	rows := len(featureList)
	if title != "" {
		rows++
	}
	return huh.NewMultiSelect[string]().
		Title(title).
		// Value before Options, for the reason given in versionSelect.
		Value(features).
		Options(featureOptions(*c)...).
		// Every option, plus the row the field's title takes out of the same
		// budget. This height is the whole list's window, not a minimum: one row
		// short and the last option is only reachable by scrolling, with nothing
		// on screen to say it is there.
		//
		// The field carries no description for the same reason — a line of prose
		// here is a line the list does not get — and the help footer already says
		// "x toggle • enter confirm".
		Height(rows)
}

// The two halves of who the first commit is by. Nothing is offered unless git had
// one half already, so in accessible mode — where an empty answer normally means
// "keep what you offered me" — an empty field is only allowed through when there
// is something in it to keep.
func authorNameInput(c *config.Config, options Options, inline bool) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Name  ").
		Inline(inline).
		Value(&c.AuthorName).
		Validate(options.required(c.AuthorName, config.ValidAuthorName))
}

func authorEmailInput(c *config.Config, options Options, inline bool) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Email ").
		Inline(inline).
		Value(&c.AuthorEmail).
		Validate(options.required(c.AuthorEmail, config.ValidAuthorEmail))
}

// required is the validator for a field that may have nothing to fall back on: with
// an offered value the accessible allowance for an empty answer stands, without
// one an empty answer is refused, since huh would otherwise substitute the empty
// default and the whole Config fail validation after the last question.
func (o Options) required(offered string, validate func(string) error) func(string) error {
	if offered == "" {
		return validate
	}
	return o.validator(validate)
}

// The section that asks who the first commit is by, in the words both forms use.
const (
	authorTitle       = "Author"
	authorDescription = "Git has no identity for the first commit. Kept in this repository only; git config --global sets one everywhere."
)

// directoryInput asks where to write the project.
//
// Inline — the question and the answer on one line — because on the screen this
// has a row the width of the terminal to itself, and a row that wide spent on a
// title, a line of prose and a short path reads as three rows of nothing.
func directoryInput(c *config.Config, options Options, inline bool) *huh.Input {
	input := huh.NewInput().
		Prompt(ui.Caret).
		Title("Directory ").
		Inline(inline).
		Value(&c.OutputDir).
		Validate(options.validator(notEmpty))
	if inline {
		// The section above it says the rest. On one line, a description sits
		// between the question and the answer, which is the one place it cannot
		// be read as belonging to either.
		return input
	}
	return input.Description("Created if it does not exist. Must be empty.")
}

// typedVersionInput is the escape hatch: a version Maven Central did not offer.
//
// Bound to a field of its own rather than to the version itself, so it opens
// empty and the select's answer stays readable as the sentinel it is.
func typedVersionInput(title string, value *string) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title(title).
		Description("A version Maven Central did not offer.").
		Value(value).
		Validate(config.ValidVersion)
}

// fields are the questions the full-screen form has to reach back into once it
// is built: the coordinates everything else follows from, the answers that
// follow them, and the two lists the version lookup fills in when it lands.
type fields struct {
	projectName *huh.Input
	pkg         *huh.Input
	directory   *huh.Input
	vaadin      *huh.Select[string]
	boot        *huh.Select[string]
}

// spanning is a section with a row of its own under the columns, the width of
// the screen.
func spanning(title, description string, hide func() bool, fields ...huh.Field) section {
	s := newSection(title, description, hide, fields...)
	s.span = true
	return s
}

// sections is the whole conversation as the columns of one screen, in the order
// they are read across.
//
// The two escape hatches come last and are hidden until a version select is left
// on "type one myself", so they cost nothing on the screen until they are asked
// for — and, being last, the numbered sections in front of them keep their
// numbers when they appear.
func newSections(
	c *config.Config,
	f *fields,
	features *[]string,
	confirmed *bool,
	typed *typedVersions,
	available versions.Available,
	options Options,
) []section {
	f.projectName = projectNameInput(c, options)
	f.pkg = packageInput(c, options)
	f.directory = directoryInput(c, options, true)
	f.vaadin = versionSelect("Vaadin version", versionNote(available.Vaadin),
		withFallback(available.Vaadin, c.VaadinVersion), &c.VaadinVersion)
	f.boot = versionSelect("Spring Boot version", versionNote(available.Boot),
		withFallback(available.Boot, c.BootVersion), &c.BootVersion)

	return []section{
		newSection("Coordinates", "What this project is called to Maven.", nil,
			groupIDInput(c, options),
			artifactIDInput(c, options)),

		newSection("Identity", "What this project is called to people.", nil,
			f.projectName,
			descriptionInput(c),
			f.pkg),

		newSection("Versions", "Newest first, from Maven Central.", nil,
			f.vaadin,
			f.boot,
			javaVersionInput(c, options)),

		newSection("Stack", "The core is always generated. These are the rest.", nil,
			stackSelect(c, features, "")),

		// A row of its own above Output, and only there when git could not answer
		// for itself. Not a column: two short answers beside the tall ones would
		// be a box mostly empty, and one more column is what pushes a terminal
		// that tiled four out of tiling at all.
		spanning(authorTitle, authorDescription,
			func() bool { return !options.AskAuthor },
			authorNameInput(c, options, true),
			authorEmailInput(c, options, true)),

		// Under everything rather than beside it: where the project goes is the
		// last thing decided about it, and the button that starts the whole
		// thing follows every answer above it rather than sitting at the foot of
		// whichever column it happened to land in.
		spanning("Output", "Created if it does not exist. Must be empty.", nil,
			f.directory,
			// One button, not two. The other one is every other way out of this
			// screen — ctrl+c, or never having run it — and a No beside the
			// Generate reads as a decision that has to be made rather than the
			// one that has already been made by filling the form in.
			huh.NewConfirm().
				Affirmative("Generate").
				Negative("").
				Value(confirmed)),

		newSection("Vaadin version", "Typed, not offered.",
			func() bool { return c.VaadinVersion != custom },
			typedVersionInput("Vaadin version", &typed.vaadin)),

		newSection("Spring Boot version", "Typed, not offered.",
			func() bool { return c.BootVersion != custom },
			typedVersionInput("Spring Boot version", &typed.boot)),
	}
}

// coordinatesForm is the first of the two accessible forms: the two answers
// everything else is derived from.
//
// Built apart from being run so that the appearance can be rendered — and
// reviewed — without a terminal to run it in.
func coordinatesForm(c *config.Config, theme *huh.Theme, options Options) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			groupIDInput(c, options),
			artifactIDInput(c, options),
		).Title("Coordinates").
			Description("What this project is called to Maven."),
	)
	return options.apply(form.WithTheme(theme).WithShowHelp(true))
}

// restForm is everything the accessible conversation asks after the
// coordinates, in the order it is asked.
func restForm(
	c *config.Config,
	features *[]string,
	confirmed *bool,
	available versions.Available,
	theme *huh.Theme,
	options Options,
) *huh.Form {
	groups := []*huh.Group{
		huh.NewGroup(
			projectNameInput(c, options),
			descriptionInput(c),
			packageInput(c, options),
		).Title("Identity").
			Description("What this project is called to people."),

		versionGroup(c, available, options),

		huh.NewGroup(
			stackSelect(c, features, "Stack"),
		).Title("Stack").
			Description("The core is always generated. These are the rest."),
	}

	// Left out rather than hidden: huh asks a hidden group's questions anyway in
	// accessible mode, which would ask everyone who they are.
	if options.AskAuthor {
		groups = append(groups, huh.NewGroup(
			authorNameInput(c, options, false),
			authorEmailInput(c, options, false),
		).Title(authorTitle).
			Description(authorDescription))
	}

	groups = append(groups, huh.NewGroup(
		directoryInput(c, options, false),
		huh.NewConfirm().
			Title("Generate?").
			Value(confirmed),
	).Title("Output"))

	form := huh.NewForm(groups...)
	return options.apply(form.WithTheme(theme).WithShowHelp(true))
}
