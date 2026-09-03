package config

import "testing"

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

func TestValidJavaVersion(t *testing.T) {
	for _, s := range []string{"17", "21", "25", "31"} {
		if err := ValidJavaVersion(s); err != nil {
			t.Errorf("ValidJavaVersion(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"", "11", "8", "21.0.2", "twenty-one"} {
		if err := ValidJavaVersion(s); err == nil {
			t.Errorf("ValidJavaVersion(%q) = nil, want an error", s)
		}
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

	if err := d.ToConfig().Validate(); err != nil {
		t.Fatalf("the defaults do not produce a valid config: %v", err)
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
		VaadinVersion: "25.2.6", BootVersion: "4.1.1", OutputDir: "my-app",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("a config with no author should validate: %v", err)
	}

	c.AuthorName, c.AuthorEmail = "Ann Example", "ann@example.invalid"
	if err := c.Validate(); err != nil {
		t.Errorf("a config with an author should validate: %v", err)
	}

	c.AuthorEmail = "ann"
	if err := c.Validate(); err == nil {
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
