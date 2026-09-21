// Command checkcompat checks compat.json against what Maven Central publishes.
//
// The rules in that file are written by hand from release notes, and they change a
// handful of times a year — at a Vaadin minor or a Spring Boot minor — with nobody
// told. This is what tells them: run weekly by CI, it fails when the file and the
// published releases have drifted apart, quoting the rule's source so the fix is
// a read and an edit. It is CI and not a test, because a test that talks to Maven
// Central makes `go test ./...` fail on an aeroplane.
//
//	go run ./internal/checkcompat [compat.json]
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/binarycodes/vaadin-init/internal/compat"
	"github.com/binarycodes/vaadin-init/internal/version"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

const timeout = 30 * time.Second

func main() {
	path := "compat.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := run(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The schema check: a file that does not parse, or names an entry wrongly,
	// fails here with the entry named.
	rules, err := compat.Parse(content)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}

	// Every Vaadin line, to find the ones with no rule; only the Boot lines the
	// supported Vaadin lines sit on, which are the ones the tool ever offers.
	_, bootLines := rules.Supported()
	available := versions.Lookup(ctx, client, versions.Lines{Boot: bootLines})
	if len(available.Vaadin) == 0 || len(available.Boot) == 0 {
		return fmt.Errorf("Maven Central could not be read; nothing to check against")
	}

	pins := map[string]string{}
	for _, newest := range newestPerLine(rules, available.Vaadin) {
		pinned, err := versions.PinnedBoot(ctx, client, newest)
		if err != nil {
			return fmt.Errorf("reading the starter pom of Vaadin %s: %w", newest, err)
		}
		pins[newest] = pinned
	}

	problems := findings(rules, available, pins, time.Now())
	if len(problems) == 0 {
		fmt.Printf("%s agrees with Maven Central: Vaadin %s, Spring Boot %s, pins %v\n",
			path, available.Vaadin[0], available.Boot[0], pins)
		return nil
	}
	return fmt.Errorf("%s is out of date:\n\n%s", path, strings.Join(problems, "\n\n"))
}

// newestPerLine is the newest release of each supported Vaadin line: the release
// whose starter pom says which Boot the line is built with today.
func newestPerLine(rules compat.Rules, vaadin []string) []string {
	supported, _ := rules.Supported()
	var newest []string
	for _, line := range supported {
		for _, release := range vaadin { // newest first
			if v, ok := version.Parse(release); ok && v.InLine(line) {
				newest = append(newest, release)
				break
			}
		}
	}
	return newest
}

// findings is every way the rules and the published releases disagree. Pure, so
// that it can be tested without the network the rest of this program needs.
//
// available is what Maven Central lists, newest first: every Vaadin release, and
// the Boot releases of the lines the supported Vaadin lines sit on. pins is the
// Boot each supported line's newest release was built with, by that release.
func findings(rules compat.Rules, available versions.Available, pins map[string]string, today time.Time) []string {
	var problems []string
	report := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	// The pin has moved ahead of the rules: the newest release of a supported line
	// was built with a Boot the rules do not allow for it, which is how a new
	// `since` entry — or a new line — gets noticed.
	for _, newest := range newestPerLine(rules, available.Vaadin) {
		pinned := pins[newest]
		if pinned == "" {
			report("Vaadin %s has no starter pom on Maven Central, so its Spring Boot cannot be checked", newest)
			continue
		}
		if err := rules.CheckBoot(newest, pinned); err != nil {
			report("Vaadin %s was built with Spring Boot %s, which the rules refuse for it:\n  %s", newest, pinned, err)
		}
	}

	// A Vaadin major with no rule at all: a new line going GA, or an old one the
	// file never mentioned. Only majors from the oldest ruled line up, since the
	// lines before it are history nobody needs a rule for.
	oldest := oldestLine(rules)
	for _, major := range majors(available.Vaadin) {
		if major < oldest {
			continue
		}
		if _, err := rules.Vaadin(fmt.Sprintf("%d.0.0", major)); err != nil && !strings.Contains(err.Error(), "sits on") {
			report("Maven Central lists Vaadin %d and the rules have no line for it; add one, supported or not", major)
		}
	}

	// A Boot line a supported Vaadin line sits on is past its support window.
	_, bootLines := rules.Supported()
	for _, rule := range rules.BootRules {
		if rule.EOL == "" || !underAny(rule.Line, bootLines) {
			continue
		}
		if eol, err := time.Parse(time.DateOnly, rule.EOL); err == nil && today.After(eol) {
			report("Spring Boot %s reached the end of its support on %s; decide what the tool offers now\n  (%s)",
				rule.Line, rule.EOL, rule.Source)
		}
	}

	// A Boot minor exists with no rule, so nothing says which JDKs it runs on.
	seen := map[string]bool{}
	for _, release := range available.Boot {
		v, ok := version.Parse(release)
		if !ok || seen[v.MinorLine()] {
			continue
		}
		seen[v.MinorLine()] = true
		if !slices.ContainsFunc(rules.BootRules, func(rule compat.BootRule) bool { return v.InLine(rule.Line) }) {
			report("Maven Central lists Spring Boot %s and the rules have no line for it; record its Java range and end of support\n  (https://docs.spring.io/spring-boot/%s/system-requirements.html)",
				v.MinorLine(), v.MinorLine())
		}
	}
	return problems
}

func oldestLine(rules compat.Rules) int {
	oldest := 0
	for _, rule := range rules.VaadinRules {
		if v, ok := version.Parse(rule.Line + ".0"); ok && (oldest == 0 || v.Major < oldest) {
			oldest = v.Major
		}
	}
	return oldest
}

// majors is every major number in a release list, oldest first.
func majors(releases []string) []int {
	var list []int
	for _, release := range releases {
		if v, ok := version.Parse(release); ok && !slices.Contains(list, v.Major) {
			list = append(list, v.Major)
		}
	}
	slices.Sort(list)
	return list
}

// underAny reports whether a Boot line ("4.1") is part of one of the lines a
// supported Vaadin line sits on ("4").
func underAny(line string, lines []string) bool {
	v, ok := version.Parse(line + ".0")
	if !ok {
		return false
	}
	return slices.ContainsFunc(lines, v.InLine)
}
