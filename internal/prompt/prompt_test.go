package prompt

import (
	"io"
	"strconv"
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
// In accessible mode the multi-select asks for a number at a time, toggling each,
// until 0 confirms the selection. Those numbers are positions in the list as
// displayed, which is not the order featureList declares — selected options are
// listed first — so the positions are looked up rather than written down.
func TestStackSelectionIsAppliedBothWays(t *testing.T) {
	position := func(key string) string {
		for i, option := range featureOptions(seed()) {
			if option.Value == key {
				return strconv.Itoa(i + 1)
			}
		}
		t.Fatalf("no option for %q", key)
		return ""
	}

	blanks := strings.Repeat("\n", 8) // through to the stack question
	answers := blanks +
		position("auth") + "\n" + // on
		position("database") + "\n" + // off
		"0\n" + // confirm
		"\n" + "\n" // directory, generate?

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

// Choosing "type one myself" leaves a sentinel in the version field, and the
// version typed in the group that follows has to replace it.
//
// The sentinel must never survive into a Config: it is not a version, and a
// pom.xml carrying it would name a dependency that does not exist.
func TestTypedVersionsResolveTheSentinel(t *testing.T) {
	cases := []struct {
		name         string
		vaadin, boot string
		typed        typedVersions
		wantVaadin   string
		wantBoot     string
	}{
		{
			name:   "a typed version replaces the sentinel",
			vaadin: custom, boot: custom,
			typed:      typedVersions{vaadin: "25.3.0-beta1", boot: "4.2.0-RC1"},
			wantVaadin: "25.3.0-beta1", wantBoot: "4.2.0-RC1",
		},
		{
			name:   "a chosen version is left alone",
			vaadin: "25.2.6", boot: "4.1.1",
			typed:      typedVersions{vaadin: "ignored", boot: "ignored"},
			wantVaadin: "25.2.6", wantBoot: "4.1.1",
		},
		{
			name:   "a sentinel with nothing typed falls back",
			vaadin: custom, boot: custom,
			typed:      typedVersions{},
			wantVaadin: "25.2.6", wantBoot: "4.1.1",
		},
		{
			name:   "only the field left on the sentinel is replaced",
			vaadin: custom, boot: "4.1.0",
			typed:      typedVersions{vaadin: "25.9.9"},
			wantVaadin: "25.9.9", wantBoot: "4.1.0",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := config.Config{VaadinVersion: c.vaadin, BootVersion: c.boot}
			c.typed.resolve(&cfg, "25.2.6", "4.1.1")

			if cfg.VaadinVersion != c.wantVaadin {
				t.Errorf("Vaadin version = %q, want %q", cfg.VaadinVersion, c.wantVaadin)
			}
			if cfg.BootVersion != c.wantBoot {
				t.Errorf("Boot version = %q, want %q", cfg.BootVersion, c.wantBoot)
			}
			if err := config.ValidVersion(cfg.VaadinVersion); err != nil {
				t.Errorf("resolved Vaadin version is not a version: %v", err)
			}
			if err := config.ValidVersion(cfg.BootVersion); err != nil {
				t.Errorf("resolved Boot version is not a version: %v", err)
			}
		})
	}
}

// The sentinel must not be mistakable for an answer.
func TestSentinelIsNotAValidVersion(t *testing.T) {
	if err := config.ValidVersion(custom); err == nil {
		t.Error("the sentinel should never pass version validation")
	}
	if custom == "" {
		t.Error("an empty sentinel collides with the zero value of the field it sits beside")
	}
}
