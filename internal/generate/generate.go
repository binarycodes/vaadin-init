// Package generate writes a project from the embedded templates.
//
// The whole tree is rendered into memory before anything is written, so a broken
// template or a bad path fails with nothing on disk. A generator that leaves half
// a project behind is worse than one that refuses: the user cannot tell what is
// missing, and the obvious recovery — run it again — is the one thing the
// half-written directory now blocks.
//
// The templates are kept one directory per Vaadin version floor — templates/24,
// templates/25, templates/25.2 — each holding only the files that differ from
// the floors below it. A project for version X is rendered through every
// directory whose floor is at or below X, newest first, so the closest set wins
// and a higher one is never consulted. What a starter project contains changes at
// a major and now and then at a minor, and a directory per change is what keeps
// that out of the templates as conditionals.
package generate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/version"
)

// file is one template and where it lands.
//
// dst is itself a template, which is what places the Java sources under the
// chosen package without a placeholder segment in the source tree that every
// reader then has to decode.
type file struct {
	src  string
	dst  string
	mode os.FileMode

	// when reports whether this project wants the file at all. nil means always.
	when func(config.Config) bool

	// verbatim copies the bytes through untouched. It marks the files that are
	// shared rather than generated — the task runner and the commit-msg hook name
	// no project, so they are the same file in every generated project and
	// rendering them would only invite someone to make them not be.
	verbatim bool
}

const (
	// Not 0644/0755 as literals at each use: a mode is either "a file" or "a
	// thing that runs", and those are the only two answers here.
	fileMode = os.FileMode(0o644)
	execMode = os.FileMode(0o755)
)

func database(c config.Config) bool      { return c.Database }
func auth(c config.Config) bool          { return c.Auth }
func containers(c config.Config) bool    { return c.ContainerRequired() }
func browserTests(c config.Config) bool  { return c.BrowserTests() }
func protectedRoot(c config.Config) bool { return c.ProtectedRootTest() }

// manifest is the generated project, declared once.
//
// A new template is a line here rather than a convention about directory names,
// because the conditions do not nest the way a directory tree does: the dev
// compose file is wanted by the database and by auth alike, and a
// templates/database/ tree would have to either duplicate it or lie about who
// owns it.
var manifest = []file{
	{src: "pom.xml.tmpl", dst: "pom.xml"},
	{src: "run.sh", dst: "run.sh", mode: execMode, verbatim: true},
	{src: "run.conf.tmpl", dst: "run.conf"},
	{src: "run.tasks.sh.tmpl", dst: "run.tasks.sh"},
	{src: "commit-msg", dst: ".githooks/commit-msg", mode: execMode, verbatim: true},
	{src: "gitignore.tmpl", dst: ".gitignore"},
	{src: "README.md.tmpl", dst: "README.md"},

	{src: "application.properties.tmpl", dst: "src/main/resources/application.properties"},
	{src: "styles.css", dst: "src/main/resources/META-INF/resources/styles.css", verbatim: true},

	{src: "java/Application.java.tmpl", dst: "src/main/java/{{.PackagePath}}/Application.java"},
	{src: "java/MainView.java.tmpl", dst: "src/main/java/{{.PackagePath}}/ui/view/MainView.java"},
	{src: "java/MainViewTest.java.tmpl", dst: "src/test/java/{{.PackagePath}}/ui/view/MainViewTest.java"},

	{src: "styles/main-view.css", dst: "src/main/resources/META-INF/resources/styles/main-view.css", verbatim: true},

	{src: "compose.yaml.tmpl", dst: "environment/dev/compose.yaml", when: containers},

	{src: "sql/V1__init_schema.sql.tmpl", dst: "src/main/resources/db/migration/V1__init_schema.sql", when: database},
	{src: "java/Note.java.tmpl", dst: "src/main/java/{{.PackagePath}}/notes/Note.java", when: database},
	{src: "java/NoteRepository.java.tmpl", dst: "src/main/java/{{.PackagePath}}/notes/NoteRepository.java", when: database},
	{src: "java/NoteService.java.tmpl", dst: "src/main/java/{{.PackagePath}}/notes/service/NoteService.java", when: database},
	{src: "java/TestcontainersConfiguration.java.tmpl", dst: "src/test/java/{{.PackagePath}}/TestcontainersConfiguration.java", when: database},

	{src: "java/SecurityConfig.java.tmpl", dst: "src/main/java/{{.PackagePath}}/config/SecurityConfig.java", when: auth},
	{src: "java/TestSecurityConfiguration.java.tmpl", dst: "src/test/java/{{.PackagePath}}/TestSecurityConfiguration.java", when: auth},
	{src: "keycloak-realm.json.tmpl", dst: "environment/dev/keycloak/realm.json", when: auth},

	{src: "java/MainViewIT.java.tmpl", dst: "src/test/java/{{.PackagePath}}/ui/view/MainViewIT.java", when: browserTests},
	{src: "java/ProtectedRootIT.java.tmpl", dst: "src/test/java/{{.PackagePath}}/ProtectedRootIT.java", when: protectedRoot},
}

// atLeast reports whether the project's Vaadin is the given version or newer,
// for a manifest entry one line has and another does not. Presence is the
// manifest's to decide, so a file dropped in a later line is a predicate here
// and not a tombstone in the later directory.
func atLeast(vaadin string) func(config.Config) bool {
	return func(c config.Config) bool { return version.Compare(c.VaadinVersion, vaadin) >= 0 }
}

// layer is one template directory: its name, and the version it is true from.
type layer struct {
	name  string
	floor version.Version
}

// Generator renders the manifest from a template filesystem.
type Generator struct {
	templates fs.FS

	// The version directories, newest floor first.
	layers []layer
}

// New reads the template root. Every entry there is a version directory; a name
// that is not a version floor, or a file at the top, is an error naming it. An
// embedded tree is fixed at build time, so this is a build-time assertion with a
// runtime signature.
func New(templates fs.FS) (*Generator, error) {
	entries, err := fs.ReadDir(templates, ".")
	if err != nil {
		return nil, fmt.Errorf("reading the template root: %w", err)
	}
	g := &Generator{templates: templates}
	for _, entry := range entries {
		floor, ok := version.ParseFloor(entry.Name())
		if !entry.IsDir() || !ok {
			return nil, fmt.Errorf("templates/%s: every entry at the top of the template tree is a directory named by the Vaadin version it is true from, like 25 or 25.2", entry.Name())
		}
		g.layers = append(g.layers, layer{name: entry.Name(), floor: floor})
	}
	if len(g.layers) == 0 {
		return nil, errors.New("the template tree has no version directories")
	}
	sort.Slice(g.layers, func(i, j int) bool { return g.layers[i].floor.After(g.layers[j].floor) })
	return g, nil
}

// Layers lists the directories consulted for a Vaadin version, newest floor
// first: every directory whose floor is at or below it.
//
// An empty chain is an error naming the oldest floor there is, not a fall-back
// to it: the case only arises when the rules and the tree disagree — a --compat
// override marking a line supported that this binary has no templates for — and
// a project rendered from another line's templates is exactly what a version
// check exists to stop.
func (g *Generator) Layers(vaadin string) ([]string, error) {
	v, ok := version.Parse(vaadin)
	if !ok {
		return nil, fmt.Errorf("%q is not a Vaadin version", vaadin)
	}
	var chain []string
	for _, l := range g.layers {
		if !l.floor.After(v) {
			chain = append(chain, l.name)
		}
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("no templates for Vaadin %s: the oldest set is for %s", vaadin, g.layers[len(g.layers)-1].name)
	}
	return chain, nil
}

// open reads a template from the first layer that has it, and says which.
func (g *Generator) open(chain []string, src string) (body []byte, from string, err error) {
	for _, l := range chain {
		from = path.Join(l, src)
		body, err = fs.ReadFile(g.templates, from)
		if err == nil {
			return body, from, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("reading template %s: %w", from, err)
		}
	}
	return nil, "", fmt.Errorf("no template %s in any of %s", src, strings.Join(chain, ", "))
}

// File is one finished file waiting to be written.
type File struct {
	Path string
	Mode os.FileMode
	Body []byte

	// Source is the template it came from, layer included: 25.2/pom.xml.tmpl.
	Source string
}

// Render produces the whole tree in memory. Paths are relative to the project
// root and use forward slashes, as they do in the manifest; they become
// platform paths only at the moment they are written.
func (g *Generator) Render(c config.Config) ([]File, error) {
	chain, err := g.Layers(c.VaadinVersion)
	if err != nil {
		return nil, err
	}

	var out []File
	for _, f := range manifest {
		if f.when != nil && !f.when(c) {
			continue
		}

		body, source, err := g.open(chain, f.src)
		if err != nil {
			return nil, err
		}

		dst, err := renderString("path:"+f.src, f.dst, c)
		if err != nil {
			return nil, err
		}
		if !f.verbatim {
			text, err := renderString(f.src, string(body), c)
			if err != nil {
				return nil, err
			}
			body = []byte(text)
		}

		mode := f.mode
		if mode == 0 {
			mode = fileMode
		}
		out = append(out, File{Path: dst, Mode: mode, Body: body, Source: source})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// renderString executes one template. missingkey=error only bites on maps, so
// the real protection against a typo'd field is that text/template refuses to
// execute an unknown field on a struct at all.
func renderString(name, text string, c config.Config) (string, error) {
	parsed, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, c); err != nil {
		return "", fmt.Errorf("rendering %s: %w", name, err)
	}
	return buf.String(), nil
}

// WriteOptions are the decisions about writing, as opposed to about the project.
//
// A struct rather than three booleans in a row: at the call site, Write(cfg,
// false, true, true) says nothing about which of them is which.
type WriteOptions struct {
	// Force writes into a directory that already has something in it.
	Force bool

	// Git makes the target a repository and points git at the hook the project
	// ships. Without it, nothing under .git is touched.
	Git bool

	// Commit makes the first commit. Only ever the first: a repository that
	// already has history is left alone.
	Commit bool
}

// Result reports what a run did, so the caller can print it rather than the
// generator printing as it goes.
type Result struct {
	Root       string
	Paths      []string
	GitInit    bool
	HooksPath  bool
	AuthorSet  bool
	Committed  bool
	GitMessage string
}

// Write renders and then writes the project.
//
// An existing non-empty directory stops the run unless force is set: the target
// is usually a path the user typed, and quietly merging a new project into
// whatever was already there is not a recoverable mistake.
func (g *Generator) Write(c config.Config, options WriteOptions) (Result, error) {
	files, err := g.Render(c)
	if err != nil {
		return Result{}, err
	}

	root, err := filepath.Abs(c.OutputDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolving %s: %w", c.OutputDir, err)
	}

	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 && !options.Force {
		return Result{}, fmt.Errorf("%s already exists and is not empty; pass --force to write into it anyway", root)
	} else if err != nil && !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("checking %s: %w", root, err)
	}

	result := Result{Root: root}
	for _, f := range files {
		target := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, fmt.Errorf("creating %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, f.Body, f.Mode); err != nil {
			return result, fmt.Errorf("writing %s: %w", target, err)
		}
		// Writing an existing file leaves its old mode alone, so --force onto a
		// tree where run.sh lost its executable bit has to set it again.
		if err := os.Chmod(target, f.Mode); err != nil {
			return result, fmt.Errorf("setting mode on %s: %w", target, err)
		}
		result.Paths = append(result.Paths, f.Path)
	}

	if options.Git {
		g.initRepository(root, c, options.Commit, &result)
	}
	return result, nil
}

// initRepository makes the checkout a git repository, points git at the hook the
// project ships, and makes the first commit.
//
// The hook is the reason this is not left to the user: a file under .githooks is
// inert until core.hooksPath names it, so a generated commit-msg hook that nobody
// wires up enforces nothing while looking like it does.
//
// The commit is here for a related reason. A project generated with traceable
// builds cannot build at all until a commit exists — its own build refuses,
// because there is no SHA to stamp — so "generated" and "buildable" would
// otherwise be two different states with an unexplained step between them.
//
// Failure here is reported, not fatal. The project is already written and
// perfectly usable; git being absent, or having no identity configured, is a fact
// about the machine rather than a problem with the generated project.
func (g *Generator) initRepository(root string, c config.Config, commit bool, result *Result) {
	git, err := exec.LookPath("git")
	if err != nil {
		result.GitMessage = "git is not on the PATH: run `git init` yourself, then `git config core.hooksPath .githooks`"
		return
	}

	// output as well as error: git says why it refused in its output, and that
	// sentence is the only part worth showing.
	runOutput := func(args ...string) (string, error) {
		command := exec.Command(git, args...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return string(output), nil
	}
	run := func(args ...string) error {
		_, err := runOutput(args...)
		return err
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		result.GitInit = true
	} else if err := run("init", "--quiet"); err != nil {
		result.GitMessage = err.Error()
		return
	} else {
		result.GitInit = true
	}

	if err := run("config", "core.hooksPath", ".githooks"); err != nil {
		result.GitMessage = err.Error()
		return
	}
	result.HooksPath = true

	// Into this repository's config and nowhere else. The answer was asked for
	// because git had none, and the fix that reaches every other repository is
	// git's own to offer — the summary points at it — not something to do on the
	// side of generating a project.
	if c.AuthorName != "" {
		if err := run("config", "user.name", c.AuthorName); err != nil {
			result.GitMessage = err.Error()
			return
		}
	}
	if c.AuthorEmail != "" {
		if err := run("config", "user.email", c.AuthorEmail); err != nil {
			result.GitMessage = err.Error()
			return
		}
	}
	result.AuthorSet = c.AuthorName != "" || c.AuthorEmail != ""

	if !commit {
		return
	}

	// Only ever the first commit. With --force onto an existing checkout, `git
	// add -A` would sweep up whatever else was in the working tree and commit it
	// under a message about a new project — which is not this tool's business and
	// is not something the user could easily undo.
	if err := run("rev-parse", "--verify", "HEAD"); err == nil {
		result.GitMessage = "this repository already has commits, so none was made"
		return
	}

	if err := run("add", "-A"); err != nil {
		result.GitMessage = err.Error()
		return
	}
	// A message the project's own hook accepts: Conventional Commits, one line.
	// A generated project whose first commit its own hook would have rejected
	// would be a poor advertisement for the hook.
	if output, err := runOutput("commit", "--message", initialCommitMessage); err != nil {
		result.GitMessage = "nothing was committed: " + firstLine(output) +
			". Commit yourself with: git add -A && git commit -m '" + initialCommitMessage + "'"
		return
	}
	result.Committed = true
}

// Author is who git would make a commit as.
type Author struct {
	Name  string
	Email string
}

// Known reports whether git has enough to commit with. Both halves, although a
// name alone would do on some platforms: a commit by whatever the login account
// happens to be called is the one the prompt exists to avoid.
func (a Author) Known() bool {
	return a.Name != "" && a.Email != ""
}

// CurrentAuthor asks git who it would commit as in the repository about to be
// created at root, so that the question can be asked before the commit is
// attempted rather than reported after it has failed.
//
// Asked where the commit will happen, because that is the only place git answers
// for: an identity in the config of the repository the user is standing in is not
// inherited by a repository created inside it, and one supplied by a conditional
// include keyed on the directory is not seen from anywhere else. useConfigOnly
// makes git refuse to guess — what it would have guessed, the login name at the
// host, is exactly what the prompt is there to replace. When git refuses,
// whichever half it does have is still fetched, so the prompt can open on it.
//
// The error is git being absent, and nothing else: a machine with no git gets no
// question, since there will be no commit to ask on behalf of.
func CurrentAuthor(root string) (Author, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return Author{}, err
	}
	dir, done := askingPlace(git, root)
	defer done()

	output := func(args ...string) string {
		command := exec.Command(git, args...)
		command.Dir = dir
		out, _ := command.Output()
		return strings.TrimSpace(string(out))
	}

	if ident := output("-c", "user.useConfigOnly=true", "var", "GIT_COMMITTER_IDENT"); ident != "" {
		if author, ok := parseIdent(ident); ok {
			return author, nil
		}
	}
	return Author{
		Name:  output("config", "--get", "user.name"),
		Email: output("config", "--get", "user.email"),
	}, nil
}

// askingPlace is a directory git will answer for as it would for the new
// repository, and what to do with it afterwards.
//
// The output directory itself when it already is a repository — its own config is
// the one the commit will use. Otherwise a throwaway repository beside where the
// project will go, since a repository is what a conditional include is keyed on
// and a nested one inherits nothing from the repository around it. Beside rather
// than inside: a directory left in the target by an interrupted run would be the
// thing that makes the next run refuse to write there. If none of that is
// possible, git is asked from a directory that is no repository at all, which
// still gets everything the machine has configured.
func askingPlace(git, root string) (string, func()) {
	nothing := func() {}
	if root, err := filepath.Abs(root); err == nil {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			return root, nothing
		}
		beside := filepath.Dir(root)
		for {
			if info, err := os.Stat(beside); err == nil && info.IsDir() {
				break
			}
			if parent := filepath.Dir(beside); parent != beside {
				beside = parent
			} else {
				break
			}
		}
		if probe, err := os.MkdirTemp(beside, ".vaadin-init-*"); err == nil {
			remove := func() { os.RemoveAll(probe) }
			if exec.Command(git, "-C", probe, "init", "--quiet").Run() == nil {
				return probe, remove
			}
			remove()
		}
	}
	return os.TempDir(), nothing
}

// parseIdent reads `Name <email> <timestamp> <zone>`, the one form git var prints.
func parseIdent(ident string) (Author, bool) {
	open := strings.LastIndex(ident, " <")
	end := strings.LastIndex(ident, ">")
	if open < 0 || end < open {
		return Author{}, false
	}
	author := Author{Name: strings.TrimSpace(ident[:open]), Email: ident[open+2 : end]}
	return author, author.Known()
}

// initialCommitMessage is what the first commit says. It has to satisfy the hook
// the project ships — Conventional Commits, one line — since a generated project
// whose own first commit its own hook would reject is a poor advertisement.
const initialCommitMessage = "chore: initial commit"

// firstLine keeps git's complaint to its useful part. "Author identity unknown"
// is followed by a dozen lines of advice that would bury the summary.
func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return strings.TrimSpace(text[:index])
	}
	return strings.TrimSpace(text)
}
