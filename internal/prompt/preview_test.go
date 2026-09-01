package prompt

import (
	"fmt"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// TestPreview prints the form as it will appear, one group at a time.
//
// A form is a tea.Model, so a frame of it can be rendered without a terminal to
// run it in — which is the only way to review how this looks from a place that
// has no terminal, and the only way to see a styling change in a diff-able form.
//
// Skipped by default because it asserts nothing; run it to look:
//
//	CLICOLOR_FORCE=1 PREVIEW=1 go test ./internal/prompt/ -run TestPreview -v
func TestPreview(t *testing.T) {
	if os.Getenv("PREVIEW") == "" {
		t.Skip("set PREVIEW=1 to print the rendered form")
	}

	// Stated rather than detected: there is no terminal here to detect, and the
	// point is to see the colours a capable terminal will show. PROFILE=ansi
	// renders the sixteen-colour fallback instead, which is the case worth
	// looking at deliberately.
	profile := termenv.TrueColor
	if os.Getenv("PROFILE") == "ansi" {
		profile = termenv.ANSI
	}
	lipgloss.SetColorProfile(profile)
	lipgloss.SetHasDarkBackground(os.Getenv("LIGHT") == "")

	c := seed()
	theme := ui.Theme()
	available := offered().Value()

	forms := []struct {
		name string
		form *huh.Form
	}{
		{"coordinates", coordinatesForm(&c, theme, Options{})},
	}

	features := selectedFeatures(c)
	confirmed := true
	rest := restForm(&c, &features, &confirmed, available,
		available.Vaadin, available.Boot, theme, Options{})

	fmt.Print(ui.Banner("v0.1.0", "25", "4"))

	for _, f := range forms {
		fmt.Println(frame(f.form))
	}

	// The second form holds several groups and shows one at a time, so each is
	// rendered and then advanced past.
	//
	// Initialised once, outside the loop: re-initialising between frames makes
	// the form redistribute the window height each time and progressively shrink
	// the field it is showing, which looks exactly like a list dropping its last
	// option and is a fault in this harness rather than in the form.
	start(rest)
	for i := 0; i < 5; i++ {
		fmt.Println(rest.View())
		rest.NextGroup()
	}

	// What the tool prints after the form is part of the same look, so it is
	// reviewed in the same place.
	fmt.Print(ui.Summary("Note Harbor is ready", []ui.Row{
		{Label: "where", Value: "/home/dev/note-harbor  (22 files)"},
		{Label: "stack", Value: ui.Join("Vaadin 25.2.6", "Spring Boot 4.1.1", "Java 21")},
		{Label: "options", Value: ui.Join("database", "auth", "e2e", "coverage", "traceable builds")},
		{Label: "git", Value: "initialised, commit-msg hook wired up"},
	}, ""))

	fmt.Println()
	fmt.Print(ui.NextSteps("Next", []ui.Step{
		{Command: "cd note-harbor"},
		{Command: "./run.sh env", Purpose: "bring up the development stack"},
		{Command: "./run.sh run", Purpose: "start the application"},
		{Command: "./run.sh verify", Purpose: "unit tests and integration tests"},
		{Command: "./run.sh help", Purpose: "every task"},
	}))

	fmt.Println()
	fmt.Printf("  %s\n\n", ui.Heading("8 files would be written"))
	fmt.Println(ui.FileTree("/home/dev/note-harbor", []string{
		"pom.xml",
		"run.sh",
		".githooks/commit-msg",
		"environment/dev/compose.yaml",
		"environment/dev/keycloak/realm.json",
		"src/main/java/io/binarycodes/noteharbor/Application.java",
		"src/main/java/io/binarycodes/noteharbor/ui/view/MainView.java",
		"src/main/resources/db/migration/V1__init_schema.sql",
	}))
}

// start brings a form up as a terminal would: initialise it, then tell it the
// size of the window it has to lay out in.
func start(form *huh.Form) {
	form.Init()
	form.Update(tea.WindowSizeMsg{Width: 96, Height: 24})
}

// frame renders one still of a form.
func frame(form *huh.Form) string {
	start(form)
	return form.View()
}

// Value makes a VersionSource readable as the value it produces.
func (s VersionSource) Value() versions.Available { return s() }

var _ = config.Config{}
