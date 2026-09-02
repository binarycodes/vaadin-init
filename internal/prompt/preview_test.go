package prompt

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// TestPreview prints the screen as it will appear, at a few terminal sizes.
//
// The screen is a tea.Model, so a frame of it can be rendered without a terminal
// to run it in — which is the only way to review how this looks from a place that
// has no terminal, and the only way to see a styling change in a diff-able form.
//
// Skipped by default because it asserts nothing; run it to look:
//
//	CLICOLOR_FORCE=1 PREVIEW=1 go test ./internal/prompt/ -run TestPreview -v
func TestPreview(t *testing.T) {
	if os.Getenv("PREVIEW") == "" {
		t.Skip("set PREVIEW=1 to print the rendered screen")
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

	sizes := []struct{ width, height int }{
		{160, 40},
		{140, 32},
		{120, 30},
		{100, 24},
		{80, 24},
	}
	if only := os.Getenv("SIZE"); only != "" {
		width, height, _ := strings.Cut(only, "x")
		w, _ := strconv.Atoi(width)
		h, _ := strconv.Atoi(height)
		sizes = sizes[:0]
		sizes = append(sizes, struct{ width, height int }{w, h})
	}

	for _, size := range sizes {
		s := preview(size.width, size.height)
		fmt.Printf("\n\n%s\n", strings.Repeat("─", size.width))
		fmt.Printf("%dx%d — tiled: %v\n\n", size.width, size.height, s.columns)
		fmt.Println(s.View())
	}

	// The same screen once there is a project on the other end of it.
	done := preview(sizes[0].width, sizes[0].height)
	done.outcome = Outcome{
		Title: "Note Harbor is ready",
		Rows: []ui.Row{
			{Label: "where", Value: "/home/dev/note-harbor  (22 files)"},
			{Label: "stack", Value: ui.Join("Vaadin 25.2.6", "Spring Boot 4.1.1", "Java 21")},
			{Label: "options", Value: ui.Join("database", "auth", "e2e", "coverage", "traceable builds")},
			{Label: "git", Value: "initialised, commit-msg hook wired up"},
		},
		Steps: []ui.Step{
			{Command: "cd note-harbor"},
			{Command: "./run.sh env", Purpose: "bring up the development stack"},
			{Command: "./run.sh run", Purpose: "start the application"},
			{Command: "./run.sh verify", Purpose: "unit tests and integration tests"},
			{Command: "./run.sh help", Purpose: "every task"},
		},
	}
	done.phase = written
	done.command.Focus()
	// A task that has been run, so the log is showing what it said.
	done.log = []string{
		ui.Echoed("run.sh test"),
		"[INFO] Scanning for projects...",
		"[INFO] Building note-harbor 0.0.1-SNAPSHOT",
		"[INFO] Tests run: 4, Failures: 0, Errors: 0, Skipped: 0",
		"[INFO] BUILD SUCCESS",
		ui.Finished("done"),
	}
	fmt.Printf("\n\n%s\n", strings.Repeat("─", sizes[0].width))
	fmt.Printf("%dx%d — written\n\n", sizes[0].width, sizes[0].height)
	fmt.Println(done.View())

	// What the tool prints after the screen is part of the same look, so it is
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

// preview brings a screen up as a terminal would: build it, hand it the
// versions the lookup would have found, and tell it the size of the window.
func preview(width, height int) *screen {
	c := seed()
	s := newScreen(&c, offered(), Options{
		Banner: ui.Banner("v0.1.0",
			strconv.Itoa(versions.VaadinMajor), strconv.Itoa(versions.BootMajor)),
	})
	s.Init()
	s.Update(versionsMsg(fetched()))
	s.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return s
}
