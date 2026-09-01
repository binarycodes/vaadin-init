// Package config holds the answers vaadin-init collects and the rules for what
// counts as a valid answer.
//
// One struct is filled by the TUI, by command-line flags and by the defaults file
// alike, and it is the only thing the templates are rendered against. A new
// question therefore means a new field here rather than a new parameter threaded
// through the prompt, the generator and every template.
package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Config is a complete description of a project to generate.
type Config struct {
	// Maven coordinates and the human-facing name.
	GroupID     string
	ArtifactID  string
	ProjectName string
	Description string

	// The base Java package. Derived from the coordinates unless the user says
	// otherwise, because the two agreeing is the common case and typing it twice
	// is how they end up disagreeing.
	Package string

	// Versions the build pins. Kept as strings: they are written into XML and a
	// shell config verbatim, and nothing here does arithmetic on them.
	JavaVersion   string
	VaadinVersion string
	BootVersion   string

	// The optional stack pieces. The lean core is not represented because it is
	// not a choice.
	Database  bool
	Auth      bool
	E2E       bool
	Coverage  bool
	Traceable bool

	// Where to write. Everything is created under here.
	OutputDir string
}

// PackagePath is the base package as a directory path, for the src/main/java
// tree. Templates use it to place their own file, which is why the destination
// paths in the generator's manifest are themselves templates.
func (c Config) PackagePath() string {
	return strings.ReplaceAll(c.Package, ".", "/")
}

// ContainerRequired answers run.conf's question of the same name: whether any
// task should refuse to start without a container runtime.
//
// Only the pieces that put a service in the dev stack count. Browser tests do
// not: Playwright brings its own browser, so a project with e2e tests and no
// database still builds and runs on a machine with no Docker or Podman at all.
func (c Config) ContainerRequired() bool {
	return c.Database || c.Auth
}

// ITProfile is the Maven profile that puts the integration tests in production
// mode, or empty when the project has none — run.sh reads it as a plain string
// and adds no -P when it is unset.
func (c Config) ITProfile() string {
	if c.E2E {
		return "it"
	}
	return ""
}

// CommitProperty is the property carrying the built commit's SHA, or empty when
// the project does not ask for one. Empty is what lets a checkout that is not a
// git repository build at all, so it is the honest answer rather than a default.
func (c Config) CommitProperty() string {
	if c.Traceable {
		return "build.commit"
	}
	return ""
}

// BrowserTests reports whether the project gets a Playwright test that drives
// the real UI in a browser.
//
// Not when auth is on: a browser arriving at a protected root is redirected to
// the identity provider, so the test would either need a running Keycloak to log
// in against — turning `verify` red on any machine without the dev stack up — or
// assert nothing about the UI it was written for. Those projects get
// ProtectedRootTest instead, and the README says how to grow it into a real login
// test once an identity provider is available to test against.
func (c Config) BrowserTests() bool {
	return c.E2E && !c.Auth
}

// ProtectedRootTest reports whether the project gets the integration test that
// asserts an anonymous visitor is sent to the identity provider. It needs no
// browser and no identity provider, so it passes anywhere the build runs.
func (c Config) ProtectedRootTest() bool {
	return c.E2E && c.Auth
}

// ContainerPrefix is what the dev stack's containers and volumes are named with.
// A container name is not written to be read, so lowercase and hyphenated.
func (c Config) ContainerPrefix() string {
	return strings.ToLower(c.ArtifactID) + "-dev"
}

// DatabaseName is the dev database, and the role that owns it. Underscores
// rather than hyphens: an unquoted PostgreSQL identifier cannot carry a hyphen,
// and quoting one everywhere is a tax on every later query.
func (c Config) DatabaseName() string {
	return strings.ReplaceAll(strings.ToLower(c.ArtifactID), "-", "_")
}

// Selected lists the optional pieces that are on, for the summary the tool
// prints when it is done.
func (c Config) Selected() []string {
	var on []string
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"database", c.Database},
		{"auth", c.Auth},
		{"e2e", c.E2E},
		{"coverage", c.Coverage},
		{"traceable builds", c.Traceable},
	} {
		if f.set {
			on = append(on, f.name)
		}
	}
	return on
}

// Validate rejects a Config that would generate a project that cannot build.
// Both entry points run it: the TUI validates field by field as it goes, and
// this catches what flags set without ever passing through a prompt.
func (c Config) Validate() error {
	for _, check := range []struct {
		field string
		err   error
	}{
		{"group id", ValidGroupID(c.GroupID)},
		{"artifact id", ValidArtifactID(c.ArtifactID)},
		{"project name", ValidProjectName(c.ProjectName)},
		{"package", ValidPackage(c.Package)},
		{"java version", ValidJavaVersion(c.JavaVersion)},
		{"vaadin version", ValidVersion(c.VaadinVersion)},
		{"spring boot version", ValidVersion(c.BootVersion)},
	} {
		if check.err != nil {
			return fmt.Errorf("%s: %w", check.field, check.err)
		}
	}
	if c.OutputDir == "" {
		return fmt.Errorf("output directory: must not be empty")
	}
	return nil
}

// The shapes Maven and Java actually accept, narrowed to the ones a new project
// should have. Being stricter than the tools here is deliberate: a group id with
// an uppercase letter is legal and still wrong, and the generator is the last
// place it can be cheaply fixed.
var (
	groupIDPattern    = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9][a-z0-9_]*)*$`)
	artifactIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	identifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	versionPattern    = regexp.MustCompile(`^\d+\.\d+(\.\d+)?(-[A-Za-z0-9.]+)?$`)
)

func ValidGroupID(s string) error {
	if s == "" {
		return fmt.Errorf("must not be empty")
	}
	if !groupIDPattern.MatchString(s) {
		return fmt.Errorf("must be lowercase reverse-DNS, e.g. com.example.tools")
	}
	return nil
}

func ValidArtifactID(s string) error {
	if s == "" {
		return fmt.Errorf("must not be empty")
	}
	if !artifactIDPattern.MatchString(s) {
		return fmt.Errorf("must be lowercase words joined by single hyphens, e.g. my-app")
	}
	return nil
}

func ValidProjectName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("must not be empty")
	}
	return nil
}

// ValidPackage checks every segment is a Java identifier and not a keyword,
// because javac's complaint about a package called `new` names the generated
// file and gives no hint that the tool could have said so first.
func ValidPackage(s string) error {
	if s == "" {
		return fmt.Errorf("must not be empty")
	}
	for _, segment := range strings.Split(s, ".") {
		if !identifierPattern.MatchString(segment) {
			return fmt.Errorf("segment %q must be lowercase and start with a letter", segment)
		}
		if javaKeywords[segment] {
			return fmt.Errorf("segment %q is a Java keyword", segment)
		}
	}
	return nil
}

// ValidJavaVersion enforces the floor Spring Boot 4 sets rather than a list of
// blessed releases, so a JDK newer than this tool needs no new release of it.
func ValidJavaVersion(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a major version number, e.g. 21")
	}
	if n < 17 {
		return fmt.Errorf("Spring Boot 4 needs Java 17 or newer; got %d", n)
	}
	return nil
}

func ValidVersion(s string) error {
	if s == "" {
		return fmt.Errorf("must not be empty")
	}
	if !versionPattern.MatchString(s) {
		return fmt.Errorf("must look like 25.2.6 or 25.3.0-beta1; got %q", s)
	}
	return nil
}

var javaKeywords = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true, "byte": true,
	"case": true, "catch": true, "char": true, "class": true, "const": true,
	"continue": true, "default": true, "do": true, "double": true, "else": true,
	"enum": true, "extends": true, "final": true, "finally": true, "float": true,
	"for": true, "goto": true, "if": true, "implements": true, "import": true,
	"instanceof": true, "int": true, "interface": true, "long": true, "native": true,
	"new": true, "package": true, "private": true, "protected": true, "public": true,
	"return": true, "short": true, "static": true, "strictfp": true, "super": true,
	"switch": true, "synchronized": true, "this": true, "throw": true, "throws": true,
	"transient": true, "try": true, "void": true, "volatile": true, "while": true,
	"_": true,
}
