package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/binarycodes/vaadin-init/internal/compat"
)

func TestValidGroupID(t *testing.T) {
	valid := []string{"com.example", "io.binarycodes", "com.example.tools", "a", "com.example.my_app"}
	invalid := []string{"", "Com.Example", "com..example", ".com", "com.example.", "com example", "1com.example"}

	for _, s := range valid {
		if err := ValidGroupID(s); err != nil {
			t.Errorf("ValidGroupID(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := ValidGroupID(s); err == nil {
			t.Errorf("ValidGroupID(%q) = nil, want an error", s)
		}
	}
}

func TestValidArtifactID(t *testing.T) {
	valid := []string{"my-app", "app", "note-harbor", "a1", "a-1-b"}
	invalid := []string{"", "My-App", "-app", "app-", "my--app", "my_app", "my.app"}

	for _, s := range valid {
		if err := ValidArtifactID(s); err != nil {
			t.Errorf("ValidArtifactID(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := ValidArtifactID(s); err == nil {
			t.Errorf("ValidArtifactID(%q) = nil, want an error", s)
		}
	}
}

// A package segment that is a Java keyword compiles nowhere, so the tool has to
// be the one that says so.
func TestValidPackageRejectsKeywords(t *testing.T) {
	for _, s := range []string{"com.new.app", "com.example.class", "com.static"} {
		if err := ValidPackage(s); err == nil {
			t.Errorf("ValidPackage(%q) = nil, want an error", s)
		}
	}
	if err := ValidPackage("com.example.myapp"); err != nil {
		t.Errorf("ValidPackage(com.example.myapp) = %v, want nil", err)
	}
}

// The shape only: which majors are allowed depends on the other two versions,
// and is the rules' question rather than this one's.
func TestValidJavaVersion(t *testing.T) {
	for _, s := range []string{"17", "21", "25", "31"} {
		if err := ValidJavaVersion(s); err != nil {
			t.Errorf("ValidJavaVersion(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", "0", "-1", "21.0.2", "twenty-one"} {
		if err := ValidJavaVersion(s); err == nil {
			t.Errorf("ValidJavaVersion(%q) = nil, want an error", s)
		}
	}
}

// rules is the shipped compat.json, which is what every Config is validated
// against unless the user has their own.
func rules(t *testing.T) compat.Rules {
	t.Helper()
	content, err := os.ReadFile("../../compat.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := compat.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The three versions are validated together: a Config whose fields are each fine
// on their own and wrong as a set is the one nothing else catches, since the
// flags never pass through a prompt.
func TestValidateChecksTheVersionsTogether(t *testing.T) {
	base := func() Config {
		var d Defaults
		d.GroupID, d.ArtifactID, d.Theme = "com.example", "my-app", ThemeAura
		d.Ports.From, d.Ports.To = 49000, 51000
		c := d.ToConfig()
		c.VaadinVersion, c.BootVersion, c.JavaVersion = "25.2.6", "4.1.0", "21"
		return c
	}
	cases := []struct {
		name                string
		vaadin, boot, java  string
		accepted            bool
		wantField, wantRule string
	}{
		{"the pinned pair on the default JDK", "25.2.6", "4.1.0", "21", true, "", ""},
		{"the pinned pair on the newest LTS", "25.2.6", "4.1.0", "25", true, "", ""},
		{"a Boot older than the minor needs", "25.2.6", "4.0.8", "21", false, "spring boot version", "needs Spring Boot 4.1.0 or newer; got 4.0.8"},
		{"a JDK below Vaadin's floor", "25.2.6", "4.1.0", "17", false, "java version", "Vaadin 25 needs Java 21 or newer; got 17"},
		{"a JDK above Boot's ceiling", "25.2.6", "4.1.0", "27", false, "java version", "Spring Boot 4.1 supports Java up to 26; got 27"},
		{"a Vaadin line this tool does not generate", "24.10.9", "3.5.15", "17", false, "vaadin version", "this tool generates Vaadin 25 projects"},
		{"the wrong line on a right-looking Boot", "24.10.9", "4.1.0", "21", false, "vaadin version", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := base()
			cfg.VaadinVersion, cfg.BootVersion, cfg.JavaVersion = c.vaadin, c.boot, c.java
			err := cfg.Validate(rules(t))
			if c.accepted {
				if err != nil {
					t.Fatalf("Validate = %v, want accepted", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate accepted")
			}
			if !strings.HasPrefix(err.Error(), c.wantField+":") || !strings.Contains(err.Error(), c.wantRule) {
				t.Errorf("Validate = %q, want %q naming %q", err, c.wantField, c.wantRule)
			}
		})
	}
}

func TestDeriveProjectName(t *testing.T) {
	cases := map[string]string{
		"my-app":      "My App",
		"note-harbor": "Note Harbor",
		"shelf":       "Shelf",
		"a-b-c":       "A B C",
	}
	for in, want := range cases {
		if got := DeriveProjectName(in); got != want {
			t.Errorf("DeriveProjectName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDerivePackage(t *testing.T) {
	cases := []struct {
		group, artifact, want string
	}{
		{"com.example", "my-app", "com.example.myapp"},
		{"io.binarycodes", "note-harbor", "io.binarycodes.noteharbor"},
		// Already suffixed: appending again would give com.example.shelf.shelf,
		// which every user would then have to edit back.
		{"com.example.shelf", "shelf", "com.example.shelf"},
		{"com.example", "app-2", "com.example.app2"},
	}
	for _, c := range cases {
		got := DerivePackage(c.group, c.artifact)
		if got != c.want {
			t.Errorf("DerivePackage(%q, %q) = %q, want %q", c.group, c.artifact, got, c.want)
		}
		// Whatever it derives has to be a package javac accepts, since the
		// non-interactive path never shows it to anyone.
		if err := ValidPackage(got); err != nil {
			t.Errorf("DerivePackage(%q, %q) = %q, which is not a valid package: %v", c.group, c.artifact, got, err)
		}
	}
}

// The derived answers have to be valid, since the non-interactive path never
// puts them in front of a validator the user can see.
func TestDefaultsProduceAValidConfig(t *testing.T) {
	var d Defaults
	d.GroupID = "com.example"
	d.ArtifactID = "my-app"
	d.JavaVersion = "21"
	d.VaadinVersion = "25.2.6"
	d.BootVersion = "4.1.1"
	d.Theme = ThemeAura
	d.Ports.From, d.Ports.To = 49000, 51000

	if err := d.ToConfig().Validate(rules(t)); err != nil {
		t.Fatalf("the defaults do not produce a valid config: %v", err)
	}
}

// The shipped defaults file has to reach a valid Config with every key carried
// over: a key added to the file and forgotten in ToConfig would otherwise fall to
// its zero value and be caught, if at all, by whoever generates the next project.
func TestTheShippedDefaultsReachTheConfig(t *testing.T) {
	embedded, err := os.ReadFile("../../defaults.toml")
	if err != nil {
		t.Fatal(err)
	}
	var d Defaults
	if err := toml.Unmarshal(embedded, &d); err != nil {
		t.Fatalf("defaults.toml is not valid TOML: %v", err)
	}
	c := d.ToConfig()
	if err := c.Validate(rules(t)); err != nil {
		t.Fatalf("the shipped defaults do not produce a valid config: %v", err)
	}
	if c.Theme != ThemeAura {
		t.Errorf("theme = %q, want the Vaadin 25 default", c.Theme)
	}
}

// A theme is one of two, and always one: an empty theme would render an
// Application with no stylesheet at all, which compiles and looks broken.
func TestValidTheme(t *testing.T) {
	for _, s := range []string{ThemeAura, ThemeLumo} {
		if err := ValidTheme(s); err != nil {
			t.Errorf("ValidTheme(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", "Aura", "material", "lumo "} {
		if err := ValidTheme(s); err == nil {
			t.Errorf("ValidTheme(%q) = nil, want an error", s)
		}
	}

	c := Config{Theme: ThemeAura}
	if !c.Aura() || c.Lumo() || c.ThemeName() != "Aura" {
		t.Errorf("an aura config reports Aura=%v Lumo=%v name=%q", c.Aura(), c.Lumo(), c.ThemeName())
	}
	c.Theme = ThemeLumo
	if c.Aura() || !c.Lumo() || c.ThemeName() != "Lumo" {
		t.Errorf("a lumo config reports Aura=%v Lumo=%v name=%q", c.Aura(), c.Lumo(), c.ThemeName())
	}
}

func TestFeaturesDriveTheDerivedAnswers(t *testing.T) {
	c := Config{ArtifactID: "my-app"}

	if c.ContainerRequired() {
		t.Error("a project with no database and no auth should need no container runtime")
	}
	c.E2E = true
	if c.ContainerRequired() {
		t.Error("browser tests bring their own browser, so they should not require a container runtime")
	}
	if !c.BrowserTests() {
		t.Error("e2e without auth should generate the browser test")
	}
	c.Auth = true
	if c.BrowserTests() {
		t.Error("e2e with auth should not generate a browser test that needs an identity provider")
	}
	if !c.ProtectedRootTest() {
		t.Error("e2e with auth should generate the redirect test instead")
	}
	if !c.ContainerRequired() {
		t.Error("auth puts an identity provider in the dev stack, so a runtime is required")
	}
	if c.ITProfile() == "" {
		t.Error("a project with integration tests needs the production-mode profile")
	}
}

// The author of the first commit is git's business unless git has none, so a
// Config without one is complete — but one that names an author has to name a
// usable one.
func TestAuthorIsOptionalButChecked(t *testing.T) {
	c := Config{
		GroupID: "com.example", ArtifactID: "my-app", ProjectName: "My App",
		Package: "com.example.myapp", JavaVersion: "21",
		VaadinVersion: "25.2.6", BootVersion: "4.1.1", Theme: ThemeAura, OutputDir: "my-app",
		AppPort: 49100, DatabasePort: 49200, AuthPort: 49300,
	}
	if err := c.Validate(rules(t)); err != nil {
		t.Fatalf("a config with no author should validate: %v", err)
	}

	c.AuthorName, c.AuthorEmail = "Ann Example", "ann@example.invalid"
	if err := c.Validate(rules(t)); err != nil {
		t.Errorf("a config with an author should validate: %v", err)
	}

	c.AuthorEmail = "ann"
	if err := c.Validate(rules(t)); err == nil {
		t.Error("an email with no @ should be refused")
	}
}

func TestValidAuthorEmail(t *testing.T) {
	valid := []string{"ann@example.invalid", "a.b+c@sub.example.org", "me@localhost"}
	invalid := []string{"", "   ", "ann", "@example.org", "ann@", "ann example@x.org", "<ann@example.org>"}

	for _, s := range valid {
		if err := ValidAuthorEmail(s); err != nil {
			t.Errorf("ValidAuthorEmail(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range invalid {
		if err := ValidAuthorEmail(s); err == nil {
			t.Errorf("ValidAuthorEmail(%q) = nil, want an error", s)
		}
	}
}

// Three projects on one machine need three different ports each, and the range is
// where they all come from — so the draw has to stay inside it, never repeat, and
// step around whatever is already listening.
func TestPickPortsStaysInRangeAndAvoidsWhatIsBusy(t *testing.T) {
	busy := map[int]bool{49000: true, 49001: true, 49002: true}
	original := portFree
	portFree = func(port int) bool { return !busy[port] }
	t.Cleanup(func() { portFree = original })

	for range 50 {
		ports := PickPorts(49000, 49006, 3)
		if len(ports) != 3 {
			t.Fatalf("PickPorts returned %d ports, want 3", len(ports))
		}
		seen := map[int]bool{}
		for _, p := range ports {
			if p < 49000 || p > 49006 {
				t.Errorf("port %d is outside the range", p)
			}
			if busy[p] {
				t.Errorf("port %d is busy and was picked while free ones remained", p)
			}
			if seen[p] {
				t.Errorf("port %d was picked twice", p)
			}
			seen[p] = true
		}
	}

	// With too few free ports the draw still fills up, from the busy ones: the
	// project is not running yet, and a Config has to be produced.
	ports := PickPorts(49000, 49003, 3)
	if len(ports) != 3 {
		t.Errorf("with one free port, PickPorts returned %d ports, want 3", len(ports))
	}
}

func TestPortsMustBeUnprivilegedAndDistinct(t *testing.T) {
	for _, n := range []int{1024, 49000, 65535} {
		if err := ValidPort(n); err != nil {
			t.Errorf("ValidPort(%d) = %v, want nil", n, err)
		}
	}
	for _, n := range []int{0, 80, 1023, 65536, -1} {
		if err := ValidPort(n); err == nil {
			t.Errorf("ValidPort(%d) = nil, want an error", n)
		}
	}

	c := Config{
		GroupID: "com.example", ArtifactID: "my-app", ProjectName: "My App",
		Package: "com.example.myapp", JavaVersion: "21",
		VaadinVersion: "25.2.6", BootVersion: "4.1.1", Theme: ThemeAura, OutputDir: "my-app",
		AppPort: 49100, DatabasePort: 49100, AuthPort: 49300,
	}
	if err := c.Validate(rules(t)); err == nil {
		t.Error("two pieces on the same port should be refused")
	}
	c.DatabasePort = 49200
	if err := c.Validate(rules(t)); err != nil {
		t.Errorf("three distinct ports should validate: %v", err)
	}
}

// A personal defaults file that names a range no three ports fit in is stopped
// before it produces a Config, with the path in the message.
func TestDefaultsRefuseAnUnusablePortRange(t *testing.T) {
	embedded := []byte("[ports]\nfrom = 49000\nto = 51000\n")
	for _, body := range []string{
		"[ports]\nfrom = 49000\nto = 49001\n",
		"[ports]\nfrom = 80\nto = 90\n",
		"[ports]\nfrom = 51000\nto = 49000\n",
	} {
		path := filepath.Join(t.TempDir(), "defaults.toml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDefaults(embedded, path); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%q: LoadDefaults = %v, want an error naming %s", body, err, path)
		}
	}

	path := filepath.Join(t.TempDir(), "defaults.toml")
	if err := os.WriteFile(path, []byte("[ports]\nfrom = 60000\nto = 60002\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDefaults(embedded, path)
	if err != nil {
		t.Fatalf("a range of exactly three ports should be accepted: %v", err)
	}
	c := d.ToConfig()
	if c.AppPort < 60000 || c.AppPort > 60002 || c.DatabasePort < 60000 || c.DatabasePort > 60002 || c.AuthPort < 60000 || c.AuthPort > 60002 {
		t.Errorf("ports %d, %d, %d were not drawn from the file's range", c.AppPort, c.DatabasePort, c.AuthPort)
	}
}
