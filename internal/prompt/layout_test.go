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

// The terminal sizes these layouts have to hold up in. 20 rows is a small
// terminal but not an unreasonable one, and it is the size that exposed both of
// the faults these tests exist for.
var windowHeights = []int{20, 24, 30, 40}

// As many releases of each as a real lookup returns.
func fetched() versions.Available {
	return versions.Available{
		Vaadin: []string{"25.2.6", "25.2.5", "25.2.4", "25.2.3", "25.2.2"},
		Boot:   []string{"4.1.1", "4.1.0", "4.0.8", "4.0.7", "4.0.6"},
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]`)

// groupViews walks the form and returns what each group renders, with the styling
// stripped so assertions are about the text on screen.
//
// Stepping rather than rendering one group: a group's height is settled by the
// form as a whole, so a group examined on its own is not the group the user sees.
func groupViews(t *testing.T, windowHeight int) []string {
	t.Helper()

	c := seed()
	features := selectedFeatures(c)
	confirmed := true
	available := fetched()

	form := restForm(&c, &features, &confirmed, &typedVersions{}, available,
		available.Vaadin, available.Boot, ui.Theme(), Options{})
	form.Init()
	form.Update(tea.WindowSizeMsg{Width: 96, Height: windowHeight})

	var views []string
	for i := 0; i < 8; i++ {
		views = append(views, ansi.ReplaceAllString(form.View(), ""))
		form.NextGroup()
	}
	return views
}

// viewContaining finds the group that asked about something, by a phrase only it
// carries.
func viewContaining(views []string, marker string) string {
	for _, view := range views {
		if strings.Contains(view, marker) {
			return view
		}
	}
	return ""
}

// Every version a list offers has to be on screen when that list is reached.
//
// This is the fault the tool shipped with: huh scans the options for the bound
// value to decide which line the cursor opens on, and that scan happens inside
// Options — so a value bound afterwards arrived too late and the list opened at
// its last entry with every version scrolled out of sight above. It looked like a
// list with one item until an arrow key was pressed.
func TestVersionListsShowEveryOption(t *testing.T) {
	for _, height := range windowHeights {
		views := groupViews(t, height)

		for _, list := range []struct {
			marker   string
			expected []string
		}{
			{"Vaadin version", fetched().Vaadin},
			{"Spring Boot version", fetched().Boot},
		} {
			view := viewContaining(views, list.marker)
			if view == "" {
				t.Fatalf("window height %d: no group asked about %q", height, list.marker)
			}
			for _, version := range list.expected {
				if !strings.Contains(view, version) {
					t.Errorf("window height %d: version %s is not visible in its list", height, version)
				}
			}
			if !strings.Contains(view, "type one myself") {
				t.Errorf("window height %d: the escape hatch is not visible in %q", height, list.marker)
			}
		}
	}
}

// The cursor has to open on the newest release, since that is the answer the
// prompt is offering and the one enter accepts.
func TestVersionListsOpenOnTheNewestRelease(t *testing.T) {
	views := groupViews(t, 24)

	for _, list := range []struct {
		marker string
		newest string
	}{
		{"Vaadin version", fetched().Vaadin[0]},
		{"Spring Boot version", fetched().Boot[0]},
	} {
		view := viewContaining(views, list.marker)
		if view == "" {
			t.Fatalf("no group asked about %q", list.marker)
		}
		var cursorLine string
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, ui.Caret) {
				cursorLine = line
				break
			}
		}
		if !strings.Contains(cursorLine, list.newest) {
			t.Errorf("the cursor should open on %s, but the cursor line is %q", list.newest, cursorLine)
		}
	}
}

// Every option in the stack list has to be on screen without scrolling.
//
// huh gives a group one height for the whole form and shrinks a field that does
// not fit, so a line of prose added to this group steals a row from the list and
// drops its last option out of view. Nothing errors, and the option that
// disappears is always the last one — so the tool would simply stop offering it.
func TestStackListShowsEveryOptionWithoutScrolling(t *testing.T) {
	for _, height := range windowHeights {
		view := viewContaining(groupViews(t, height), "The core is always generated")
		if view == "" {
			t.Fatalf("window height %d: no group asked about the stack", height)
		}
		for _, f := range featureList {
			label := strings.SplitN(f.label, " —", 2)[0]
			if !strings.Contains(view, label) {
				t.Errorf("window height %d: %q is not visible in the stack list", height, label)
			}
		}
	}
}

// The stack list must open at its first option whatever the defaults select.
//
// huh puts the cursor — and the viewport — on the first *selected* option, so a
// defaults file that turns the first entries off would otherwise open the list
// already scrolled past them.
func TestStackListOpensAtTheTopWhateverIsPreselected(t *testing.T) {
	c := seed()
	c.Database = false
	c.Auth = false

	features := selectedFeatures(c)
	confirmed := true
	available := fetched()

	form := restForm(&c, &features, &confirmed, &typedVersions{}, available,
		available.Vaadin, available.Boot, ui.Theme(), Options{})
	form.Init()
	form.Update(tea.WindowSizeMsg{Width: 96, Height: 24})

	var view string
	for i := 0; i < 8; i++ {
		candidate := ansi.ReplaceAllString(form.View(), "")
		if strings.Contains(candidate, "The core is always generated") {
			view = candidate
			break
		}
		form.NextGroup()
	}
	if view == "" {
		t.Fatal("no group asked about the stack")
	}
	if !strings.Contains(view, "Database") {
		t.Error("the first option should be visible even when nothing at the top is preselected")
	}
}

var _ = huh.NewForm
