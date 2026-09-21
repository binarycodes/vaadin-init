package prompt

import (
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/binarycodes/vaadin-init/internal/compat"
	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// rules is the shipped compat.json, which is what the conversation applies unless
// the user has their own.
var rules = func() compat.Rules {
	content, err := os.ReadFile("../../compat.json")
	if err != nil {
		panic(err)
	}
	r, err := compat.Parse(content)
	if err != nil {
		panic(err)
	}
	return r
}()

// pinned is a pin source that knows one release: 25.2.6 was built with 4.1.0,
// which is not the newest Boot the lookup offers.
func pinned(vaadin string) string {
	if vaadin == "25.2.6" {
		return "4.1.0"
	}
	return ""
}

// conversation is the options every test conversation starts from.
func conversation() Options {
	return Options{Rules: rules, Pinned: pinned}
}

func seed() config.Config {
	c := config.Config{
		GroupID:       "com.example",
		ArtifactID:    "my-app",
		Description:   "A Vaadin application",
		JavaVersion:   "21",
		VaadinVersion: "25.2.6",
		BootVersion:   "4.1.1",
		Theme:         config.ThemeAura,
		Database:      true,
		E2E:           true,
		Coverage:      true,
		Traceable:     true,
		AppPort:       49100,
		DatabasePort:  49200,
		AuthPort:      49300,
	}
	c.ProjectName = config.DeriveProjectName(c.ArtifactID)
	c.Package = config.DerivePackage(c.GroupID, c.ArtifactID)
	c.OutputDir = c.ArtifactID
	return c
}

func lookedUp() VersionSource {
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
	options := conversation()
	options.Accessible = true
	options.Input = strings.NewReader(answers)
	options.Output = io.Discard
	session, err := Run(seed(), lookedUp(), options)
	return session.Config, err
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
	if got.BootVersion != "4.1.0" {
		t.Errorf("Boot version = %q, want the release the Vaadin was built with", got.BootVersion)
	}
	if got.JavaVersion != "21" {
		t.Errorf("Java version = %q, want the default, which the pair allows", got.JavaVersion)
	}
	if err := got.Validate(rules); err != nil {
		t.Errorf("the accepted defaults do not validate: %v", err)
	}
}

// A version typed over the offered one is checked against the rules at the
// field, so the conversation keeps asking rather than carrying an incompatible
// set forward to fail after the last question.
func TestAnIncompatibleTypedVersionIsRefusedAtTheField(t *testing.T) {
	// Coordinates and identity accepted, then: a Vaadin from a line this tool
	// does not generate, refused, then the offered one; a Boot older than that
	// Vaadin needs, refused, then a newer one; a Java below the floor, refused,
	// then one in range.
	answers := strings.Repeat("\n", 5) +
		"24.10.9\n\n" +
		"4.0.8\n4.1.1\n" +
		"17\n25\n" +
		strings.Repeat("\n", 30)

	got, err := run(t, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.VaadinVersion != "25.2.6" || got.BootVersion != "4.1.1" || got.JavaVersion != "25" {
		t.Errorf("versions = %s / %s / %s; the refused answers should not have been kept",
			got.VaadinVersion, got.BootVersion, got.JavaVersion)
	}
	if err := got.Validate(rules); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}
}

// The Java default follows the pair: a defaults file naming a JDK the chosen
// Vaadin does not run on opens on the newest LTS release it does.
func TestAnOutOfRangeJavaDefaultSnaps(t *testing.T) {
	c := seed()
	c.JavaVersion = "17"
	options := conversation()
	options.Accessible = true
	options.Input = strings.NewReader(strings.Repeat("\n", 40))
	options.Output = io.Discard

	session, err := Run(c, lookedUp(), options)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if session.Config.JavaVersion != "25" {
		t.Errorf("Java version = %q, want the newest LTS in range", session.Config.JavaVersion)
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

	blanks := strings.Repeat("\n", 9) // through to the stack question
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

// The theme is asked at the head of the stack, as a numbered list; its number
// reaches the Config, and a blank line keeps the default.
func TestTheThemeIsAskedAndReachesTheConfig(t *testing.T) {
	position := func(theme string) string {
		for i, option := range themeOptions(rules, "25.2.6", config.ThemeAura) {
			if option.Value == theme {
				return strconv.Itoa(i + 1)
			}
		}
		t.Fatalf("no option for %q", theme)
		return ""
	}

	upToTheTheme := strings.Repeat("\n", 8) // coordinates, identity, versions
	got, err := run(t, upToTheTheme+position(config.ThemeLumo)+"\n"+"0\n"+"\n\n")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Theme != config.ThemeLumo {
		t.Errorf("theme = %q, want the one chosen", got.Theme)
	}
	if err := got.Validate(rules); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}

	got, err = run(t, strings.Repeat("\n", 40))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Theme != config.ThemeAura {
		t.Errorf("theme = %q, want the seeded default kept", got.Theme)
	}
}

// The lists the screen derives, on their own: what Boot is offered for a Vaadin,
// and what Java for the pair.
func TestBootChoices(t *testing.T) {
	fetched := []string{"4.1.3", "4.1.2", "4.1.1", "4.1.0", "4.0.8", "4.0.7"}

	list, open := bootChoices(rules, "25.2.6", fetched, "4.1.0", "4.1.1")
	if want := "4.1.3 4.1.2 4.1.1 4.1.0"; strings.Join(list, " ") != want || open != "4.1.0" {
		t.Errorf("with a pin: list %v opens on %s; want %s opening on the pin", list, open, want)
	}

	// The pin is in the list even when it is not among the newest offered.
	many := []string{"4.1.7", "4.1.6", "4.1.5", "4.1.4", "4.1.3", "4.1.2", "4.1.1", "4.1.0"}
	list, open = bootChoices(rules, "25.2.6", many, "4.1.0", "4.1.7")
	if len(list) != offered+1 || list[len(list)-1] != "4.1.0" || open != "4.1.0" {
		t.Errorf("an old pin should be added to the list: %v opens on %s", list, open)
	}

	list, open = bootChoices(rules, "25.2.6", fetched, "", "4.1.1")
	if open != "4.1.3" || len(list) != 4 {
		t.Errorf("without a pin: %v opens on %s; want the newest compatible", list, open)
	}

	// Offline: the fallback, and only the fallback.
	list, open = bootChoices(rules, "25.2.6", nil, "", "4.1.1")
	if len(list) != 1 || list[0] != "4.1.1" || open != "4.1.1" {
		t.Errorf("offline: %v opens on %s; want the fallback alone", list, open)
	}
}

func TestJavaChoices(t *testing.T) {
	// The LTS releases in the range and the newest in it: what the range comes
	// to for anyone starting a project; --java-version names the rest.
	list, open, note := javaChoices(rules, "25.2.6", "4.1.0", "21")
	if want := "21 25 26"; strings.Join(list, " ") != want {
		t.Errorf("list = %v, want %s", list, want)
	}
	if open != "21" {
		t.Errorf("opens on %s, want the default, which is in range", open)
	}
	if note != "21 or newer for Vaadin 25, up to 26 for Spring Boot 4.1." {
		t.Errorf("note = %q", note)
	}

	// A default that is in range but not LTS is offered too, since it is where
	// the cursor opens.
	list, _, _ = javaChoices(rules, "25.2.6", "4.1.0", "23")
	if want := "21 23 25 26"; strings.Join(list, " ") != want {
		t.Errorf("list = %v, want %s", list, want)
	}

	_, open, note = javaChoices(rules, "25.2.6", "4.1.0", "17")
	if open != "25" || !strings.Contains(note, "The default, 17, is outside that.") {
		t.Errorf("an out-of-range default should snap to the newest LTS and say so: %s, %q", open, note)
	}

	// A pair the rules refuse offers nothing and says why, without the source
	// cited under it.
	list, _, note = javaChoices(rules, "25.2.6", "4.0.8", "21")
	if list != nil || !strings.Contains(note, "needs Spring Boot 4.1.0 or newer") || strings.Contains(note, "\n") {
		t.Errorf("an incompatible pair: list %v, note %q", list, note)
	}
}

// The command bar takes a task name and hands it back, trimmed.
func TestTheCommandBarTakesATaskName(t *testing.T) {
	got, err := Task(Options{
		Accessible: true,
		Input:      strings.NewReader("  verify  \n"),
		Output:     io.Discard,
	})
	if err != nil {
		t.Fatalf("Task: %v", err)
	}
	if got != "verify" {
		t.Errorf("task = %q, want the name that was typed", got)
	}
}

// Read out to a screen reader, the bar is one more question in a conversation
// where enter has meant "that is fine" every time, so a bare enter is taken the
// same way — as is saying so.
func TestTheCommandBarTakesNoAnswer(t *testing.T) {
	for _, answer := range []string{"\n", "quit\n", "exit\n"} {
		got, err := Task(Options{
			Accessible: true,
			Input:      strings.NewReader(answer),
			Output:     io.Discard,
		})
		if err != nil {
			t.Fatalf("Task(%q): %v", answer, err)
		}
		if got != "" {
			t.Errorf("task = %q for %q, want nothing", got, answer)
		}
	}
}

// runAsking is run with the author question added, the way main adds it when
// git has no identity to commit with.
func runAsking(t *testing.T, c config.Config, answers string) (config.Config, error) {
	t.Helper()
	options := conversation()
	options.Accessible = true
	options.AskAuthor = true
	options.Input = strings.NewReader(answers)
	options.Output = io.Discard
	session, err := Run(c, lookedUp(), options)
	return session.Config, err
}

// upToTheAuthor accepts every answer before the author question: the two
// coordinates, the three identity answers, the three versions, the theme, and
// the stack — which, being a multi-select, is confirmed with a 0 rather than a
// blank line.
const upToTheAuthor = "\n\n" + "\n\n\n" + "\n\n\n" + "\n" + "0\n"

// Who the first commit is by is asked when git cannot say, and with nothing to
// fall back on the question keeps being asked until it has an answer: the blank
// line that accepts every other default is refused here.
func TestTheAuthorIsAskedForWhenGitHasNone(t *testing.T) {
	answers := upToTheAuthor + "\nAnn Example\nnot an email\nann@example.invalid\n" + strings.Repeat("\n", 10)

	got, err := runAsking(t, seed(), answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.AuthorName != "Ann Example" {
		t.Errorf("author name = %q, want the one typed", got.AuthorName)
	}
	if got.AuthorEmail != "ann@example.invalid" {
		t.Errorf("author email = %q, want the one typed after the refused one", got.AuthorEmail)
	}
	if err := got.Validate(rules); err != nil {
		t.Errorf("the answers do not validate: %v", err)
	}
}

// The half git did have is offered back, and a blank line keeps it.
func TestAnAuthorHalfKnownIsOfferedBack(t *testing.T) {
	c := seed()
	c.AuthorName = "Ann Example"
	answers := upToTheAuthor + "\nann@example.invalid\n" + strings.Repeat("\n", 10)

	got, err := runAsking(t, c, answers)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.AuthorName != "Ann Example" {
		t.Errorf("author name = %q, want the offered one kept", got.AuthorName)
	}
	if got.AuthorEmail != "ann@example.invalid" {
		t.Errorf("author email = %q, want the one typed", got.AuthorEmail)
	}
}

// And when git knows already, nobody is asked: the same blank lines produce a
// Config that names no author, so nothing is written to the repository's config.
func TestTheAuthorIsNotAskedForWhenGitKnows(t *testing.T) {
	got, err := run(t, strings.Repeat("\n", 40))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.AuthorName != "" || got.AuthorEmail != "" {
		t.Errorf("author = %q <%q>, want none", got.AuthorName, got.AuthorEmail)
	}
}
