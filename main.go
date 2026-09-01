// Command vaadin-init bootstraps an opinionated Vaadin and Spring Boot project.
//
// Run it with no arguments for the TUI. Every question it asks is also a flag, so
// the same project can be generated from a script: --yes skips the prompts and
// takes the flags and the defaults file as the answers.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/generate"
	"github.com/binarycodes/vaadin-init/internal/prompt"
	"github.com/binarycodes/vaadin-init/internal/ui"
	"github.com/binarycodes/vaadin-init/internal/versions"
)

// The templates and the defaults ship inside the binary, which is the whole
// distribution story: one file to download, nothing to install beside it, and no
// way for a binary to find itself next to someone else's templates.
//
// all: rather than a plain embed because the pattern would otherwise skip files
// whose names begin with a dot — the generated .gitignore and the commit-msg hook
// under .githooks are exactly that, and their absence would not be a build error.
//
//go:embed all:templates
var templateFS embed.FS

//go:embed defaults.toml
var defaultsTOML []byte

// version is stamped in at build time; see the Makefile.
var version = "dev"

// How long the version lookup gets before the built-in defaults are used
// instead. Short on purpose: it runs while the first questions are being
// answered, and a slow network should cost a stale default, never a hang.
const lookupTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		if errors.Is(err, prompt.ErrCancelled) {
			fmt.Fprintln(os.Stderr, ui.Cancelled())
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, ui.Error(err))
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("vaadin-init", flag.ContinueOnError)
	flags.Usage = func() { usage(flags) }

	defaultsPath := flags.String("defaults", "", "read defaults from this file instead of the per-user one")
	showVersion := flags.Bool("version", false, "print the version of vaadin-init and exit")

	// The defaults file has to be read before the other flags are defined,
	// because it supplies their default values — which is also what makes the
	// generated --help describe this machine's defaults rather than the tool's.
	if err := preParse(flags, os.Args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("vaadin-init %s\n", version)
		return nil
	}

	defaults, err := config.LoadDefaults(defaultsTOML, *defaultsPath)
	if err != nil {
		return err
	}
	cfg := defaults.ToConfig()

	groupID := flags.String("group-id", cfg.GroupID, "Maven group id")
	artifactID := flags.String("artifact-id", cfg.ArtifactID, "Maven artifact id")
	name := flags.String("name", cfg.ProjectName, "project name (default: derived from the artifact id)")
	pkg := flags.String("package", cfg.Package, "base Java package (default: derived from the coordinates)")
	description := flags.String("description", cfg.Description, "project description")
	vaadinVersion := flags.String("vaadin-version", "", "Vaadin version (default: the newest release found on Maven Central)")
	bootVersion := flags.String("boot-version", "", "Spring Boot version (default: the newest release found on Maven Central)")
	javaVersion := flags.String("java-version", cfg.JavaVersion, "JDK major version the build pins")
	outputDir := flags.String("dir", "", "where to write the project (default: the artifact id)")

	database := flags.Bool("database", cfg.Database, "PostgreSQL, Flyway, JPA, Testcontainers and a dev compose file")
	auth := flags.Bool("auth", cfg.Auth, "OIDC login against Keycloak in the dev stack")
	e2e := flags.Bool("e2e", cfg.E2E, "Playwright browser tests behind an it profile")
	coverage := flags.Bool("coverage", cfg.Coverage, "a JaCoCo coverage gate")
	traceable := flags.Bool("traceable", cfg.Traceable, "require every build to carry its commit SHA")

	yes := flags.Bool("yes", false, "skip the prompts and generate from the flags and defaults")
	force := flags.Bool("force", false, "write into the target directory even if it is not empty")
	noGit := flags.Bool("no-git", false, "do not touch git at all: no init, no hook, no commit")
	noCommit := flags.Bool("no-commit", false, "set the repository up but leave the first commit to you")
	dryRun := flags.Bool("dry-run", false, "list the files that would be written, and write nothing")
	accessible := flags.Bool("accessible", os.Getenv("ACCESSIBLE") != "", "ask the questions as plain sequential prompts, for screen readers")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; every setting is a flag (see --help)", flags.Arg(0))
	}

	// Start the lookup now so it overlaps with the questions.
	lookup := startLookup()

	cfg.GroupID = *groupID
	cfg.ArtifactID = *artifactID
	cfg.Description = *description
	cfg.JavaVersion = *javaVersion
	cfg.Database, cfg.Auth = *database, *auth
	cfg.E2E, cfg.Coverage, cfg.Traceable = *e2e, *coverage, *traceable

	// The derived answers follow the coordinates unless the user named one
	// explicitly. Without this, --artifact-id on its own would generate a project
	// whose package still carries the defaults file's example name.
	set := setFlags(flags)
	if set["name"] {
		cfg.ProjectName = *name
	} else {
		cfg.ProjectName = config.DeriveProjectName(cfg.ArtifactID)
	}
	if set["package"] {
		cfg.Package = *pkg
	} else {
		cfg.Package = config.DerivePackage(cfg.GroupID, cfg.ArtifactID)
	}
	if set["dir"] {
		cfg.OutputDir = *outputDir
	} else {
		cfg.OutputDir = cfg.ArtifactID
	}
	if set["vaadin-version"] {
		cfg.VaadinVersion = *vaadinVersion
	}
	if set["boot-version"] {
		cfg.BootVersion = *bootVersion
	}

	// A terminal is what makes the full-screen form possible, so without one the
	// questions are skipped rather than asked into a pipe. --accessible is the
	// exception: it asks in plain lines, which is exactly what works over a pipe,
	// and asking for it is asking to be asked.
	interactive := !*yes && (*accessible ||
		(isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stdin.Fd())))
	if interactive {
		// Not in accessible mode: a rule drawn down the left of two lines is
		// decoration, and a screen reader has to read it out before reaching the
		// first question.
		if !*accessible {
			fmt.Print(ui.Banner(version,
				strconv.Itoa(versions.VaadinMajor), strconv.Itoa(versions.BootMajor)))
		}
		cfg, err = prompt.Run(cfg, lookup, prompt.Options{Accessible: *accessible})
		if err != nil {
			return err
		}
	} else {
		// Outside the TUI the lookup still supplies the version defaults, so a
		// scripted run and an interactive one start from the same numbers.
		available := lookup()
		if !set["vaadin-version"] {
			if latest := versions.Latest(available.Vaadin); latest != "" {
				cfg.VaadinVersion = latest
			}
		}
		if !set["boot-version"] {
			if latest := versions.Latest(available.Boot); latest != "" {
				cfg.BootVersion = latest
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	templates, err := fs.Sub(templateFS, "templates")
	if err != nil {
		return err
	}
	generator := generate.New(templates)

	if *dryRun {
		return printDryRun(generator, cfg)
	}

	result, err := generator.Write(cfg, generate.WriteOptions{
		Force:  *force,
		Git:    !*noGit,
		Commit: !*noGit && !*noCommit,
	})
	if err != nil {
		return err
	}
	printResult(cfg, result)
	return nil
}

// preParse reads the flags that change how the rest of the flags are defined.
//
// It parses a copy so that an unknown flag here is not an error: the real parse
// has the full set and is the one entitled to complain about a typo.
func preParse(flags *flag.FlagSet, args []string) error {
	quiet := flag.NewFlagSet("pre", flag.ContinueOnError)
	quiet.SetOutput(discard{})
	flags.VisitAll(func(f *flag.Flag) { quiet.Var(f.Value, f.Name, f.Usage) })
	if err := quiet.Parse(args); err != nil && !errors.Is(err, flag.ErrHelp) {
		// An unknown flag is the expected outcome here and is left to the real
		// parse; anything else is worth reporting now.
		if !strings.Contains(err.Error(), "flag provided but not defined") {
			return err
		}
	}
	return nil
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// setFlags is the set of flag names the user actually typed, which is the only
// way to tell "--java-version 21" from a default that happens to be 21.
func setFlags(flags *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// startLookup runs the version lookup in the background and returns the function
// that waits for it. The lookup happens once however many times that function is
// called, so the interactive and scripted paths can both just ask.
func startLookup() prompt.VersionSource {
	client := &http.Client{Timeout: lookupTimeout}
	done := make(chan versions.Available, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
		defer cancel()
		done <- versions.Lookup(ctx, client)
	}()

	var once sync.Once
	var available versions.Available
	return func() versions.Available {
		once.Do(func() { available = <-done })
		return available
	}
}

func printDryRun(generator *generate.Generator, cfg config.Config) error {
	files, err := generator.Render(cfg)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path)
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Heading(fmt.Sprintf("%d files would be written", len(files))))
	fmt.Println()
	fmt.Println(ui.FileTree(root, paths))
	return nil
}

// displayPath is how a path is written for a person: relative to where they are
// if it is under there, absolute otherwise.
//
// The absolute path is correct and, for the directory just created under the
// current one, useless — it is mostly the part the user already knows, and it is
// what they would have to delete from a `cd` line before running it.
func displayPath(path string) string {
	working, err := os.Getwd()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(working, path)
	if err != nil || relative == "" || strings.HasPrefix(relative, "..") {
		return path
	}
	return relative
}

func printResult(cfg config.Config, result generate.Result) {
	options := "none — core only"
	if on := cfg.Selected(); len(on) > 0 {
		options = ui.Join(on...)
	}

	git := "not initialised"
	if result.GitInit {
		parts := []string{"initialised"}
		if result.HooksPath {
			parts = append(parts, "commit-msg hook wired up")
		}
		if result.Committed {
			parts = append(parts, "first commit made")
		}
		git = ui.Join(parts...)
	}

	rows := []ui.Row{
		{Label: "where", Value: fmt.Sprintf("%s  (%d files)", displayPath(result.Root), len(result.Paths))},
		{Label: "stack", Value: ui.Join(
			"Vaadin "+cfg.VaadinVersion,
			"Spring Boot "+cfg.BootVersion,
			"Java "+cfg.JavaVersion)},
		{Label: "options", Value: options},
		{Label: "git", Value: git},
	}

	fmt.Print(ui.Summary(cfg.ProjectName+" is ready", rows, result.GitMessage))

	steps := []ui.Step{{Command: "cd " + displayPath(result.Root)}}
	if cfg.ContainerRequired() {
		steps = append(steps, ui.Step{Command: "./run.sh env", Purpose: "bring up the development stack"})
	}
	steps = append(steps,
		ui.Step{Command: "./run.sh run", Purpose: "start the application"},
	)
	if cfg.E2E {
		steps = append(steps, ui.Step{Command: "./run.sh verify", Purpose: "unit tests and integration tests"})
	} else {
		steps = append(steps, ui.Step{Command: "./run.sh test", Purpose: "the unit tests"})
	}
	steps = append(steps, ui.Step{Command: "./run.sh help", Purpose: "every task"})

	fmt.Println()
	fmt.Print(ui.NextSteps("Next", steps))
	fmt.Println()
}

func usage(flags *flag.FlagSet) {
	out := flags.Output()
	fmt.Fprintln(out, "vaadin-init bootstraps an opinionated Vaadin and Spring Boot project.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  vaadin-init                     ask, then generate")
	fmt.Fprintln(out, "  vaadin-init --yes [flags]       generate without asking")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Flags (defaults shown are this machine's, after the defaults file):")
	flags.PrintDefaults()
}
