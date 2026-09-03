package generate

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		{"src/test/java/com/example/tools/noteharbor/TestSecurityConfiguration.java", func(c config.Config) bool { return c.Auth }},
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

// gitAvailable skips a test on a machine with no git, and gives the ones that run
// an identity to commit with.
//
// The identity comes from the environment rather than from `git config`, because a
// test must not write to the user's global configuration — and because a CI runner
// often has no identity at all, which is the case that made the commit worth
// making here in the first place.
func gitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the PATH")
	}
	t.Setenv("GIT_AUTHOR_NAME", "vaadin-init test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "vaadin-init test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

func gitOutput(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// A generated project has to be buildable the moment it is generated.
//
// With traceable builds on — the default — its own build refuses to run until a
// commit exists, because there is no SHA to stamp into it. So the first commit is
// part of generating, not a step the user has to discover from an error message.
func TestGeneratedProjectIsCommitted(t *testing.T) {
	gitAvailable(t)

	c := baseConfig()
	c.OutputDir = t.TempDir()
	c.Traceable = true

	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: true})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !result.GitInit || !result.HooksPath {
		t.Fatalf("the repository should be set up: %+v", result)
	}
	if !result.Committed {
		t.Fatalf("no commit was made: %s", result.GitMessage)
	}

	// The commit has to exist, and be the one thing in the history.
	subject, err := gitOutput(t, result.Root, "log", "--format=%s")
	if err != nil {
		t.Fatalf("git log: %v: %s", err, subject)
	}
	if subject != initialCommitMessage {
		t.Errorf("commit subject = %q, want %q", subject, initialCommitMessage)
	}

	// And nothing may be left behind, or the working tree starts out dirty.
	status, err := gitOutput(t, result.Root, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status: %v: %s", err, status)
	}
	if status != "" {
		t.Errorf("the working tree should be clean after generating, got:\n%s", status)
	}
}

// The commit-msg hook the project ships is wired up before that commit is made,
// so the commit goes through the hook. A message its own hook would reject would
// be a poor advertisement for the hook.
func TestTheInitialCommitSatisfiesTheGeneratedHook(t *testing.T) {
	gitAvailable(t)

	c := baseConfig()
	c.OutputDir = t.TempDir()

	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: true})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !result.Committed {
		t.Fatalf("no commit was made: %s", result.GitMessage)
	}

	hooksPath, err := gitOutput(t, result.Root, "config", "core.hooksPath")
	if err != nil || hooksPath != ".githooks" {
		t.Fatalf("core.hooksPath = %q (%v), want .githooks", hooksPath, err)
	}

	// Directly: the hook must reject what it claims to reject, or the commit
	// above proves nothing about it.
	message := filepath.Join(t.TempDir(), "message")
	if err := os.WriteFile(message, []byte("not a conventional subject\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hook := exec.Command(filepath.Join(result.Root, ".githooks", "commit-msg"), message)
	if err := hook.Run(); err == nil {
		t.Error("the hook should reject a non-conventional subject")
	}
}

// A repository that already has history is left alone.
//
// --force onto an existing checkout would otherwise sweep whatever was in the
// working tree into a commit about a new project — which is not this tool's
// business, and not something the user could easily undo.
func TestExistingHistoryIsNotTouched(t *testing.T) {
	gitAvailable(t)

	root := t.TempDir()
	if output, err := gitOutput(t, root, "init", "--quiet"); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "NOTES.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := gitOutput(t, root, "add", "-A"); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	if output, err := gitOutput(t, root, "commit", "--message", "chore: my own commit"); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}

	c := baseConfig()
	c.OutputDir = root

	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: true})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if result.Committed {
		t.Error("no commit should have been made over existing history")
	}

	subjects, err := gitOutput(t, root, "log", "--format=%s")
	if err != nil {
		t.Fatalf("git log: %v: %s", err, subjects)
	}
	if subjects != "chore: my own commit" {
		t.Errorf("history = %q, want only the user's own commit", subjects)
	}
}

// Asking for a repository without a commit has to give exactly that.
func TestCommitCanBeDeclined(t *testing.T) {
	gitAvailable(t)

	c := baseConfig()
	c.OutputDir = t.TempDir()

	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: false})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !result.HooksPath {
		t.Fatalf("the repository should still be set up: %+v", result)
	}
	if result.Committed {
		t.Error("no commit should have been made")
	}
	if _, err := gitOutput(t, result.Root, "rev-parse", "--verify", "HEAD"); err == nil {
		t.Error("the repository should have no commits")
	}
}

// And asking for no git at all has to leave git alone entirely.
func TestGitCanBeDeclinedEntirely(t *testing.T) {
	c := baseConfig()
	c.OutputDir = t.TempDir()

	result, err := templates(t).Write(c, WriteOptions{Force: true})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if result.GitInit || result.HooksPath || result.Committed {
		t.Errorf("nothing about git should have happened: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(result.Root, ".git")); !os.IsNotExist(err) {
		t.Error("no .git directory should have been created")
	}
}

// gitWithoutIdentity is the machine the author prompt exists for: git on the
// PATH and no idea who is using it. Every place git would look is pointed at
// nothing — the environment, the global file, the system file — because a
// developer's own machine has all three filled in and would make these tests
// pass for the wrong reason.
func gitWithoutIdentity(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the PATH")
	}
	for _, name := range []string{
		"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL",
	} {
		if value, set := os.LookupEnv(name); set {
			t.Cleanup(func() { os.Setenv(name, value) })
			os.Unsetenv(name)
		}
	}
	home := t.TempDir()
	global := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// The failure that started this: a fresh machine, a generated project, and a
// first commit that never happened because git did not know who was making it.
// The project is still written, and the summary says what to do.
func TestWithoutAnAuthorTheCommitIsReportedNotMade(t *testing.T) {
	gitWithoutIdentity(t)

	c := baseConfig()
	c.OutputDir = t.TempDir()
	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Committed {
		t.Fatal("a commit was made with no identity to make it as")
	}
	if !strings.Contains(result.GitMessage, "nothing was committed") ||
		!strings.Contains(result.GitMessage, initialCommitMessage) {
		t.Errorf("the summary does not say how to commit: %q", result.GitMessage)
	}
}

// An author given to the generator goes into the new repository's own config and
// is who the first commit is by — and nowhere else, since a tool that bootstraps
// one project has no business changing every other repository's settings.
func TestTheAuthorIsKeptInTheRepository(t *testing.T) {
	gitWithoutIdentity(t)

	c := baseConfig()
	c.OutputDir = t.TempDir()
	c.AuthorName, c.AuthorEmail = "Ann Example", "ann@example.invalid"
	result, err := templates(t).Write(c, WriteOptions{Force: true, Git: true, Commit: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuthorSet {
		t.Error("the result does not say the author was kept")
	}
	if !result.Committed {
		t.Fatalf("no commit was made: %s", result.GitMessage)
	}

	author, err := gitOutput(t, result.Root, "log", "--format=%an <%ae>")
	if err != nil || author != "Ann Example <ann@example.invalid>" {
		t.Errorf("the commit is by %q (%v), want the author given", author, err)
	}
	email, err := gitOutput(t, result.Root, "config", "--local", "user.email")
	if err != nil || email != "ann@example.invalid" {
		t.Errorf("the repository's own user.email = %q (%v)", email, err)
	}

	// And the global configuration is exactly as empty as it was.
	if global, err := gitOutput(t, result.Root, "config", "--global", "--list"); global != "" {
		t.Errorf("the global configuration was written to: %q (%v)", global, err)
	}
}

// Asking before the commit rather than after it fails depends on knowing what git
// would do, and git is the one to ask: whatever it has — a global file, the
// environment, half of each — is what the answer opens on.
func TestCurrentAuthorIsWhatGitWouldCommitAs(t *testing.T) {
	gitWithoutIdentity(t)

	author, err := CurrentAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if author.Known() || author != (Author{}) {
		t.Errorf("with nothing configured, author = %+v, want nothing", author)
	}

	global := os.Getenv("GIT_CONFIG_GLOBAL")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Ann Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	author, _ = CurrentAuthor()
	if author.Known() || author.Name != "Ann Example" || author.Email != "" {
		t.Errorf("with only a name configured, author = %+v, want the name and no email", author)
	}

	t.Setenv("GIT_COMMITTER_EMAIL", "ann@example.invalid")
	author, _ = CurrentAuthor()
	if !author.Known() || author != (Author{Name: "Ann Example", Email: "ann@example.invalid"}) {
		t.Errorf("with a name and an email, author = %+v, want both", author)
	}
}

// The identity of the repository the user happens to be standing in is not one
// the new repository will have, so it must not be what stops the question.
func TestCurrentAuthorIgnoresTheRepositoryStoodIn(t *testing.T) {
	gitWithoutIdentity(t)

	elsewhere := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.name", "Somebody Else"},
		{"config", "user.email", "else@example.invalid"},
	} {
		if output, err := gitOutput(t, elsewhere, args...); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	t.Chdir(elsewhere)

	author, err := CurrentAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if author.Known() {
		t.Errorf("author = %+v, taken from a repository the new one will not inherit from", author)
	}
}
