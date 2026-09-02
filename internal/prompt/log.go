package prompt

import (
	"context"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/binarycodes/vaadin-init/internal/ui"
)

// How many lines of output are kept. A Maven build is thousands of lines and
// only the last screenful is ever shown, but a few screens of scrollback is what
// makes an error worth going back for — and this is the whole of what the tool
// holds in memory, so it is a number and not a promise.
const logLimit = 2000

// logLine is one line a running task has said.
type logLine string

// ranMsg is a task finishing, whatever it finished as.
type ranMsg struct{ err error }

// start runs a task and streams what it says into the log.
//
// The command is echoed into the log first: the log outlives the bar it was
// typed into, and a run of output with nothing above it saying what produced it
// is unreadable by the second command.
func (s *screen) start(task string) tea.Cmd {
	s.command.SetValue("")
	s.note(ui.Echoed("run.sh " + task))

	if s.runner == nil {
		s.note(ui.Failed("there is nothing here to run tasks with"))
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	lines := make(chan tea.Msg, 256)
	s.running, s.cancel, s.lines = true, cancel, lines
	s.current = "run.sh " + task
	s.command.Blur()

	go func() {
		out := &lineWriter{lines: lines}
		err := s.runner(ctx, task, out)
		// Flushed before the outcome is sent, so a task whose last line has no
		// newline still has it read before the line that says it is over.
		out.flush()
		lines <- ranMsg{err: err}
		close(lines)
	}()

	return s.await()
}

// await is the next thing the running task has to say.
//
// Drained in batches rather than one line per message: a build says thousands of
// lines and only the last screenful is ever drawn, so a redraw per line is work
// nobody sees. Whatever has already arrived is taken in one go, and the
// blocking read at the top is what keeps this cheap when nothing is happening.
func (s *screen) await() tea.Cmd {
	lines := s.lines
	return func() tea.Msg {
		first, ok := <-lines
		if !ok {
			return nil
		}
		if _, over := first.(ranMsg); over {
			return first
		}

		batch := []tea.Msg{first}
		for len(batch) < 512 {
			select {
			case next, ok := <-lines:
				if !ok {
					return batch
				}
				batch = append(batch, next)
				if _, over := next.(ranMsg); over {
					return batch
				}
			default:
				return batch
			}
		}
		return batch
	}
}

// logged takes what a running task has said, and says whether there is more to
// come.
func (s *screen) logged(msg tea.Msg) (more bool) {
	switch msg := msg.(type) {
	case logLine:
		s.note(string(msg))
		return true

	case []tea.Msg:
		for _, one := range msg {
			more = s.logged(one)
			if !more {
				return false
			}
		}
		return true

	case ranMsg:
		s.finished(msg.err)
		return false
	}
	return true
}

// finished puts the task's outcome at the end of its output and gives the bar
// back.
func (s *screen) finished(err error) {
	s.running, s.cancel, s.lines = false, nil, nil
	switch {
	case err == nil:
		s.note(ui.Finished("done"))
	case s.stopped:
		s.note(ui.Finished("stopped"))
	default:
		s.note(ui.Failed(err.Error()))
	}
	s.stopped = false
	s.command.Focus()
}

// stop asks a running task to stop. What it costs to leave it running is the
// terminal: this screen is the only thing that can hand it back.
func (s *screen) stop() {
	if s.cancel == nil {
		return
	}
	s.stopped = true
	s.cancel()
}

func (s *screen) note(line string) {
	s.log = append(s.log, line)
	if len(s.log) > logLimit {
		s.log = s.log[len(s.log)-logLimit:]
	}
}

// logView is the tail of the log, cut to the box it is drawn in.
//
// The tail, because a task is watched as it runs and what matters is the line
// that has just arrived. Lines are cut to the width rather than wrapped: build
// output is full of lines longer than any column, and wrapping them means one
// stack trace pushes everything that came before it off the top.
func (s *screen) logView(width, height int) string {
	if height < 1 || width < 1 {
		return ""
	}

	lines := s.log
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}

	cut := lipgloss.NewStyle().MaxWidth(width)
	shown := make([]string, 0, height)
	for _, line := range lines {
		shown = append(shown, cut.Render(strings.ReplaceAll(line, "\t", "    ")))
	}
	if len(shown) == 0 {
		shown = append(shown, ui.Working("output from a task appears here"))
	}
	return strings.Join(shown, "\n")
}

// lineWriter turns what a task writes into whole lines, and hands them over as
// they are finished.
//
// A writer rather than a scanner on a pipe, so that the lines arrive in the
// order they were written: the caller returns only once everything it was given
// has been written, which a scanner running in a goroutine of its own cannot
// promise.
//
// Locked because a task writes what it says and what went wrong down the same
// writer, from two goroutines.
type lineWriter struct {
	lines   chan<- tea.Msg
	mutex   sync.Mutex
	pending []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.pending = append(w.pending, p...)
	for {
		at := strings.IndexByte(string(w.pending), '\n')
		if at < 0 {
			return len(p), nil
		}
		w.lines <- logLine(strings.TrimRight(string(w.pending[:at]), "\r"))
		w.pending = w.pending[at+1:]
	}
}

// flush hands over a last line that never ended in a newline — a prompt, or a
// task killed mid-sentence.
func (w *lineWriter) flush() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if len(w.pending) > 0 {
		w.lines <- logLine(string(w.pending))
		w.pending = nil
	}
}
