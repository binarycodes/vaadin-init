// Package prompt is the TUI: it asks the questions and returns the answers.
//
// It owns no policy. Every value it offers arrives already decided — the defaults
// file supplies the starting point and the version lookup supplies the lists — so
// this package is only the conversation, and the same Config can be produced with
// no terminal at all by setting flags instead.
package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/binarycodes/vaadin-init/internal/compat"
	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/version"
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

// PinSource is the Spring Boot release a Vaadin release was built with, or ""
// when that is not known. It may block on the network, so the screen asks it
// from a command and the accessible conversation asks it once, between forms.
type PinSource func(vaadin string) string

// How many releases to offer in a version list. Enough to pick the previous patch
// or an older minor deliberately, few enough that the list fits a short terminal
// without scrolling — anyone who wants an older release than these can type it.
const offered = 5

// offer is the head of a list, newest first, as the select shows it.
func offer(list []string) []string {
	if len(list) > offered {
		return list[:offered]
	}
	return list
}

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

	// Rules say which Spring Boot and which JDK go with the chosen Vaadin: what
	// the Boot list is filtered to, what the Java list holds, and what an answer
	// typed in accessible mode is checked against. The rules come from a file,
	// not from here.
	Rules compat.Rules

	// Pinned is where the Boot list opens: the release the chosen Vaadin was
	// built with. Left nil, the newest compatible release is offered instead.
	Pinned PinSource

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

	// Derived in order, each from the one before, the way the screen derives
	// them as they are chosen: the newest Vaadin, the Boot it was built with,
	// a Java both allow, and a theme the Vaadin ships.
	available := lookup()
	c.VaadinVersion = withFallback(offer(available.Vaadin), c.VaadinVersion)[0]
	_, c.BootVersion = bootChoices(options.Rules, c.VaadinVersion, available.Boot,
		options.pinned(c.VaadinVersion), c.BootVersion)
	if _, open, _ := javaChoices(options.Rules, c.VaadinVersion, c.BootVersion, c.JavaVersion); open != "" {
		c.JavaVersion = open
	}
	if theme := options.Rules.ThemeDefault(c.VaadinVersion, c.Theme); theme != "" {
		c.Theme = theme
	}

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

// pinned asks the pin source, if there is one.
func (o Options) pinned(vaadin string) string {
	if o.Pinned == nil {
		return ""
	}
	return o.Pinned(vaadin)
}

// versionGroup asks for the versions in accessible mode: plain inputs,
// pre-filled with the derived answers.
//
// Inputs rather than selects: an input offers a default to accept or type over,
// which is the whole of what a list read out one number at a time would buy, and
// it is the one place a release the lookup did not offer can still be named.
//
// The validators read the earlier answers when they run, not when the form is
// built, so a Boot typed over the default is still checked against the Vaadin
// typed just before it.
func versionGroup(c *config.Config, available versions.Available, options Options) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Prompt(ui.Caret).
			Title("Vaadin version").
			Description(versionNote(available.Vaadin)).
			Value(&c.VaadinVersion).
			Validate(options.validator(vaadinValidator(options.Rules))),
		huh.NewInput().
			Prompt(ui.Caret).
			Title("Spring Boot version").
			Description(versionNote(available.Boot)).
			Value(&c.BootVersion).
			Validate(options.validator(bootValidator(options.Rules, c))),
		huh.NewInput().
			Prompt(ui.Caret).
			Title("Java version").
			Description(options.Rules.Describe(c.VaadinVersion, c.BootVersion)).
			Value(&c.JavaVersion).
			Validate(options.validator(javaValidator(options.Rules, c))),
	).Title("Versions").
		Description("Pinned in pom.xml, and in run.conf for the task runner.")
}

// The validators for a version typed in accessible mode: the shape, and then the
// rules. A version from a line this tool does not generate is refused at the
// field, where the answer can still be changed, rather than after the last
// question.

func vaadinValidator(rules compat.Rules) func(string) error {
	return func(s string) error {
		if err := config.ValidVersion(s); err != nil {
			return err
		}
		_, err := rules.Vaadin(s)
		return err
	}
}

func bootValidator(rules compat.Rules, c *config.Config) func(string) error {
	return func(s string) error {
		if err := config.ValidVersion(s); err != nil {
			return err
		}
		return rules.CheckBoot(c.VaadinVersion, s)
	}
}

func javaValidator(rules compat.Rules, c *config.Config) func(string) error {
	return func(s string) error {
		if err := config.ValidJavaVersion(s); err != nil {
			return err
		}
		java, _ := strconv.Atoi(s)
		return rules.CheckJava(c.VaadinVersion, c.BootVersion, java)
	}
}

// bootChoices is what the Boot question offers for a Vaadin version, and where
// it opens: the compatible releases, newest first, with the release the Vaadin
// was built with among them whether or not it is one of the newest. Offline —
// nothing fetched and no pin — the only option is the fallback, which is what the
// defaults file named.
func bootChoices(rules compat.Rules, vaadin string, fetched []string, pinned, fallback string) (list []string, open string) {
	open = rules.BootDefault(vaadin, pinned, fetched)
	if open == "" {
		return []string{fallback}, fallback
	}
	list = offer(rules.CompatibleBoot(vaadin, fetched))
	if !slices.Contains(list, open) {
		list = append(list, open)
		slices.SortFunc(list, func(a, b string) int { return version.Compare(b, a) })
	}
	return list, open
}

// javaChoices is what the Java question offers for a Vaadin and Boot pair, where
// it opens, and the line under it saying why. The list is empty when the pair
// itself is refused, and the note then says so.
//
// The long-term-support releases in the range and the newest release in it, not
// every major: a feature release older than the newest is out of support the day
// the next one ships, so a list of them is a list of JDKs nobody should start a
// project on — and a list of six is what pushes the Versions column past a
// terminal that tiled. --java-version names the rest, and the rules accept them.
//
// The cursor opens on the preferred release — the defaults file's — when the
// range allows it, and on the newest LTS in the range when it does not, with the
// note saying what was given up.
func javaChoices(rules compat.Rules, vaadin, boot, preferred string) (list []string, open, note string) {
	floor, ceiling, err := rules.JavaRange(vaadin, boot)
	if err != nil {
		return nil, "", firstLine(err.Error())
	}
	// No ceiling written down for this Boot line: offer up to the newest JDK any
	// line in the rules supports.
	if ceiling == 0 {
		ceiling = max(floor, rules.NewestJava())
	}
	want, _ := strconv.Atoi(preferred)
	chosen, snapped := rules.JavaDefault(floor, ceiling, want)
	for java := floor; java <= ceiling; java++ {
		if rules.LTS(java) || java == ceiling || java == chosen {
			list = append(list, strconv.Itoa(java))
		}
	}
	open = strconv.Itoa(chosen)
	if !slices.Contains(list, open) {
		list = append(list, open)
	}
	note = rules.Describe(vaadin, boot)
	if snapped {
		note += fmt.Sprintf(" The default, %s, is outside that.", preferred)
	}
	return list, open, note
}

// firstLine is an error's message without the source cited under it, for a
// description a column wide.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// versionOptions is a version list as a select's options.
func versionOptions(list []string) []huh.Option[string] {
	return labelled(list, func(v string) string { return v })
}

// bootOptions is the Boot list with the release the Vaadin was built with saying
// so, since that is the one reason to pick it over a newer one.
func bootOptions(list []string, pinned, vaadin string) []huh.Option[string] {
	return labelled(list, func(v string) string {
		// Without "Vaadin" before the release: the list sits under the Vaadin
		// list, and the longer label wraps in a column, which puts the list's
		// rows and the viewport's out of step.
		if v == pinned {
			return v + " · built with " + vaadin
		}
		return v
	})
}

// javaOptions is the Java list with the long-term-support releases marked, since
// that is the one reason to pick an older one.
func javaOptions(rules compat.Rules, list []string) []huh.Option[string] {
	return labelled(list, func(v string) string {
		if n, _ := strconv.Atoi(v); rules.LTS(n) {
			return v + " · LTS"
		}
		return v
	})
}

func labelled(list []string, label func(string) string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(list))
	for _, v := range list {
		options = append(options, huh.NewOption(label(v), v))
	}
	return options
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
		Value(&c.GroupID).
		Validate(options.validator(config.ValidGroupID))
}

func artifactIDInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Artifact ID").
		Value(&c.ArtifactID).
		Validate(options.validator(config.ValidArtifactID))
}

func projectNameInput(c *config.Config, options Options) *huh.Input {
	return huh.NewInput().
		Prompt(ui.Caret).
		Title("Project name").
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
		Value(&c.Package).
		Validate(options.validator(config.ValidPackage))
}

// stackSelect asks which of the optional pieces to generate.
//
// Titled, although the section already says what it is about: the theme select
// above it in the same section has a title, and a list with none under a select
// with one reads as more of the theme's options.
func stackSelect(c *config.Config, features *[]string) *huh.MultiSelect[string] {
	return huh.NewMultiSelect[string]().
		Title("Features").
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
		Height(len(featureList) + 1)
}

// The section that asks what goes in the project: the theme, and the optional
// pieces. In the words both forms use.
const (
	stackTitle       = "Stack"
	stackDescription = "The core is always generated. Choose its theme, and the rest."
)

// themeOptions are the themes a Vaadin version ships, in the order the rules
// list them — the line's default first — named for a person. A version with no
// rule offers whatever the Config holds, so the select is never empty.
func themeOptions(rules compat.Rules, vaadin string, fallback string) []huh.Option[string] {
	return labelled(withFallback(rules.Themes(vaadin), fallback), func(theme string) string {
		return config.Config{Theme: theme}.ThemeName()
	})
}

// themeSelect asks which of the chosen Vaadin's themes the project loads.
//
// In the Stack section rather than one of its own: two options do not fill a
// column, and a sixth column is what takes the width from the other five that
// pushes a terminal which tiled out of tiling.
//
// Validated against the Vaadin as answered: in accessible mode the list is built
// before the Vaadin is typed, and a theme the typed line does not ship has to be
// refused at the field rather than after the last question.
func themeSelect(c *config.Config, options Options) *huh.Select[string] {
	return huh.NewSelect[string]().
		Title("Theme").
		Description(themeNote(options.Rules, c.VaadinVersion)).
		// Value before Options, for the reason given in versionSelect.
		Value(&c.Theme).
		Options(themeOptions(options.Rules, c.VaadinVersion, c.Theme)...).
		Validate(func(theme string) error { return options.Rules.CheckTheme(c.VaadinVersion, theme) })
}

// themeNote says which theme the chosen Vaadin's line defaults to: "Aura is the
// Vaadin 25 default." One line at the narrowest column that tiles. The utility
// classes that come with Lumo are said where they are loaded, in the generated
// code. Nothing for a Vaadin the rules do not know.
func themeNote(rules compat.Rules, vaadin string) string {
	rule, err := rules.Vaadin(vaadin)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s is the Vaadin %s default.", config.Config{Theme: rule.Themes[0]}.ThemeName(), rule.Line)
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

// fields are the questions the full-screen form has to reach back into once it
// is built: the coordinates everything else follows from, the answers that
// follow them, and the four lists that follow the lookup and each other.
type fields struct {
	projectName *huh.Input
	pkg         *huh.Input
	directory   *huh.Input
	vaadin      *huh.Select[string]
	boot        *huh.Select[string]
	java        *huh.Select[string]
	theme       *huh.Select[string]
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
func newSections(
	c *config.Config,
	f *fields,
	features *[]string,
	confirmed *bool,
	available versions.Available,
	options Options,
) []section {
	f.projectName = projectNameInput(c, options)
	f.pkg = packageInput(c, options)
	f.directory = directoryInput(c, options, true)
	f.vaadin = versionSelect("Vaadin version", versionNote(available.Vaadin),
		withFallback(offer(available.Vaadin), c.VaadinVersion), &c.VaadinVersion)

	// Built from whatever the Config holds now — the defaults, until the lookup
	// lands — and re-derived by the screen as the answers above them change.
	pinned := options.pinned(c.VaadinVersion)
	bootList, bootOpen := bootChoices(options.Rules, c.VaadinVersion, available.Boot, pinned, c.BootVersion)
	c.BootVersion = bootOpen
	f.boot = huh.NewSelect[string]().
		Title("Spring Boot version").
		Description(versionNote(available.Boot)).
		Value(&c.BootVersion).
		Options(bootOptions(bootList, pinned, c.VaadinVersion)...)

	javaList, javaOpen, javaNote := javaChoices(options.Rules, c.VaadinVersion, c.BootVersion, c.JavaVersion)
	if javaOpen != "" {
		c.JavaVersion = javaOpen
	}
	f.java = huh.NewSelect[string]().
		Title("Java version").
		Description(javaNote).
		Value(&c.JavaVersion).
		Options(javaOptions(options.Rules, withFallback(javaList, c.JavaVersion))...)

	if theme := options.Rules.ThemeDefault(c.VaadinVersion, c.Theme); theme != "" {
		c.Theme = theme
	}
	f.theme = themeSelect(c, options)

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
			f.java),

		newSection(stackTitle, stackDescription, nil,
			f.theme,
			stackSelect(c, features)),

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
			themeSelect(c, options),
			stackSelect(c, features),
		).Title(stackTitle).
			Description(stackDescription),
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
