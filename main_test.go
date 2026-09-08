package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/generate"
	"github.com/binarycodes/vaadin-init/internal/ui"
)

// A scripted run whose versions do not go together fails before anything is
// written: the check runs before the generator, and that ordering is the thing
// worth pinning. Every version is given, so the run needs no network and the
// test none either.
func TestAScriptedRunWithAnIncompatibleSetWritesNothing(t *testing.T) {
	cases := []struct {
		name                       string
		vaadin, boot, java         string
		wantField, wantExplanation string
	}{
		{"a Boot older than the Vaadin needs", "25.2.6", "4.0.8", "21",
			"spring boot version", "needs Spring Boot 4.1.0 or newer"},
		{"a JDK below the Vaadin floor", "25.2.6", "4.1.0", "17",
			"java version", "Vaadin 25 needs Java 21 or newer"},
		{"a Vaadin line this tool does not generate", "24.10.9", "3.5.15", "17",
			"vaadin version", "this tool generates Vaadin 25 projects"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "app")
			err := run([]string{
				"--yes", "--no-git",
				"--group-id", "com.example", "--artifact-id", "app", "--dir", target,
				"--vaadin-version", c.vaadin, "--boot-version", c.boot, "--java-version", c.java,
			})
			if err == nil {
				t.Fatal("the run succeeded")
			}
			if !strings.HasPrefix(err.Error(), c.wantField+":") || !strings.Contains(err.Error(), c.wantExplanation) {
				t.Errorf("error = %q, want %q explaining %q", err, c.wantField, c.wantExplanation)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Errorf("the target directory exists after a refused run: %v", err)
			}
		})
	}
}

// A fully pinned set that goes together is accepted with no network at all.
func TestAScriptedRunWithACompatibleSetIsAccepted(t *testing.T) {
	target := filepath.Join(t.TempDir(), "app")
	err := run([]string{
		"--yes", "--dry-run",
		"--group-id", "com.example", "--artifact-id", "app", "--dir", target,
		"--vaadin-version", "25.2.6", "--boot-version", "4.1.0", "--java-version", "25",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

// The summary says when the Boot is the release the Vaadin was built with — the
// one fact about the pair that was looked up rather than chosen — under the stack
// row, and says nothing when it is not.
func TestTheSummaryNamesTheBootTheVaadinWasBuiltWith(t *testing.T) {
	cfg := config.Config{ProjectName: "App", VaadinVersion: "25.2.6", BootVersion: "4.1.0", JavaVersion: "21", Theme: config.ThemeAura}
	result := generate.Result{Root: "/tmp/app", Paths: []string{"pom.xml"}}

	with := outcome(cfg, result, true)
	rendered := ui.Summary(with.Title, with.Rows, with.Notice)
	if !strings.Contains(rendered, "Spring Boot 4.1.0 is the release Vaadin 25.2.6 was built with") {
		t.Errorf("the summary does not say the Boot was pinned:\n%s", rendered)
	}
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if strings.Contains(line, "is the release") && !strings.HasPrefix(strings.TrimLeft(line, "│ "), "Spring Boot 4.1.0 is") {
			t.Errorf("line %d should be indented under the stack value: %q", i, line)
		}
	}

	without := outcome(cfg, result, false)
	for _, row := range without.Rows {
		if strings.Contains(row.Value, "was built with") {
			t.Errorf("a typed Boot should not be called pinned: %q", row.Value)
		}
	}
}
