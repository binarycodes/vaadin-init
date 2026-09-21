package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shipped file, which is also the file the plan's examples are written
// against.
func shipped(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile("../../compat.json")
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func rules(t *testing.T) Rules {
	t.Helper()
	r, err := Parse(shipped(t))
	if err != nil {
		t.Fatalf("the shipped compat.json does not load: %v", err)
	}
	return r
}

// An explicit path is an instruction; the per-user path is a convention.
func TestAMissingExplicitFileIsAnError(t *testing.T) {
	if _, err := Load(shipped(t), filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("an explicit path that is missing should be an error")
	}
}

func TestTheShippedRulesNameOneSupportedLine(t *testing.T) {
	vaadin, boot := rules(t).Supported()
	if len(vaadin) != 1 || vaadin[0] != "25" {
		t.Errorf("supported Vaadin lines = %v, want [25]", vaadin)
	}
	if len(boot) != 1 || boot[0] != "4" {
		t.Errorf("supported Boot lines = %v, want [4]", boot)
	}
}

// A line the file knows but this tool has no templates for is refused with the
// reason, and one the file has never heard of is refused too.
func TestVaadinRefusesLinesThisToolDoesNotGenerate(t *testing.T) {
	r := rules(t)
	for _, v := range []string{"24.10.9", "26.0.0", "23.3.0"} {
		if _, err := r.Vaadin(v); err == nil {
			t.Errorf("Vaadin(%q) accepted", v)
		}
	}
	_, err := r.Vaadin("24.10.9")
	for _, want := range []string{"Vaadin 25", "24.10.9", "Spring Boot 3", "vaadin.com/docs/v24"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Vaadin(24.10.9) = %q, want it to mention %q", err, want)
		}
	}
	if _, err := r.Vaadin("25.2.6"); err != nil {
		t.Errorf("Vaadin(25.2.6) = %v", err)
	}
	if _, err := r.Vaadin("garbage"); err == nil {
		t.Error("Vaadin(garbage) accepted")
	}
}

func TestBootMinAppliesSince(t *testing.T) {
	r := rules(t)
	for vaadin, want := range map[string]string{
		"25.0.5": "4.0.0",
		"25.1.0": "4.0.4",
		"25.1.7": "4.0.4",
		"25.2.0": "4.1.0",
		"25.2.6": "4.1.0",
		"25.3.0": "4.1.0",
	} {
		got, err := r.BootMin(vaadin)
		if err != nil {
			t.Errorf("BootMin(%s): %v", vaadin, err)
			continue
		}
		if got != want {
			t.Errorf("BootMin(%s) = %s, want %s", vaadin, got, want)
		}
	}
	if _, err := r.BootMin("24.10.9"); err == nil {
		t.Error("BootMin of an unsupported line should be an error")
	}
}

func TestCompatibleBootKeepsTheLineAndTheMinimum(t *testing.T) {
	r := rules(t)
	candidates := []string{"4.2.0", "4.1.1", "4.1.0", "4.0.8", "4.0.4", "4.0.0", "3.5.15"}

	got := r.CompatibleBoot("25.2.6", candidates)
	if want := "4.2.0 4.1.1 4.1.0"; strings.Join(got, " ") != want {
		t.Errorf("CompatibleBoot(25.2.6) = %v, want %s", got, want)
	}
	got = r.CompatibleBoot("25.1.0", candidates)
	if want := "4.2.0 4.1.1 4.1.0 4.0.8 4.0.4"; strings.Join(got, " ") != want {
		t.Errorf("CompatibleBoot(25.1.0) = %v, want %s", got, want)
	}
	if got := r.CompatibleBoot("24.10.9", candidates); got != nil {
		t.Errorf("CompatibleBoot of an unsupported line = %v, want nothing", got)
	}
}

func TestJavaRange(t *testing.T) {
	r := rules(t)

	floor, ceiling, err := r.JavaRange("25.2.6", "4.1.0")
	if err != nil || floor != 21 || ceiling != 26 {
		t.Errorf("JavaRange(25.2.6, 4.1.0) = %d-%d, %v; want 21-26", floor, ceiling, err)
	}

	// The pair is refused before Java is looked at.
	if _, _, err := r.JavaRange("25.2.6", "4.0.8"); err == nil {
		t.Error("JavaRange(25.2.6, 4.0.8) should fail on the pair")
	}

	// A Boot minor the file has no rule for has no ceiling — nobody has written
	// one down — but still Vaadin's floor.
	floor, ceiling, err = r.JavaRange("25.2.6", "4.2.0")
	if err != nil || floor != 21 || ceiling != 0 {
		t.Errorf("JavaRange(25.2.6, 4.2.0) = %d-%d, %v; want 21 and no ceiling", floor, ceiling, err)
	}
	if got := r.NewestJava(); got != 26 {
		t.Errorf("NewestJava = %d, want 26", got)
	}
}

func TestCheck(t *testing.T) {
	r := rules(t)
	cases := []struct {
		vaadin, boot string
		java         int
		theme        string
		wantErr      string // a fragment, or "" for accepted
	}{
		{"25.2.6", "4.1.0", 21, "aura", ""},
		{"25.2.6", "4.1.1", 25, "aura", ""},
		{"25.2.6", "4.1.0", 26, "aura", ""}, // java_max is inclusive
		{"25.2.6", "4.1.0", 27, "aura", "java version: Spring Boot 4.1 supports Java up to 26; got 27"},
		{"25.2.6", "4.1.0", 17, "aura", "java version: Vaadin 25 needs Java 21 or newer; got 17"},
		{"25.2.6", "4.0.8", 21, "aura", "spring boot version: Vaadin 25.2.6 needs Spring Boot 4.1.0 or newer; got 4.0.8"},
		{"25.1.0", "4.0.0", 21, "aura", "spring boot version: Vaadin 25.1.0 needs Spring Boot 4.0.4 or newer; got 4.0.0"},
		{"25.0.5", "4.0.0", 21, "aura", ""},
		{"25.2.6", "3.5.15", 21, "aura", "spring boot version: Vaadin 25 sits on Spring Boot 4; got 3.5.15"},
		{"24.10.9", "3.5.15", 17, "aura", "vaadin version: this tool generates Vaadin 25 projects"},
		{"24.10.9", "4.1.0", 21, "aura", "vaadin version:"},
		{"25.2.6", "4.2.0", 30, "aura", ""}, // no rule for 4.2 yet, so no ceiling to break
		{"25.2.6", "4.1.0", 21, "lumo", ""},
		{"25.2.6", "4.1.0", 21, "material", "theme: Vaadin 25 ships aura and lumo, not material"},
	}
	for _, c := range cases {
		err := r.Check(c.vaadin, c.boot, c.java, c.theme)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("Check(%s, %s, %d, %s) = %v, want accepted", c.vaadin, c.boot, c.java, c.theme, err)
		case c.wantErr != "" && err == nil:
			t.Errorf("Check(%s, %s, %d, %s) accepted, want %q", c.vaadin, c.boot, c.java, c.theme, c.wantErr)
		case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
			t.Errorf("Check(%s, %s, %d, %s) = %q, want %q", c.vaadin, c.boot, c.java, c.theme, err, c.wantErr)
		}
	}

	// The source is quoted so the fix is a read and an edit.
	err := r.Check("25.2.6", "4.0.8", 21, "aura")
	if err == nil || !strings.Contains(err.Error(), "https://github.com/vaadin/platform/releases/tag/25.2.0") {
		t.Errorf("Check should cite the rule's source: %v", err)
	}
}

func TestBootDefault(t *testing.T) {
	r := rules(t)
	fetched := []string{"4.1.1", "4.1.0", "4.0.8"}
	if got := r.BootDefault("25.2.6", "4.1.0", fetched); got != "4.1.0" {
		t.Errorf("with a pin = %q, want the pin", got)
	}
	if got := r.BootDefault("25.2.6", "", fetched); got != "4.1.1" {
		t.Errorf("without a pin = %q, want the newest compatible", got)
	}
	// A pin the rules refuse is not offered: the rules are what the validator
	// applies, and the check job is what reconciles the two.
	if got := r.BootDefault("25.2.6", "4.0.8", fetched); got != "4.1.1" {
		t.Errorf("with an incompatible pin = %q, want the newest compatible", got)
	}
	if got := r.BootDefault("25.2.6", "", nil); got != "" {
		t.Errorf("with nothing = %q, want nothing", got)
	}
	if got := r.BootDefault("24.10.9", "3.5.15", []string{"3.5.15"}); got != "" {
		t.Errorf("for an unsupported line = %q, want nothing", got)
	}
}

// The bare checks carry no field prefix, for the field that asks the question.
func TestBareChecks(t *testing.T) {
	r := rules(t)
	if err := r.CheckBoot("25.2.6", "4.0.8"); err == nil || strings.HasPrefix(err.Error(), "spring boot version") {
		t.Errorf("CheckBoot = %v", err)
	}
	if err := r.CheckJava("25.2.6", "4.1.0", 17); err == nil || strings.HasPrefix(err.Error(), "java version") {
		t.Errorf("CheckJava = %v", err)
	}
	if err := r.CheckJava("25.2.6", "4.0.8", 21); err == nil || !strings.Contains(err.Error(), "4.1.0 or newer") {
		t.Errorf("CheckJava should refuse the pair first: %v", err)
	}
}

// Aura is a Vaadin 25 theme; 24 ships Lumo alone.
func TestThemes(t *testing.T) {
	r := rules(t)
	if got := strings.Join(r.Themes("25.2.6"), " "); got != "aura lumo" {
		t.Errorf("Themes(25.2.6) = %q", got)
	}
	if got := r.Themes("24.10.9"); got != nil {
		t.Errorf("Themes of an unsupported line = %v, want nothing", got)
	}
	if err := r.CheckTheme("25.2.6", "lumo"); err != nil {
		t.Errorf("CheckTheme(25.2.6, lumo) = %v", err)
	}
	if err := r.CheckTheme("25.2.6", "material"); err == nil || !strings.Contains(err.Error(), "ships aura and lumo, not material") {
		t.Errorf("CheckTheme(25.2.6, material) = %v", err)
	}
	if got := r.ThemeDefault("25.2.6", "lumo"); got != "lumo" {
		t.Errorf("ThemeDefault keeps a theme the line ships: %q", got)
	}
	if got := r.ThemeDefault("25.2.6", "material"); got != "aura" {
		t.Errorf("ThemeDefault falls back to the line's own: %q", got)
	}
	if got := r.ThemeDefault("24.10.9", "aura"); got != "" {
		t.Errorf("ThemeDefault of an unsupported line = %q, want nothing", got)
	}
}

func TestJavaDefault(t *testing.T) {
	r := rules(t)
	cases := []struct {
		floor, ceiling, preferred int
		want                      int
		snapped                   bool
	}{
		{21, 26, 21, 21, false},
		{21, 26, 25, 25, false},
		{21, 26, 17, 25, true}, // below the floor: newest LTS in range
		{21, 26, 27, 25, true}, // above the ceiling
		{26, 26, 21, 26, true}, // no LTS in range: the floor
		{21, 0, 30, 30, false}, // no ceiling written down
	}
	for _, c := range cases {
		got, snapped := r.JavaDefault(c.floor, c.ceiling, c.preferred)
		if got != c.want || snapped != c.snapped {
			t.Errorf("JavaDefault(%d, %d, %d) = %d, %v; want %d, %v",
				c.floor, c.ceiling, c.preferred, got, snapped, c.want, c.snapped)
		}
	}
	if !r.LTS(21) || r.LTS(22) {
		t.Error("LTS should know 21 and not 22")
	}
}

func TestDescribe(t *testing.T) {
	r := rules(t)
	if got := r.Describe("25.2.6", "4.1.0"); got != "21 or newer for Vaadin 25, up to 26 for Spring Boot 4.1." {
		t.Errorf("Describe = %q", got)
	}
	if got := r.Describe("25.2.6", "4.2.0"); got != "21 or newer for Vaadin 25." {
		t.Errorf("Describe with no Boot rule = %q", got)
	}
}

// An override layered over the embedded file wins per line and leaves the
// other lines alone.
func TestAnOverrideWinsPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compat.json")
	override := `{
	  "vaadin": [
	    {"line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "4.1.0",
	     "since": [{"vaadin": "25.3.0", "boot_min": "4.2.0"}], "themes": ["aura"], "source": "local"}
	  ],
	  "boot": [
	    {"line": "4.2", "java_min": 17, "java_max": 27, "eol": "2028-01-31", "source": "local"}
	  ],
	  "java": {"lts": [21, 25]}
	}`
	if err := os.WriteFile(path, []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(shipped(t), path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, _ := r.BootMin("25.3.0"); got != "4.2.0" {
		t.Errorf("BootMin(25.3.0) = %s, want the override's since entry", got)
	}
	if got, _ := r.BootMin("25.0.0"); got != "4.1.0" {
		t.Errorf("BootMin(25.0.0) = %s, want the override's boot_min, not the embedded one", got)
	}
	if _, err := r.Vaadin("24.10.9"); err == nil || !strings.Contains(err.Error(), "Spring Boot 3") {
		t.Errorf("the embedded 24 rule should survive an override that does not name it: %v", err)
	}
	if _, ceiling, _ := r.JavaRange("25.3.0", "4.2.0"); ceiling != 27 {
		t.Errorf("a Boot line added by the override should be known: ceiling %d", ceiling)
	}
	if _, ceiling, _ := r.JavaRange("25.2.0", "4.1.0"); ceiling != 26 {
		t.Errorf("an embedded Boot line should survive: ceiling %d", ceiling)
	}
	if r.LTS(17) {
		t.Error("the override's LTS list should replace the embedded one")
	}
}

// A file with a mistake in it is refused naming the entry, so the person who
// made the edit can find it.
func TestAMalformedFileIsRejectedNamingTheEntry(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{
			"boot_min unparsable",
			`{"vaadin": [{"line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "four", "themes": ["aura"], "source": ""}],
			  "boot": [], "java": {"lts": [21]}}`,
			`vaadin line "25": boot_min: "four" is not a version`,
		},
		{
			"boot_min outside its line",
			`{"vaadin": [{"line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "3.5.0", "themes": ["aura"], "source": ""}],
			  "boot": [], "java": {"lts": [21]}}`,
			`boot_min: "3.5.0" is not in line 4`,
		},
		{
			"since outside its line",
			`{"vaadin": [{"line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "4.0.0",
			   "since": [{"vaadin": "24.1.0", "boot_min": "4.0.4"}], "themes": ["aura"], "source": ""}],
			  "boot": [], "java": {"lts": [21]}}`,
			`since: vaadin: "24.1.0" is not in line 25`,
		},
		{
			"an unknown key",
			`{"vaadin": [{"line": "25", "supported": true, "java_mim": 21, "boot_line": "4", "boot_min": "4.0.0", "themes": ["aura"], "source": ""}],
			  "boot": [], "java": {"lts": [21]}}`,
			`java_mim`,
		},
		{
			"a boot range upside down",
			`{"vaadin": [], "boot": [{"line": "4.1", "java_min": 27, "java_max": 26, "eol": "", "source": ""}], "java": {"lts": [21]}}`,
			`boot line "4.1": java_min 27 and java_max 26 do not make a range`,
		},
		{
			"an eol that is not a date",
			`{"vaadin": [], "boot": [{"line": "4.1", "java_min": 17, "java_max": 26, "eol": "next summer", "source": ""}], "java": {"lts": [21]}}`,
			`eol "next summer" is not a date`,
		},
		{
			"a line listed twice",
			`{"vaadin": [], "boot": [{"line": "4.1", "java_min": 17, "java_max": 26, "eol": "", "source": ""},
			  {"line": "4.1", "java_min": 17, "java_max": 26, "eol": "", "source": ""}], "java": {"lts": [21]}}`,
			`boot line "4.1": listed twice`,
		},
		{
			"no themes",
			`{"vaadin": [{"line": "25", "supported": true, "java_min": 21, "boot_line": "4", "boot_min": "4.0.0", "source": ""}],
			  "boot": [], "java": {"lts": [21]}}`,
			`vaadin line "25": themes must name at least one theme`,
		},
		{
			"no lts list",
			`{"vaadin": [], "boot": [], "java": {"lts": []}}`,
			`lts must name at least one release`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "compat.json")
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(shipped(t), path)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), path) {
				t.Errorf("error = %q, want it to name %q and the file", err, c.want)
			}
		})
	}
}
