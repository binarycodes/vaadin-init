package prompt

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/binarycodes/vaadin-init/internal/ui"
)

// Every option in the stack list has to be on screen without scrolling.
//
// This is a layout assertion because the failure is silent and specific: huh
// gives a group one height for the whole form and shrinks a field that does not
// fit, so a line of description added anywhere in the group steals a row from the
// list and drops its last option out of view. Nothing errors, and the option that
// disappears is always the last one — so the tool would simply stop offering it.
func TestStackListShowsEveryOptionWithoutScrolling(t *testing.T) {
	for _, windowHeight := range []int{20, 24, 30, 40} {
		c := seed()
		features := selectedFeatures(c)
		confirmed := true
		available := offered().Value()

		form := restForm(&c, &features, &confirmed, available,
			available.Vaadin, available.Boot, ui.Theme(), Options{})
		form.Init()
		form.Update(tea.WindowSizeMsg{Width: 96, Height: windowHeight})

		// Identity → Versions → Stack. The custom-version groups in between are
		// hidden, and NextGroup steps over them.
		form.NextGroup()
		form.NextGroup()

		view := form.View()
		for _, f := range featureList {
			label := strings.SplitN(f.label, " —", 2)[0]
			if !strings.Contains(view, label) {
				t.Errorf("window height %d: %q is not visible in the stack list", windowHeight, label)
			}
		}
	}
}
