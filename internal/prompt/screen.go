package prompt

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/binarycodes/vaadin-init/internal/compat"
	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// runScreen asks everything on one full-screen page — and, once it has been
// agreed to, writes the project and shows what it wrote on that same page.
func runScreen(c config.Config, lookup VersionSource, options Options) (Session, error) {
	s := newScreen(&c, lookup, options)

	if _, err := tea.NewProgram(s, s.programOptions()...).Run(); err != nil {
		return Session{Config: c}, err
	}
	if s.failure != nil {
		return Session{Config: c}, s.failure
	}
	if s.form.State == huh.StateAborted || !s.confirmed {
		return Session{Config: c}, ErrCancelled
	}

	applyFeatures(&c, s.features)
	return Session{Config: c, Written: s.phase == written}, nil
}

// screen is the full-screen conversation.
//
// A model of this package's own around huh's form, rather than the form's own
// Run, because three things have to happen that a form on its own does not do:
// the whole of it has to be laid out at once and dropped back to one section at
// a time when the terminal is too small for that; a key has to jump straight to
// a section, since a screen showing every answer invites correcting the one in
// the third column; and the answers derived from the coordinates have to follow
// them as they are typed, which the old two-form split did by rebuilding the
// second form and this screen cannot, because both are already on it.
type screen struct {
	form   *huh.Form
	tiled  *tiled
	fields *fields
	c      *config.Config

	features  []string
	confirmed bool

	// What the Boot and Java lists are derived from. The Boot list follows the
	// Vaadin answer and the Java list follows both, the way the derived answers
	// follow the coordinates — so each is re-derived when what it follows
	// changes, and left alone otherwise.
	rules     compat.Rules
	available versions.Available
	from      struct{ vaadin, boot string }

	// The Java and the theme the defaults file named, which is where those lists
	// open whenever the chosen Vaadin allows it.
	preferredJava  string
	preferredTheme string

	// The Boot the defaults file named: the only Boot to offer when nothing was
	// fetched and no pin is known.
	fallbackBoot string

	// The Boot each Vaadin release was built with, as the pin source answers —
	// "" for not known — keyed by Vaadin version, so each is asked for once.
	pin  PinSource
	pins map[string]string

	// What the screen is doing: asking the questions, writing what they came to,
	// or showing what was written.
	phase    phase
	generate func(config.Config) (Outcome, error)
	outcome  Outcome
	failure  error

	// The command bar the screen turns into once there is a project, and the log
	// underneath it that everything a task says goes into.
	command textinput.Model
	leaving bool

	runner  func(context.Context, string, io.Writer) error
	running bool
	current string
	stopped bool
	cancel  context.CancelFunc
	lines   chan tea.Msg
	log     []string

	lookup VersionSource
	banner string
	input  io.Reader
	output io.Writer

	// What the coordinates last derived. A derived answer is replaced only while
	// it still holds what was derived for it: once the user has typed their own
	// project name, editing the artifact id must not take it away again.
	derived struct {
		projectName string
		pkg         string
		directory   string
	}

	// Which fields the cursor has been in. The version lists arrive after the
	// form is built, and replacing the options under someone who is choosing
	// from them would move their selection.
	visited map[huh.Field]bool

	width, height int
	columns       bool // whether the sections are currently tiled
	shown         int  // how many sections were on screen when widths were set
}

// phase is which of the screen's three states it is in. They are one screen and
// not three: the banner, the boxes and the bar stay where they are, and only what
// is inside them changes — a project is the answer to the questions, not a
// different program.
type phase int

const (
	asking phase = iota
	writing
	written
)

// writtenMsg carries back what generating the project came to.
type writtenMsg struct {
	outcome Outcome
	err     error
}

// versionsMsg carries the release lists in from the lookup.
//
// The lookup is waited on inside the program rather than before it, because
// waiting before it means a blank terminal for as long as Maven Central takes.
// The first sections are answerable without it, which is the whole reason the
// lookup was started early in the first place.
type versionsMsg versions.Available

// pinMsg carries in the Boot a Vaadin release was built with. Asked for when the
// Vaadin answer is known, and for the same reason not waited on.
type pinMsg struct{ vaadin, boot string }

func newScreen(c *config.Config, lookup VersionSource, options Options) *screen {
	s := &screen{
		c:              c,
		features:       selectedFeatures(*c),
		confirmed:      true,
		preferredJava:  c.JavaVersion,
		preferredTheme: c.Theme,
		fallbackBoot:   c.BootVersion,
		rules:          options.Rules,
		pin:            options.Pinned,
		pins:           map[string]string{},
		lookup:         lookup,
		// Trimmed of the blank lines it is printed with: in a screen of its own
		// the banner is the top of the page, and a blank row above it is a row
		// the questions do not get.
		banner:   strings.Trim(options.Banner, "\n"),
		input:    options.Input,
		output:   options.Output,
		visited:  map[huh.Field]bool{},
		fields:   &fields{},
		generate: options.Generate,
		runner:   options.Task,
		command:  ui.CommandInput(),
	}
	s.derived.projectName = c.ProjectName
	s.derived.pkg = c.Package
	s.derived.directory = c.OutputDir

	// Built with no releases yet: the lists start as whatever the defaults file
	// named, and are replaced when the lookup lands. The pin is not asked for
	// here either — newSections is given no pin source — because a blocking call
	// before the screen is up is exactly the wait the lookup was moved off of.
	options.Pinned = nil
	sections := newSections(c, s.fields, &s.features, &s.confirmed,
		versions.Available{}, options)
	s.from.vaadin, s.from.boot = c.VaadinVersion, c.BootVersion

	s.tiled = &tiled{sections: sections}

	groups := make([]*huh.Group, 0, len(sections))
	for _, section := range sections {
		groups = append(groups, section.group)
	}

	s.form = huh.NewForm(groups...).
		WithTheme(ui.Theme()).
		WithKeyMap(keys()).
		// The help is drawn by this screen, in the bar along the bottom, rather
		// than by each group under itself.
		WithShowHelp(false).
		WithLayout(s.tiled)
	return s
}

// keys is huh's own keymap with the confirm's second button taken out of it.
//
// The Output section's confirm has one button, so there is nothing to toggle
// between and no "no" to answer: left in, the bar would offer keys that do
// nothing, which is worse than offering none.
func keys() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	keymap.Confirm.Toggle.SetEnabled(false)
	keymap.Confirm.Accept.SetEnabled(false)
	keymap.Confirm.Reject.SetEnabled(false)
	return keymap
}

func (s *screen) programOptions() []tea.ProgramOption {
	// The alt screen is what makes this one page rather than a form scrolling
	// past: the sections keep their place while they are answered, and the
	// terminal the user started from is handed back untouched when it exits.
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if s.input != nil {
		options = append(options, tea.WithInput(s.input))
	}
	if s.output != nil {
		options = append(options, tea.WithOutput(s.output))
	}
	return options
}

func (s *screen) Init() tea.Cmd {
	return tea.Batch(s.form.Init(), func() tea.Msg { return versionsMsg(s.lookup()) })
}

func (s *screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return s, s.relayout()

	case versionsMsg:
		cmd := s.offer(versions.Available(msg))
		return s, tea.Batch(cmd, s.relayout())

	case pinMsg:
		s.pins[msg.vaadin] = msg.boot
		if msg.vaadin != s.c.VaadinVersion {
			return s, nil
		}
		s.deriveBoot()
		s.deriveJava()
		return s, s.relayout()

	case writtenMsg:
		if msg.err != nil {
			s.failure = msg.err
			return s, tea.Quit
		}
		s.outcome = msg.outcome
		s.phase = written
		return s, s.command.Focus()

	case logLine, ranMsg, []tea.Msg:
		if s.logged(msg) {
			return s, s.await()
		}
		return s, nil

	case tea.KeyMsg:
		if s.phase != asking {
			return s, s.typing(msg)
		}
		if n, ok := sectionKey(msg.String()); ok {
			return s, s.jump(n)
		}
	}

	if s.phase != asking {
		var cmd tea.Cmd
		s.command, cmd = s.command.Update(msg)
		return s, cmd
	}

	cmd := s.pass(msg)
	return s, cmd
}

// pass hands a message to the form and then puts right everything that
// depends on what it did.
func (s *screen) pass(msg tea.Msg) tea.Cmd {
	_, cmd := s.form.Update(msg)

	if field := s.form.GetFocusedField(); field != nil {
		s.visited[field] = true
	}
	s.follow()
	cmd = tea.Batch(cmd, s.derive())

	if s.form.State != huh.StateNormal {
		return s.finish()
	}
	return cmd
}

// finish is what happens when the last question has been answered.
//
// Where the tool used to hand back the terminal and print its summary into
// whatever was behind it, it now writes the project and stays: the screen it was
// asked for on is the screen it is reported on, and the screen the project is
// then run from.
func (s *screen) finish() tea.Cmd {
	if s.generate == nil || s.form.State != huh.StateCompleted || !s.confirmed {
		return tea.Quit
	}

	s.phase = writing

	// The answers are only complete once the stack multi-select is written back.
	c := *s.c
	applyFeatures(&c, s.features)

	return func() tea.Msg {
		outcome, err := s.generate(c)
		return writtenMsg{outcome: outcome, err: err}
	}
}

// typing is the command bar taking a key.
//
// While a task is running the bar is not taking anything: the keys belong to the
// task, and the only one that means something here is the one that stops it.
func (s *screen) typing(msg tea.KeyMsg) tea.Cmd {
	if s.phase != written {
		return nil
	}

	if s.running {
		if msg.Type == tea.KeyCtrlC {
			s.stop()
		}
		return nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		typed := strings.TrimSpace(s.command.Value())
		// An empty bar is someone who has not decided yet, not someone who is
		// done: leaving is a word, and it is the word on the bar.
		if typed == "" {
			return nil
		}
		if Leaving(typed) {
			s.leaving = true
			return tea.Quit
		}
		return s.start(typed)

	// Handled here because the form's own keymap went out of scope with the
	// questions, and a bar nobody can leave is a terminal nobody gets back.
	case tea.KeyEsc, tea.KeyCtrlC:
		s.leaving = true
		return tea.Quit
	}

	var cmd tea.Cmd
	s.command, cmd = s.command.Update(msg)
	return cmd
}

// follow keeps the derived answers following the coordinates.
func (s *screen) follow() {
	s.track(s.fields.projectName, &s.c.ProjectName, &s.derived.projectName,
		config.DeriveProjectName(s.c.ArtifactID))
	s.track(s.fields.pkg, &s.c.Package, &s.derived.pkg,
		config.DerivePackage(s.c.GroupID, s.c.ArtifactID))
	s.track(s.fields.directory, &s.c.OutputDir, &s.derived.directory,
		s.c.ArtifactID)
}

// track replaces what a field is showing, but only while it is still showing
// what was derived for it.
//
// Re-binding the value is how the text on screen is replaced: huh reads the
// bound value when the field is built and not again, so setting the Config
// alone would change what is generated without changing what is being read.
//
// Bound twice, through an empty string first, to put the cursor after the text.
// The text input keeps its cursor where it was when its value is replaced, unless
// the value was empty; and it was empty once, when the artifact id was cleared,
// so the cursor lands after the first letter derived and stays there while the
// rest is typed. A field reached with its cursor in the second column is one
// where ctrl+u deletes one letter and typing lands in the middle of the old
// answer. huh offers no way to move the cursor, but an empty value is the case
// it moves it for.
func (s *screen) track(field *huh.Input, bound, derived *string, next string) {
	if *bound != *derived || next == *derived {
		return
	}
	*bound, *derived = next, next
	empty := ""
	field.Value(&empty)
	field.Value(bound)
}

// offer replaces the Vaadin list with what the lookup found, and then derives
// the two lists that follow it.
func (s *screen) offer(available versions.Available) tea.Cmd {
	s.available = available
	vaadin := offer(available.Vaadin)

	if len(vaadin) > 0 && !s.visited[s.fields.vaadin] {
		// The newest release is the answer being offered, so it is the one the
		// cursor has to open on — and huh takes the cursor from the bound value.
		s.c.VaadinVersion = vaadin[0]
		s.fields.vaadin.Options(versionOptions(vaadin)...)
		s.fields.vaadin.Description(versionNote(vaadin))
	}
	// Derived even when the Vaadin answer did not move: the Boot list is now
	// filtered from what was fetched rather than from the one fallback.
	s.from.vaadin = ""
	return s.derive()
}

// derive re-derives what follows an answer that has changed: the Boot and theme
// lists when the Vaadin answer has, the Java list when either version above it
// has. Nothing follows the Java or the theme, and nothing on the screen can be
// chosen from a list that was not derived for the answers above it. It also asks
// for the pin of a Vaadin release not asked about before.
func (s *screen) derive() tea.Cmd {
	var cmd tea.Cmd
	if s.c.VaadinVersion != s.from.vaadin {
		s.from.vaadin = s.c.VaadinVersion
		cmd = s.fetchPin(s.c.VaadinVersion)
		s.deriveBoot()
		s.deriveTheme()
	}
	if s.c.BootVersion != s.from.boot {
		s.from.boot = s.c.BootVersion
		s.deriveJava()
	}
	return cmd
}

// fetchPin asks the pin source, once per Vaadin release, from a command so the
// screen never waits on it.
func (s *screen) fetchPin(vaadin string) tea.Cmd {
	if s.pin == nil {
		return nil
	}
	if _, asked := s.pins[vaadin]; asked {
		return nil
	}
	s.pins[vaadin] = ""
	return func() tea.Msg { return pinMsg{vaadin, s.pin(vaadin)} }
}

// deriveBoot rebuilds the Boot list for the Vaadin answer.
//
// A choice already made in the list is kept if the new list still has it, and
// the cursor is moved to where the list opens otherwise: the pinned release, or
// the newest the rules allow.
func (s *screen) deriveBoot() {
	vaadin := s.c.VaadinVersion
	pinned := s.pins[vaadin]
	list, open := bootChoices(s.rules, vaadin, s.available.Boot, pinned, s.fallbackBoot)
	s.keep(s.fields.boot, &s.c.BootVersion, list, open)
	relist(s.fields.boot, &s.c.BootVersion, bootOptions(list, pinned, vaadin))
	s.fields.boot.Description(versionNote(s.available.Boot))
}

// deriveJava rebuilds the Java list for the Vaadin and Boot answers, and the line
// under it saying what bounds it. A pair the rules refuse — possible only when
// the Boot is the offline fallback — leaves the list as it was and says why.
func (s *screen) deriveJava() {
	vaadin, boot := s.c.VaadinVersion, s.c.BootVersion
	list, open, note := javaChoices(s.rules, vaadin, boot, s.preferredJava)
	s.fields.java.Description(note)
	if len(list) == 0 {
		return
	}
	s.keep(s.fields.java, &s.c.JavaVersion, list, open)
	relist(s.fields.java, &s.c.JavaVersion, javaOptions(s.rules, list))
}

// deriveTheme rebuilds the theme list for the Vaadin answer: the themes its line
// ships, opening on the defaults file's theme when that is one of them.
func (s *screen) deriveTheme() {
	list := s.rules.Themes(s.c.VaadinVersion)
	if len(list) == 0 {
		return
	}
	s.keep(s.fields.theme, &s.c.Theme, list, s.rules.ThemeDefault(s.c.VaadinVersion, s.preferredTheme))
	relist(s.fields.theme, &s.c.Theme, themeOptions(s.rules, s.c.VaadinVersion, s.c.Theme))
	s.fields.theme.Description(themeNote(s.rules, s.c.VaadinVersion))
}

// keep decides where a re-derived list's cursor goes: on the choice already made
// in it when that is still offered, and where the list opens otherwise.
func (s *screen) keep(field *huh.Select[string], value *string, list []string, open string) {
	if s.visited[field] && slices.Contains(list, *value) {
		return
	}
	*value = open
}

// relist replaces a select's options and puts its cursor on the bound value,
// with every option still in view.
//
// huh takes the cursor from the bound value when the options are set, and scrolls
// the list so that row is its first: a list opening on its third release shows
// that release at the top and nothing of the two newer ones above it. Moving the
// cursor with the key it would be moved with scrolls only when the cursor leaves
// the view, and with every option in view it never does — so the list is set with
// its first option bound, and the cursor walked down to the answer.
func relist(field *huh.Select[string], value *string, options []huh.Option[string]) {
	target := *value
	*value = options[0].Value
	field.Options(options...)
	for _, option := range options {
		if option.Value == target {
			break
		}
		field.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if *value != target {
		// Not on the list at all — nothing to walk to, so the first option it is.
		*value = options[0].Value
	}
}

// quitting is the reminder at the end of the command bar. It is the only way out
// of the bar that is not a task, so it is on screen the whole time the bar is —
// including after something has been typed and the placeholder is gone.
var quitting = []ui.Shortcut{{Key: "quit", Label: "to finish"}}

// stopping is what the bar offers instead while a task is running. The keys
// belong to the task then, and this is the one that takes them back.
var stopping = []ui.Shortcut{{Key: "ctrl+c", Label: "to stop", Active: true}}

// sectionKey reads a jump key. Alt and a digit, rather than the digit alone,
// because most of these questions are typed into and a bare 1 belongs to
// whichever field the cursor is in.
func sectionKey(key string) (int, bool) {
	digit, found := strings.CutPrefix(key, "alt+")
	if !found || len(digit) != 1 {
		return 0, false
	}
	n, err := strconv.Atoi(digit)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// jump moves the cursor to the nth section on screen.
//
// Walked rather than set, because huh keeps the position to itself: back to the
// first section, then forward. The steps that have nowhere to go are no-ops, and
// the last step never lands past the target, so the confirm at the end is never
// reached by jumping — the form submits when it is stepped past its last group.
func (s *screen) jump(n int) tea.Cmd {
	shown := s.tiled.shown()
	if n > len(shown) {
		return nil
	}
	active := s.tiled.active(s.form)
	if active == n-1 {
		return nil
	}

	// Leaving a field is what validates it. A section holding an answer that
	// cannot be used is not one to walk away from, so the jump is refused and
	// the error it just raised is what the user sees.
	var cmds []tea.Cmd
	if field := s.form.GetFocusedField(); field != nil {
		cmds = append(cmds, field.Blur())
		if active >= 0 && len(shown[active].group.Errors()) > 0 {
			cmds = append(cmds, field.Focus())
			return tea.Batch(cmds...)
		}
	}

	// Blurred at every step, not only the first. Arriving in a group focuses the
	// field waiting there, and huh only blurs the one being left when it is the
	// keyboard doing the leaving — so a walk that does not blur as it goes lights
	// up a field in every section it passed through.
	step := func(move func() tea.Cmd) {
		if field := s.form.GetFocusedField(); field != nil {
			cmds = append(cmds, field.Blur())
		}
		cmds = append(cmds, move())
	}
	for range s.tiled.sections {
		step(s.form.PrevGroup)
	}
	for i := 0; i < n-1; i++ {
		step(s.form.NextGroup)
	}
	return tea.Batch(cmds...)
}

// relayout works out how the sections fit in the terminal as it is now.
//
// Tiled if they fit, one at a time if they do not: a narrow or short terminal
// showing five columns shows five unreadable ones, and the questions matter more
// than the overview.
func (s *screen) relayout() tea.Cmd {
	s.shown = len(s.tiled.shown())

	// The command bar has the width of the bar it sits in, less what the reminder
	// at the end of it takes. Without being told, a text input shows one
	// character of its placeholder and nothing else.
	s.command.Width = max(s.width-lipgloss.Width(s.command.Prompt)-
		lipgloss.Width(ui.Shortcuts(quitting, 0))-4, 1)

	size := tea.WindowSizeMsg{Width: s.width, Height: s.height - s.chromeHeight()}

	s.form.WithLayout(s.tiled)
	_, cmd := s.form.Update(size)

	s.columns = s.tiled.Fits(s.form, size.Width, size.Height)
	if !s.columns {
		s.form.WithLayout(huh.LayoutDefault)
		// One less row than there is room for: huh's own layout puts a blank line
		// between a group and its help, and counts the group's height without it.
		size.Height--
		_, cmd = s.form.Update(size)
	}
	return cmd
}

// chromeHeight is what the screen spends on everything that is not the form: the
// banner above it and the blank line under it, and the rule and keys along the
// bottom.
func (s *screen) chromeHeight() int {
	return lipgloss.Height(s.banner) + 3
}

func (s *screen) View() string {
	if s.closing() {
		return ""
	}

	body := s.banner + "\n\n" + s.content()

	// The bar is held against the bottom edge rather than left to follow what is
	// above it. The form is a different height in every section — and a different
	// height again when a box is stretched — so a bar that follows it moves
	// about, and a row of keys that moves is one the eye has
	// to find again every time it is needed. It is also what makes the summary
	// arrive on the same screen the questions were asked on rather than a new one.
	filler := max(s.height-lipgloss.Height(body)-2, 0)

	return body + strings.Repeat("\n", filler+1) + ui.Rule(s.width) + "\n" + s.bar()
}

// closing reports whether there is nothing left to draw: the program is on its
// way out, and anything drawn now is a frame the terminal keeps.
func (s *screen) closing() bool {
	if s.failure != nil || s.leaving {
		return true
	}
	if s.phase != asking {
		return false
	}
	return s.form.State != huh.StateNormal
}

// content is the middle of the screen: the questions, or what came of them.
func (s *screen) content() string {
	if s.phase == asking {
		return s.form.View()
	}
	return s.result()
}

// result is what the screen becomes once there is a project: the summary and
// what to do next, drawn as boxes the same size and shape as the ones the
// questions were in, and under them a log that takes whatever room is left.
//
// The same screen carrying on, not the tool having handed the terminal to
// something else. A task named in the bar runs into the log below it, so the
// project can be brought up and tested without ever leaving the page it was
// asked for on.
func (s *screen) result() string {
	column := (s.width - columnGap) / 2

	if s.phase == writing {
		return ui.SectionBox("Writing", "", ui.Fields([]ui.Row{
			{Label: "where", Value: s.c.OutputDir},
		}, column-ui.SectionFrame), true, column, 0)
	}

	top := s.summary(column)

	// Everything the summary did not take. The log is the biggest thing on this
	// screen because it is the one thing here whose size nobody can predict.
	space := s.height - lipgloss.Height(s.banner) - 2
	height := max(space-lipgloss.Height(top)-1, 3)

	log := ui.SectionBox("Log", "",
		s.logView(s.width-ui.SectionFrame, height-2), s.running, s.width, height)

	return top + "\n" + log
}

// summary is the pair of boxes across the top of the result: what was written,
// and what can be done with it.
func (s *screen) summary(column int) string {
	notice := ""
	if s.outcome.Notice != "" {
		notice = "\n\n" + ui.Warning(s.outcome.Notice, column-ui.SectionFrame)
	}

	ready := func(height int) string {
		return ui.SectionBox("✓ "+s.outcome.Title, "",
			ui.Fields(s.outcome.Rows, column-ui.SectionFrame)+notice, false, column, height)
	}
	next := func(height int) string {
		return ui.SectionBox("Next", "",
			strings.TrimRight(ui.Commands(s.outcome.Steps, ""), "\n"), false, column, height)
	}

	// Drawn to the taller of the two, so the pair ends on one line like every
	// other row of boxes on this screen.
	height := max(lipgloss.Height(ready(0)), lipgloss.Height(next(0)))

	return lipgloss.JoinHorizontal(lipgloss.Top,
		ready(height), lipgloss.NewStyle().MarginLeft(columnGap).Render(next(height)))
}

// bar is what is drawn along the bottom: while the questions are being answered,
// the keys that work where the cursor is and the key for every section, with the
// one the cursor is in lit up — so the same line that says where you can go says
// where you are. Once there is a project, the same line becomes the command bar.
func (s *screen) bar() string {
	switch s.phase {
	case writing:
		return ui.Bar(ui.Working("writing the project…"), nil, s.width)
	case written:
		if s.running {
			if s.stopped {
				return ui.Bar(ui.Working("stopping "+s.current+"…"), nil, s.width)
			}
			return ui.Bar(ui.Working("running "+s.current+"…"), stopping, s.width)
		}
		return ui.Bar(s.command.View(), quitting, s.width)
	}
	// The help is huh's, rendered at the width of the bar rather than the width
	// of the box the cursor is in: a column is narrow, and help cut off at a
	// column's width is help with an ellipsis where the keys should be.
	help := s.form.Help()
	help.Width = 0
	return ui.Bar(help.ShortHelpView(s.form.KeyBinds()), s.jumpKeys(), s.width)
}

func (s *screen) jumpKeys() []ui.Shortcut {
	shown := s.tiled.shown()
	active := s.tiled.active(s.form)

	items := make([]ui.Shortcut, 0, len(shown))
	for i, section := range shown {
		items = append(items, ui.Shortcut{
			Key:    fmt.Sprintf("alt+%d", i+1),
			Label:  strings.ToLower(section.title),
			Active: i == active,
		})
	}
	return items
}
