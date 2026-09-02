package prompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/binarycodes/vaadin-init/internal/config"
	"github.com/binarycodes/vaadin-init/internal/ui"
)

// written brings a screen up with somewhere for the answers to go, agrees to
// generate, and runs the writing the way the runtime would.
func generated(t *testing.T, generate func(config.Config) (Outcome, error)) *screen {
	t.Helper()
	return generatedWith(t, generate, nil)
}

// generatedWith is the same, with somewhere for the tasks named in the command
// bar to run.
func generatedWith(t *testing.T, generate func(config.Config) (Outcome, error),
	runner func(context.Context, string, io.Writer) error,
) *screen {
	t.Helper()

	c := seed()
	s := newScreen(&c, offered(), Options{Banner: banner, Generate: generate, Task: runner})
	s.Init()
	s.Update(versionsMsg(fetched()))
	s.Update(tea.WindowSizeMsg{Width: wide, Height: tall})

	press(s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true})
	s.form.NextGroup() // agreeing to generate is being stepped past the last section

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("nothing was asked for once the questions were done")
	}
	if msg, ok := cmd().(writtenMsg); ok {
		s.Update(msg)
	}
	return s
}

func ready() Outcome {
	return Outcome{
		Title: "My App is ready",
		Rows: []ui.Row{
			{Label: "where", Value: "my-app  (22 files)"},
			{Label: "git", Value: "initialised, first commit made"},
		},
		Steps: []ui.Step{
			{Command: "cd my-app"},
			{Command: "./run.sh run", Purpose: "start the application"},
		},
	}
}

// The project is written without leaving the screen it was asked for on: same
// banner, same boxes, same bar along the bottom.
//
// The alternative is what the tool used to do — hand the terminal back and print
// into whatever was behind it — which reads as the program having ended and
// something else having started.
func TestTheProjectIsWrittenOnTheSameScreen(t *testing.T) {
	var got config.Config
	s := generated(t, func(c config.Config) (Outcome, error) {
		got = c
		return ready(), nil
	})

	if got.ArtifactID != "my-app" {
		t.Errorf("the answers did not reach the writing: %+v", got)
	}
	if s.closing() {
		t.Fatal("the screen closed as soon as the questions were done")
	}

	on := view(s)
	if !strings.Contains(on, "vaadin-init") {
		t.Error("the banner is gone, so this looks like a different program")
	}
	for _, want := range []string{"My App is ready", "my-app  (22 files)", "Next", "./run.sh run"} {
		if !strings.Contains(on, want) {
			t.Errorf("%q is not on the screen", want)
		}
	}
	if !strings.Contains(on, "╭─") {
		t.Error("the summary is not in the boxes the questions were in")
	}

	lines := strings.Split(on, "\n")
	if len(lines) != tall {
		t.Errorf("the screen is %d rows, not the %d it had a moment ago", len(lines), tall)
	}
	if !strings.Contains(lines[len(lines)-1], "run.sh") {
		t.Errorf("the bottom row is not the command bar, it is %q", lines[len(lines)-1])
	}
}

// A task named in the bar runs from inside the screen, and everything it says
// lands in the log.
//
// The tool used to hand the terminal to the task and be gone, which made the
// first command the last thing it did. The log is what lets there be a second
// one.
func TestATaskRunsIntoTheLog(t *testing.T) {
	var ran string
	s := generatedWith(t,
		func(config.Config) (Outcome, error) { return ready(), nil },
		func(_ context.Context, task string, out io.Writer) error {
			ran = task
			fmt.Fprintln(out, "building the thing")
			fmt.Fprintln(out, "built the thing")
			return nil
		})

	typeIn(s, "test")
	drive(t, s, tea.KeyMsg{Type: tea.KeyEnter})

	if ran != "test" {
		t.Errorf("the task that ran was %q", ran)
	}
	if s.closing() {
		t.Fatal("running a task should not end the screen")
	}
	if s.running {
		t.Error("the task is over, so the bar should be taking commands again")
	}

	on := view(s)
	for _, want := range []string{"Log", "run.sh test", "building the thing", "built the thing", "done"} {
		if !strings.Contains(on, want) {
			t.Errorf("%q is not in the log", want)
		}
	}
	if !strings.Contains(on, "My App is ready") {
		t.Error("the summary went away when the task ran")
	}
	if got := s.command.Value(); got != "" {
		t.Errorf("the bar still holds %q, so the next command types onto the end of the last", got)
	}
}

// A task that fails says so where its output is, rather than taking the screen
// down with it.
func TestAFailedTaskIsJustALineInTheLog(t *testing.T) {
	s := generatedWith(t,
		func(config.Config) (Outcome, error) { return ready(), nil },
		func(context.Context, string, io.Writer) error { return errors.New("exit status 1") })

	typeIn(s, "verify")
	drive(t, s, tea.KeyMsg{Type: tea.KeyEnter})

	if s.closing() {
		t.Fatal("a task that failed should not end the screen")
	}
	if on := view(s); !strings.Contains(on, "exit status 1") {
		t.Error("the log does not say what went wrong")
	}
}

// A running task can be stopped without stopping the tool.
//
// ./run.sh run is a server: it does not finish, and the only way back to the bar
// is to stop it. Ctrl+c has to mean the task while one is running — it is the
// screen that hands the terminal back, so it cannot be the thing that goes.
func TestARunningTaskIsStoppedNotTheScreen(t *testing.T) {
	s := generatedWith(t,
		func(config.Config) (Outcome, error) { return ready(), nil },
		func(ctx context.Context, _ string, out io.Writer) error {
			fmt.Fprintln(out, "listening on 8080")
			<-ctx.Done()
			return ctx.Err()
		})

	typeIn(s, "run")
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !s.running {
		t.Fatal("the task is not running")
	}

	s.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	drain(t, s, cmd)

	if s.closing() {
		t.Fatal("ctrl+c stopped the screen, not the task")
	}
	if s.running {
		t.Error("the task is still running")
	}
	on := view(s)
	for _, want := range []string{"listening on 8080", "stopped"} {
		if !strings.Contains(on, want) {
			t.Errorf("%q is not in the log", want)
		}
	}
}

// The result fills the terminal exactly, log and all.
func TestTheLogTakesTheRoomThatIsLeft(t *testing.T) {
	s := generatedWith(t,
		func(config.Config) (Outcome, error) { return ready(), nil },
		func(_ context.Context, _ string, out io.Writer) error {
			for i := range 200 {
				fmt.Fprintf(out, "line %d\n", i)
			}
			return nil
		})

	typeIn(s, "test")
	drive(t, s, tea.KeyMsg{Type: tea.KeyEnter})

	lines := strings.Split(view(s), "\n")
	if len(lines) != tall {
		t.Errorf("the screen is %d rows, not the %d it has to be", len(lines), tall)
	}
	if width := widest(view(s)); width > wide {
		t.Errorf("the screen is %d columns wide, not the %d it has to be", width, wide)
	}
	// The tail, because a task is watched as it runs.
	if !strings.Contains(view(s), "line 199") {
		t.Error("the log is not showing the end of the output")
	}
}

// drive presses a key and then runs everything it asked for, the way the
// runtime would.
func drive(t *testing.T, s *screen, key tea.KeyMsg) {
	t.Helper()
	_, cmd := s.Update(key)
	drain(t, s, cmd)
}

func drain(t *testing.T, s *screen, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 1000; i++ {
		msg := cmd()
		if msg == nil {
			return
		}
		_, cmd = s.Update(msg)
	}
}

// An empty bar is someone who has not decided yet.
//
// Enter is the key every other answer on this screen was given with, so a bare
// enter meaning "we are done here" is a way to leave by accident — and the way
// back is to run the tool again and answer every question a second time.
func TestAnEmptyCommandBarDoesNothing(t *testing.T) {
	s := generated(t, func(config.Config) (Outcome, error) { return ready(), nil })

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("pressing enter on an empty command bar should do nothing at all")
	}
	if s.closing() {
		t.Error("pressing enter on an empty command bar should not end the screen")
	}
}

// Leaving is a word, and either of the two anyone would try.
func TestTheCommandBarIsLeftByName(t *testing.T) {
	for _, word := range []string{"quit", "exit", "Quit", " exit "} {
		s := generated(t, func(config.Config) (Outcome, error) { return ready(), nil })

		typeIn(s, word)
		_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if cmd == nil {
			t.Fatalf("%q does not leave the command bar", word)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q does not leave the command bar", word)
		}
		if s.running {
			t.Errorf("%q was taken as a task to run", word)
		}
	}
}

// A project that could not be written is the caller's problem, not something to
// show a command bar about.
func TestAFailedWriteEndsTheScreen(t *testing.T) {
	failure := errors.New("the directory is not empty")
	s := generated(t, func(config.Config) (Outcome, error) { return Outcome{}, failure })

	if !errors.Is(s.failure, failure) {
		t.Errorf("failure = %v, want the one the writing gave", s.failure)
	}
	if !s.closing() {
		t.Error("the screen should be on its way out")
	}
}

// The command bar can be left the way anything else can.
//
// Its own business, because the form's keymap went out of scope with the last
// question: without this the only way out of the bar is to name a task.
func TestTheCommandBarCanBeLeft(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		s := generated(t, func(config.Config) (Outcome, error) { return ready(), nil })

		_, cmd := s.Update(tea.KeyMsg{Type: key})
		if cmd == nil {
			t.Fatalf("%v does not leave the command bar", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v does not leave the command bar", key)
		}
		if s.running {
			t.Errorf("%v started a task", key)
		}
	}
}
