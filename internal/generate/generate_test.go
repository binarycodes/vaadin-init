package generate

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/binarycodes/vaadin-init/internal/config"
)

// templates reads the real template tree rather than a fixture, because the point
// of these tests is the templates.
func templates(t *testing.T) *Generator {
	t.Helper()
	return New(os.DirFS("../../templates"))
}

func baseConfig() config.Config {
	return config.Config{
		GroupID:       "com.example.tools",
		ArtifactID:    "note-harbor",
		ProjectName:   "Note Harbor",
		Description:   "Somewhere to put things",
		Package:       "com.example.tools.noteharbor",
		JavaVersion:   "21",
		VaadinVersion: "25.2.6",
		BootVersion:   "4.1.1",
		OutputDir:     "note-harbor",
	}
}

// everyCombination enumerates all 32 combinations of the five options, so that a
// template conditional that only holds for the combinations someone happened to
// try by hand fails here instead.
func everyCombination() []config.Config {
	var all []config.Config
	for bits := 0; bits < 32; bits++ {
		c := baseConfig()
		c.Database = bits&1 != 0
		c.Auth = bits&2 != 0
		c.E2E = bits&4 != 0
		c.Coverage = bits&8 != 0
		c.Traceable = bits&16 != 0
		all = append(all, c)
	}
	return all
}

func (g *Generator) renderMap(t *testing.T, c config.Config) map[string]string {
	t.Helper()
	files, err := g.Render(c)
	if err != nil {
		t.Fatalf("Render(%+v): %v", c.Selected(), err)
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.Path] = string(f.Body)
	}
	return out
}

func TestEveryCombinationRendersWellFormedFiles(t *testing.T) {
	g := templates(t)

	for _, c := range everyCombination() {
		name := strings.Join(c.Selected(), "+")
		if name == "" {
			name = "core-only"
		}
		t.Run(name, func(t *testing.T) {
			files := g.renderMap(t, c)

			pom, ok := files["pom.xml"]
			if !ok {
				t.Fatal("no pom.xml was generated")
			}
			if err := wellFormedXML(pom); err != nil {
				t.Errorf("pom.xml is not well-formed XML: %v", err)
			}

			// A template that renders a Go zero value has almost certainly
			// named a field that does not exist the way the author meant.
			//
			// Leftover template syntax is only a fault in a file that was
			// rendered: run.sh carries `{{.Endpoints.docker.Host}}` for
			// `docker context inspect`, which is precisely why it is copied
			// rather than rendered.
			verbatim := verbatimDestinations()
			for path, body := range files {
				if strings.Contains(body, "<no value>") {
					t.Errorf("%s contains an unresolved template value", path)
				}
				if !verbatim[path] && strings.Contains(body, "{{") {
					t.Errorf("%s still contains template syntax", path)
				}
			}

			if realm, ok := files["environment/dev/keycloak/realm.json"]; ok {
				var parsed any
				if err := json.Unmarshal([]byte(realm), &parsed); err != nil {
					t.Errorf("realm.json is not valid JSON: %v", err)
				}
			}
		})
	}
}

func wellFormedXML(document string) error {
	decoder := xml.NewDecoder(strings.NewReader(document))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// The core is what every project gets, whatever else it does or does not want.
func TestCoreFilesAreAlwaysGenerated(t *testing.T) {
	g := templates(t)
	core := []string{
		"pom.xml",
		"run.sh",
		"run.conf",
		"run.tasks.sh",
		".githooks/commit-msg",
		".gitignore",
		"README.md",
		"src/main/resources/application.properties",
		"src/main/resources/META-INF/resources/styles.css",
		"src/main/java/com/example/tools/noteharbor/Application.java",
		"src/main/java/com/example/tools/noteharbor/ui/view/MainView.java",
		"src/test/java/com/example/tools/noteharbor/ui/view/MainViewTest.java",
	}

	for _, c := range everyCombination() {
		files := g.renderMap(t, c)
		for _, path := range core {
			if _, ok := files[path]; !ok {
				t.Errorf("%s missing for %v", path, c.Selected())
			}
		}
	}
}

func TestOptionalFilesFollowTheirOption(t *testing.T) {
	g := templates(t)

	cases := []struct {
		path string
		want func(config.Config) bool
	}{
		{"src/main/resources/db/migration/V1__init_schema.sql", func(c config.Config) bool { return c.Database }},
		{"src/main/java/com/example/tools/noteharbor/notes/Note.java", func(c config.Config) bool { return c.Database }},
		{"src/main/java/com/example/tools/noteharbor/notes/service/NoteService.java", func(c config.Config) bool { return c.Database }},
		{"src/test/java/com/example/tools/noteharbor/TestcontainersConfiguration.java", func(c config.Config) bool { return c.Database }},
		{"src/main/java/com/example/tools/noteharbor/config/SecurityConfig.java", func(c config.Config) bool { return c.Auth }},
		{"environment/dev/keycloak/realm.json", func(c config.Config) bool { return c.Auth }},
		{"environment/dev/compose.yaml", func(c config.Config) bool { return c.ContainerRequired() }},
		{"src/test/java/com/example/tools/noteharbor/ui/view/MainViewIT.java", func(c config.Config) bool { return c.BrowserTests() }},
		{"src/test/java/com/example/tools/noteharbor/ProtectedRootIT.java", func(c config.Config) bool { return c.ProtectedRootTest() }},
	}

	for _, c := range everyCombination() {
		files := g.renderMap(t, c)
		for _, tc := range cases {
			_, present := files[tc.path]
			if want := tc.want(c); present != want {
				t.Errorf("%s present=%v, want %v for %v", tc.path, present, want, c.Selected())
			}
		}
	}
}

// The pom has to name a dependency exactly when the project has the code that
// uses it, in both directions: an unused dependency is noise, and a missing one
// is a build that does not compile.
func TestPomNamesADependencyOnlyWhenItIsUsed(t *testing.T) {
	g := templates(t)

	cases := []struct {
		fragment string
		want     func(config.Config) bool
	}{
		{"spring-boot-starter-data-jpa", func(c config.Config) bool { return c.Database }},
		{"flyway-database-postgresql", func(c config.Config) bool { return c.Database }},
		{"testcontainers-postgresql", func(c config.Config) bool { return c.Database }},
		{"spring-boot-starter-security-oauth2-client", func(c config.Config) bool { return c.Auth }},
		{"spring-security-test", func(c config.Config) bool { return c.Auth }},
		{"<artifactId>playwright</artifactId>", func(c config.Config) bool { return c.BrowserTests() }},
		{"jacoco-maven-plugin", func(c config.Config) bool { return c.Coverage }},
		{"maven-enforcer-plugin", func(c config.Config) bool { return c.Traceable }},
		{"maven-failsafe-plugin", func(c config.Config) bool { return c.E2E }},
	}

	for _, c := range everyCombination() {
		pom := g.renderMap(t, c)["pom.xml"]
		for _, tc := range cases {
			present := strings.Contains(pom, tc.fragment)
			if want := tc.want(c); present != want {
				t.Errorf("pom mentions %q = %v, want %v for %v", tc.fragment, present, want, c.Selected())
			}
		}
	}
}

// A generated Java file has to declare the package its path puts it in, or it
// does not compile — and nothing else in these tests would notice.
func TestJavaFilesDeclareThePackageTheirPathImplies(t *testing.T) {
	g := templates(t)

	for _, c := range everyCombination() {
		for path, body := range g.renderMap(t, c) {
			if !strings.HasSuffix(path, ".java") {
				continue
			}
			directory := path[:strings.LastIndex(path, "/")]
			for _, root := range []string{"src/main/java/", "src/test/java/"} {
				directory = strings.TrimPrefix(directory, root)
			}
			want := "package " + strings.ReplaceAll(directory, "/", ".") + ";"
			if !strings.HasPrefix(body, want) {
				t.Errorf("%s should start with %q", path, want)
			}
		}
	}
}

func TestRunSHIsExecutableAndTheHookToo(t *testing.T) {
	g := templates(t)
	files, err := g.Render(baseConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		executable := f.Mode&0o111 != 0
		shouldRun := f.Path == "run.sh" || f.Path == ".githooks/commit-msg"
		if executable != shouldRun {
			t.Errorf("%s executable=%v, want %v", f.Path, executable, shouldRun)
		}
	}
}

// run.sh is shared as it stands, so it must arrive unmodified — the moment a
// project's copy is rendered, it stops being the same file for everyone.
func TestSharedFilesAreCopiedVerbatim(t *testing.T) {
	g := templates(t)
	files := g.renderMap(t, baseConfig())

	for _, path := range []string{"run.sh", ".githooks/commit-msg"} {
		source := path
		if path == ".githooks/commit-msg" {
			source = "commit-msg"
		}
		original, err := os.ReadFile("../../templates/" + source)
		if err != nil {
			t.Fatal(err)
		}
		if files[path] != string(original) {
			t.Errorf("%s was modified on the way out; it should be copied verbatim", path)
		}
	}
}

// verbatimDestinations is the set of generated paths that are copied rather than
// rendered, read from the manifest so the test cannot fall out of step with it.
func verbatimDestinations() map[string]bool {
	out := map[string]bool{}
	for _, f := range manifest {
		if f.verbatim {
			out[f.dst] = true
		}
	}
	return out
}
