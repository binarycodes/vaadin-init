package prompt

import (
	"io"
	"strings"
	"testing"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

func seed() config.Config {
	c := config.Config{
		GroupID:       "com.example",
		ArtifactID:    "my-app",
		Description:   "A Vaadin application",
		JavaVersion:   "21",
		VaadinVersion: "25.2.6",
		BootVersion:   "4.1.1",
		Database:      true,
		E2E:           true,
		Coverage:      true,
		Traceable:     true,
	}
	c.ProjectName = config.DeriveProjectName(c.ArtifactID)
	c.Package = config.DerivePackage(c.GroupID, c.ArtifactID)
	c.OutputDir = c.ArtifactID
	return c
}

func offered() VersionSource {
	return func() versions.Available {
		return versions.Available{
			Vaadin: []string{"25.2.6", "25.2.5"},
			Boot:   []string{"4.1.1", "4.1.0"},
		}
	}
}

// run drives the whole conversation from a script of answers, in accessible
// mode — which is the only way to exercise the prompts without a terminal.
func run(t *testing.T, answers string) (config.Config, error) {
	t.Helper()
	return Run(seed(), offered(), Options{
		Accessible: true,
		Input:      strings.NewReader(answers),
		Output:     io.Discard,
	})
}

// A blank line accepts what a field already shows, so this is the path of
// someone who agrees with every default.
func TestAcceptingEveryDefault(t *testing.T) {
	answers := strings.Repeat("\n", 40)

	got, err := run(t, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.GroupID != "com.example" || got.ArtifactID != "my-app" {
		t.Errorf("coordinates changed: %q / %q", got.GroupID, got.ArtifactID)
	}
	if got.VaadinVersion != "25.2.6" {
		t.Errorf("Vaadin version = %q, want the newest offered", got.VaadinVersion)
	}
	if got.BootVersion != "4.1.1" {
		t.Errorf("Boot version = %q, want the newest offered", got.BootVersion)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("the accepted defaults do not validate: %v", err)
	}
}

// The second form's defaults have to follow the coordinates the first form just
// took, which is the reason the conversation is split into two forms at all.
func TestDerivedAnswersFollowTheCoordinates(t *testing.T) {
	answers := "io.binarycodes\nnote-harbor\n" + strings.Repeat("\n", 40)

	got, err := run(t, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.GroupID != "io.binarycodes" || got.ArtifactID != "note-harbor" {
		t.Fatalf("coordinates not taken: %q / %q", got.GroupID, got.ArtifactID)
	}
	if got.ProjectName != "Note Harbor" {
		t.Errorf("project name = %q, want it derived from the artifact id", got.ProjectName)
	}
	if got.Package != "io.binarycodes.noteharbor" {
		t.Errorf("package = %q, want it derived from the coordinates", got.Package)
	}
	if got.OutputDir != "note-harbor" {
		t.Errorf("output directory = %q, want the artifact id", got.OutputDir)
	}
}

// An answer that cannot produce a buildable project must not be accepted, and
// the prompt has to keep asking rather than carry it forward.
func TestInvalidCoordinateIsRefused(t *testing.T) {
	answers := "Com.Example\ncom.example\nmy-app\n" + strings.Repeat("\n", 40)

	got, err := run(t, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.GroupID != "com.example" {
		t.Errorf("group id = %q; the rejected answer should not have been kept", got.GroupID)
	}
}

// The stack multi-select is the one answer that has to be mapped back onto
// several fields, and the mapping is the part that can silently invert.
//
// In accessible mode the multi-select asks for a number at a time, toggling
// each, until 0 confirms the selection.
func TestStackSelectionIsAppliedBothWays(t *testing.T) {
	blanks := strings.Repeat("\n", 8) // through to the stack question
	answers := blanks + "2\n" + "1\n" + "0\n" + "\n" + "\n"

	got, err := run(t, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !got.Auth {
		t.Error("auth was selected and should be on")
	}
	if got.Database {
		t.Error("database was deselected and should be off")
	}
	// Untouched options keep the value they were seeded with, rather than being
	// cleared by the answer that named neither of them.
	if !got.E2E || !got.Coverage || !got.Traceable {
		t.Errorf("untouched options should have kept their seeded values: %v", got.Selected())
	}
}
