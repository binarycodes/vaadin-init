package prompt

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// As many releases of each as a real lookup returns.
func fetched() versions.Available {
	return versions.Available{
		Vaadin: []string{"25.2.6", "25.2.5", "25.2.4", "25.2.3", "25.2.2"},
		Boot:   []string{"4.1.1", "4.1.0", "4.0.8", "4.0.7", "4.0.6"},
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]`)

// screenAt brings the full-screen form up in a terminal of the given size, with
// the version lookup already landed.
func screenAt(t *testing.T, width, height int) *screen {
	t.Helper()

	c := seed()
	s := newScreen(&c, offered(), Options{Banner: banner})
	s.Init()
	s.Update(versionsMsg(fetched()))
	s.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return s
}

// The banner is part of what has to fit, so the sizes these tests talk about are
// the sizes the tool really has to lay itself out in.
var banner = ui.Banner("v0.0.0", "25", "4")

// view is what is on the screen, with the styling stripped so that assertions
// are about the text.
func view(s *screen) string {
	return ansi.ReplaceAllString(s.View(), "")
}

func press(s *screen, msg tea.KeyMsg) {
	s.Update(msg)
}

// next moves to the next question, the way tab does. Called on the form rather
// than pressed, because the key that moves a cursor between fields does it by
// asking the runtime for another message, and there is no runtime here.
func next(s *screen) {
	s.form.NextField()
}

func typeIn(s *screen, text string) {
	for _, r := range text {
		s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// The size a terminal has to be before the sections are tiled. Wide enough for
// five columns of questions, and tall enough for the longest of them.
const (
	wide = 160
	tall = 40
)

// The whole of what is about to be generated has to be on the screen at once:
// every section, every version on offer, and every piece of the stack.
//
// This is the point of the screen. A section the user cannot see is one they
// cannot check before pressing generate, which is the same as not having asked.
func TestEverySectionIsOnScreenAtOnce(t *testing.T) {
	s := screenAt(t, wide, tall)

	if !s.columns {
		t.Fatalf("the sections should be tiled in a %dx%d terminal", wide, tall)
	}

	on := view(s)
	for _, section := range s.tiled.shown() {
		if !strings.Contains(on, section.title) {
			t.Errorf("section %q is not on the screen", section.title)
		}
	}
	for _, version := range append(fetched().Vaadin, fetched().Boot...) {
		if !strings.Contains(on, version) {
			t.Errorf("version %s is not on the screen", version)
		}
	}
	for _, f := range featureList {
		label := strings.SplitN(f.label, " —", 2)[0]
		if !strings.Contains(on, label) {
			t.Errorf("stack option %q is not on the screen", label)
		}
	}
	for _, answer := range []string{"com.example", "my-app", "My App", "com.example.myapp", "Generate"} {
		if !strings.Contains(on, answer) {
			t.Errorf("answer %q is not on the screen", answer)
		}
	}
}

// Every section has a key that goes straight to it, listed under the form.
func TestEverySectionHasAJumpKey(t *testing.T) {
	s := screenAt(t, wide, tall)

	hints := ansi.ReplaceAllString(s.bar(), "")
	for i, section := range s.tiled.shown() {
		if !strings.Contains(hints, strings.ToLower(section.title)) {
			t.Errorf("section %q is not offered a key", section.title)
		}
		_ = i
	}
	if !strings.Contains(hints, "alt+1") || !strings.Contains(hints, "alt+5") {
		t.Errorf("the jump keys are not on the screen: %q", hints)
	}
}

// A jump key moves the cursor to that section, from wherever it was.
func TestAJumpKeyGoesToItsSection(t *testing.T) {
	s := screenAt(t, wide, tall)

	if got := s.tiled.active(s.form); got != 0 {
		t.Fatalf("the cursor should start in the first section, not %d", got)
	}

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}, Alt: true})
	if got := s.tiled.active(s.form); got != 3 {
		t.Errorf("alt+4 should go to the fourth section, went to %d", got)
	}

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	if got := s.tiled.active(s.form); got != 1 {
		t.Errorf("alt+2 should go back to the second section, went to %d", got)
	}
}

// Only one field is ever being answered, wherever the cursor was told to go.
//
// Arriving in a section focuses the question waiting there, and huh blurs the one
// being left only when it is the keyboard doing the leaving — so a jump that
// walks past three sections can leave a lit-up field in each of them, and the
// screen then shows four cursors and no way to tell which one types.
func TestOnlyOneQuestionIsEverActive(t *testing.T) {
	s := screenAt(t, wide, tall)

	for _, n := range []int{5, 2, 4, 1, 3} {
		press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('0' + n)}, Alt: true})

		lit := 0
		for _, section := range s.tiled.shown() {
			if strings.Contains(ansi.ReplaceAllString(section.group.Content(), ""), focusBar) {
				lit++
			}
		}
		if lit != 1 {
			t.Errorf("after alt+%d, %d sections have a field being answered", n, lit)
		}
	}
}

// focusBar is what the theme draws down the left of the field being answered.
const focusBar = "┃"

// A jump key for a section that is not there does nothing at all.
func TestAJumpKeyPastTheLastSectionIsIgnored(t *testing.T) {
	s := screenAt(t, wide, tall)

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}, Alt: true})

	if got := s.tiled.active(s.form); got != 0 {
		t.Errorf("the cursor should not have moved, it is in section %d", got)
	}
	if s.form.State != huh.StateNormal {
		t.Errorf("the form should still be being filled in")
	}
}

// The answers derived from the coordinates follow them as they are typed.
//
// On one screen the coordinates and what they derive are both in front of the
// user, so a package that still says what the defaults file guessed is visibly
// wrong — and this is what the old split into two forms was buying.
func TestDerivedAnswersFollowTheCoordinatesOnScreen(t *testing.T) {
	s := screenAt(t, wide, tall)

	next(s) // on to the artifact id
	typeIn(s, "s")

	if s.c.ArtifactID != "my-apps" {
		t.Fatalf("the artifact id is %q", s.c.ArtifactID)
	}
	if s.c.ProjectName != "My Apps" {
		t.Errorf("project name = %q, want it to follow the artifact id", s.c.ProjectName)
	}
	if s.c.Package != "com.example.myapps" {
		t.Errorf("package = %q, want it to follow the coordinates", s.c.Package)
	}
	if s.c.OutputDir != "my-apps" {
		t.Errorf("directory = %q, want it to follow the artifact id", s.c.OutputDir)
	}

	on := view(s)
	for _, answer := range []string{"My Apps", "com.example.myapps"} {
		if !strings.Contains(on, answer) {
			t.Errorf("%q is not on the screen, so the columns disagree with each other", answer)
		}
	}
}

// An answer the user has typed is never taken back by a later edit to the
// coordinates.
func TestATypedAnswerIsNotOverwritten(t *testing.T) {
	s := screenAt(t, wide, tall)

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true})
	typeIn(s, "!") // a project name of their own
	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true})
	next(s)
	typeIn(s, "s")

	if s.c.ProjectName != "My App!" {
		t.Errorf("project name = %q, want the one that was typed", s.c.ProjectName)
	}
	if s.c.Package != "com.example.myapps" {
		t.Errorf("package = %q, want the untouched one to still follow", s.c.Package)
	}
}

// The version lists arrive after the screen is up, and the newest release is the
// answer they arrive on.
func TestTheVersionListsOpenOnTheNewestRelease(t *testing.T) {
	c := seed()
	c.VaadinVersion = "25.2.2"
	c.BootVersion = "4.0.6"

	s := newScreen(&c, offered(), Options{Banner: banner})
	s.Init()
	s.Update(tea.WindowSizeMsg{Width: wide, Height: tall})

	if on := view(s); !strings.Contains(on, "25.2.2") {
		t.Error("the version the defaults named should be offered until the lookup lands")
	}

	s.Update(versionsMsg(fetched()))

	if s.c.VaadinVersion != fetched().Vaadin[0] {
		t.Errorf("Vaadin version = %q, want the newest release", s.c.VaadinVersion)
	}
	if s.c.BootVersion != fetched().Boot[0] {
		t.Errorf("Spring Boot version = %q, want the newest release", s.c.BootVersion)
	}
}

// A terminal too small to tile asks one section at a time rather than five
// unreadable columns.
func TestASmallTerminalAsksOneSectionAtATime(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{80, 24},
		{100, 20},
		{60, 40},
	} {
		s := screenAt(t, size.width, size.height)
		if s.columns {
			t.Errorf("%dx%d: the sections should not be tiled", size.width, size.height)
		}

		on := view(s)
		if !strings.Contains(on, "Group ID") {
			t.Errorf("%dx%d: the first question is not on the screen", size.width, size.height)
		}
		if width := widest(on); width > size.width {
			t.Errorf("%dx%d: the screen is %d columns wide", size.width, size.height, width)
		}
	}
}

// The screen has to fit the terminal it is drawn in, whichever way it is laid
// out: a form taller than the window loses its footer, and one wider than the
// window is wrapped by the terminal into nonsense.
func TestTheScreenFitsTheTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{160, 40},
		{140, 34},
		{120, 30},
		{100, 24},
		{80, 24},
	} {
		s := screenAt(t, size.width, size.height)
		on := view(s)

		if height := strings.Count(on, "\n") + 1; height > size.height {
			t.Errorf("%dx%d: the screen is %d rows tall", size.width, size.height, height)
		}
		if width := widest(on); width > size.width {
			t.Errorf("%dx%d: the screen is %d columns wide", size.width, size.height, width)
		}
	}
}

func widest(text string) int {
	width := 0
	for _, line := range strings.Split(text, "\n") {
		if n := len([]rune(strings.TrimRight(line, " "))); n > width {
			width = n
		}
	}
	return width
}

// Agreeing to generate ends the screen, and hands the terminal back.
//
// The form submits when it is stepped past its last section, which is what enter
// on the confirm does; the screen has to notice and quit, or the alt screen is
// never given back and the answers never reach the caller.
func TestAgreeingEndsTheScreen(t *testing.T) {
	s := screenAt(t, wide, tall)

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true})
	s.form.NextGroup()

	if s.form.State != huh.StateCompleted {
		t.Fatalf("the form should be finished, its state is %v", s.form.State)
	}

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("the screen should ask to quit once the form is finished")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("the screen should ask to quit once the form is finished")
	}
	if s.View() != "" {
		t.Error("the screen should draw nothing once it is finished, so the terminal comes back clean")
	}
}

// The escape hatch is a section like any other, and only there when it is asked
// for: choosing "type one myself" adds a column, and answering it takes the
// column away again.
func TestTypingAVersionAddsASection(t *testing.T) {
	s := screenAt(t, wide, tall)

	if got := len(s.tiled.shown()); got != 5 {
		t.Fatalf("%d sections before a version is typed, want 5", got)
	}

	s.c.VaadinVersion = custom
	s.Update(tea.WindowSizeMsg{Width: wide, Height: tall})

	if got := len(s.tiled.shown()); got != 6 {
		t.Errorf("%d sections after choosing to type a version, want 6", got)
	}
	// Counted rather than matched on its prose, which wraps differently in a
	// column than it does in a line: the question appears twice now, once as the
	// list it was not on and once as the box to type it into.
	if got := strings.Count(view(s), "Vaadin version"); got < 2 {
		t.Error("the typed-version question is not on the screen")
	}
	if hints := ansi.ReplaceAllString(s.bar(), ""); !strings.Contains(hints, "alt+6") {
		t.Errorf("the new section has no key of its own: %q", hints)
	}
}

// The bar is on the bottom row of the terminal, whatever is above it.
//
// Not a line that follows the form: the form is a different height in every
// section, and in the tiled screen it changes height again whenever a box is
// stretched or an escape hatch appears. A row of keys that moves is one the eye
// has to find again every time it is wanted.
func TestTheBarIsAlwaysOnTheBottomRow(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{wide, tall},
		{140, 36},
		{100, 30},
		{80, 24},
	} {
		s := screenAt(t, size.width, size.height)

		for _, n := range []int{1, 3, 5} {
			press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('0' + n)}, Alt: true})

			lines := strings.Split(view(s), "\n")
			if len(lines) != size.height {
				t.Errorf("%dx%d: the screen is %d rows, so it does not reach the bottom",
					size.width, size.height, len(lines))
				break
			}
			if !strings.Contains(lines[len(lines)-1], "alt+1") {
				t.Errorf("%dx%d: the bottom row is not the bar, it is %q",
					size.width, size.height, lines[len(lines)-1])
				break
			}
		}
	}
}

// The output section runs under all of the columns, not beside one of them.
//
// Where a section belongs is a question about the questions: the directory the
// project goes in, and the button that starts the whole thing, follow every
// answer above them — so they get a row of their own, the width of the screen,
// rather than the foot of whichever column they happened to land in.
func TestOutputRunsUnderTheColumns(t *testing.T) {
	s := screenAt(t, wide, tall)

	columns, rows := s.tiled.columns()
	shown := s.tiled.shown()

	if len(rows) != 1 || shown[rows[0]].title != "Output" {
		t.Fatalf("%d sections have a row of their own, want just the output", len(rows))
	}
	if len(columns) != 4 {
		t.Errorf("%d columns, want the four the output does not sit in", len(columns))
	}

	// And it is drawn to the whole width, not to a column's worth of it.
	on := view(s)
	var output string
	for _, line := range strings.Split(on, "\n") {
		if strings.Contains(line, "─ Output ") {
			output = line
		}
	}
	if output == "" {
		t.Fatal("the output section is not on the screen")
	}
	if got := len([]rune(strings.TrimRight(output, " "))); got != wide {
		t.Errorf("the output box is %d columns wide, want the whole %d", got, wide)
	}
}

// One button, not two: there is no No to answer.
//
// Every other way out of this screen is ctrl+c or never having run it, and a No
// beside the Generate reads as a decision still to be made rather than the one
// already made by filling the form in.
func TestGeneratingIsOneButton(t *testing.T) {
	s := screenAt(t, wide, tall)

	on := view(s)
	if !strings.Contains(on, "Generate") {
		t.Fatal("there is no button to generate with")
	}
	if strings.Contains(on, "Yes") || strings.Contains(on, " No ") {
		t.Error("the button still has a second half")
	}

	// And the keys that went with the second half are gone from the bar.
	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true})
	s.form.NextField()

	bar := ansi.ReplaceAllString(s.bar(), "")
	if strings.Contains(bar, "toggle") {
		t.Errorf("the bar offers a key that does nothing: %q", bar)
	}
	if !strings.Contains(bar, "submit") {
		t.Errorf("the bar does not say the button submits: %q", bar)
	}
}
