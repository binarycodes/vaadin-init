package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/binarycodes/vaadin-init/internal/compat"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

func shipped(t *testing.T) compat.Rules {
	t.Helper()
	content, err := os.ReadFile("../../compat.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := compat.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

// What Maven Central listed on the day the file was written, and the pins the
// starter poms carried: the file agrees with it, so the check is quiet.
func today() (versions.Available, map[string]string, time.Time) {
	return versions.Available{
			Vaadin: []string{"25.2.6", "25.2.5", "25.1.11", "25.0.5", "24.10.9", "24.9.3", "23.5.0", "14.12.0"},
			Boot:   []string{"4.1.1", "4.1.0", "4.0.8", "4.0.0"},
		},
		map[string]string{"25.2.6": "4.1.0"},
		time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
}

func TestTheShippedFileAgreesWithTheDayItWasWritten(t *testing.T) {
	available, pins, day := today()
	if problems := findings(shipped(t), available, pins, day); len(problems) != 0 {
		t.Errorf("findings = %q, want none", problems)
	}
}

func TestAPinAheadOfTheRulesIsReported(t *testing.T) {
	available, pins, day := today()
	pins["25.2.6"] = "5.0.0"
	problems := findings(shipped(t), available, pins, day)
	if len(problems) != 1 || !strings.Contains(problems[0], "built with Spring Boot 5.0.0") {
		t.Errorf("findings = %q", problems)
	}
	if !strings.Contains(problems[0], "https://github.com/vaadin/platform/releases/tag/25.2.0") {
		t.Errorf("the finding should quote the rule's source: %q", problems[0])
	}

	pins["25.2.6"] = ""
	problems = findings(shipped(t), available, pins, day)
	if len(problems) != 1 || !strings.Contains(problems[0], "no starter pom") {
		t.Errorf("findings = %q", problems)
	}
}

// A new major on Maven Central with no rule is reported; the majors older than
// anything the file rules on are not.
func TestAVaadinMajorWithNoRuleIsReported(t *testing.T) {
	available, pins, day := today()
	available.Vaadin = append([]string{"26.0.0"}, available.Vaadin...)
	pins["25.2.6"] = "4.1.0"
	problems := findings(shipped(t), available, pins, day)
	if len(problems) != 1 || !strings.Contains(problems[0], "Vaadin 26") {
		t.Errorf("findings = %q", problems)
	}
	for _, p := range problems {
		if strings.Contains(p, "Vaadin 23") || strings.Contains(p, "Vaadin 14") {
			t.Errorf("a major older than the oldest ruled line is history: %q", p)
		}
	}
}

func TestABootLinePastItsSupportIsReported(t *testing.T) {
	available, pins, _ := today()
	problems := findings(shipped(t), available, pins, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if len(problems) != 1 || !strings.Contains(problems[0], "Spring Boot 4.0 reached the end of its support") {
		t.Errorf("findings = %q", problems)
	}
	// 3.5 is past its end of support too, and is not reported: no supported
	// Vaadin line sits on it.
	for _, p := range problems {
		if strings.Contains(p, "Spring Boot 3.5") {
			t.Errorf("a Boot line the tool never offers is not the check's business: %q", p)
		}
	}
}

func TestABootMinorWithNoRuleIsReported(t *testing.T) {
	available, pins, day := today()
	available.Boot = append([]string{"4.2.1", "4.2.0"}, available.Boot...)
	problems := findings(shipped(t), available, pins, day)
	if len(problems) != 1 || !strings.Contains(problems[0], "Spring Boot 4.2") || !strings.Contains(problems[0], "system-requirements") {
		t.Errorf("findings = %q", problems)
	}
}
