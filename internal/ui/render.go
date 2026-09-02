package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
)

// leftBar is the tool's mark: a thick accent rule down the left of a block. The
// active field in the form wears the same one, so the banner and the question
// being asked read as one voice rather than two.
var leftBar = lipgloss.NewStyle().
	Border(lipgloss.ThickBorder(), false, false, false, true).
	BorderForeground(Accent).
	PaddingLeft(1)

var (
	nameStyle    = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	versionStyle = lipgloss.NewStyle().Foreground(Faint)
	taglineStyle = lipgloss.NewStyle().Foreground(Muted)
	labelStyle   = lipgloss.NewStyle().Foreground(Faint)
	valueStyle   = lipgloss.NewStyle()
	pathStyle    = lipgloss.NewStyle().Foreground(Muted)
	commandStyle = lipgloss.NewStyle().Foreground(Accent)
	headingStyle = lipgloss.NewStyle().Bold(true).Foreground(Muted)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(Success)
	noticeStyle  = lipgloss.NewStyle().Foreground(Danger)
)

// Banner introduces the tool, once, above the first question.
func Banner(version, vaadinGeneration, bootGeneration string) string {
	title := nameStyle.Render("vaadin-init") + " " + versionStyle.Render(version)
	tagline := taglineStyle.Render(fmt.Sprintf(
		"An opinionated Vaadin %s and Spring Boot %s project.",
		vaadinGeneration, bootGeneration))

	return "\n" + leftBar.Render(title+"\n"+tagline) + "\n"
}

// Row is one line of the closing summary: a label and what it says.
type Row struct {
	Label string
	Value string
}

// Summary is the box printed when a project has been written.
//
// A box rather than a run of lines because it is the one thing the user came for
// and it lands at the bottom of whatever the terminal was already showing. The
// labels are aligned to a single column so the values can be read down.
func Summary(title string, rows []Row, notice string) string {
	var body strings.Builder
	body.WriteString(successStyle.Render("✓ " + title))
	body.WriteString("\n\n")
	body.WriteString(Fields(rows, contentWidth))
	if notice != "" {
		body.WriteString("\n\n")
		// Wrapped like the values above it: a notice is where a git error lands,
		// and git is not brief.
		body.WriteString(noticeStyle.Width(contentWidth).Render(notice))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(0, 2).
		Render(body.String())

	return "\n" + box + "\n"
}

// Fields is a run of labelled values, the labels aligned to one column so the
// values can be read down.
//
// Apart from the box it is usually printed in, because the same rows are shown
// twice: once inside the screen the project was asked for on, and once into the
// scrollback that is left behind when it exits.
func Fields(rows []Row, width int) string {
	labels := 0
	for _, r := range rows {
		if len(r.Label) > labels {
			labels = len(r.Label)
		}
	}
	labels += 2

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		// Wrapped, and every line after the first indented to the value column,
		// so that a long value grows the block downwards rather than sideways and
		// still reads as one value. One absolute path is otherwise enough to push
		// a border past the edge of the terminal.
		lines = append(lines, labelStyle.Width(labels).Render(r.Label)+
			hangingIndent(valueStyle.Width(max(width-labels, 1)).Render(r.Value), labels))
	}
	return strings.Join(lines, "\n")
}

// contentWidth is how wide the summary's contents may get before wrapping. Fixed
// rather than read from the terminal: this is printed as the program exits, into
// scrollback that may later be read at a different width, and a block that was
// wrapped to a wide window is unreadable in a narrow one.
const contentWidth = 72

// hangingIndent indents every line of an already-wrapped block except the first,
// which is the one that follows a label on the same row.
func hangingIndent(text string, width int) string {
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.Repeat(" ", width) + lines[i]
	}
	return strings.Join(lines, "\n")
}

// Step is one thing to do next: the command, and why anyone would run it.
type Step struct {
	Command string
	Purpose string
}

// NextSteps is the list of commands that follow, under a heading.
func NextSteps(heading string, steps []Step) string {
	return "  " + headingStyle.Render(heading) + "\n" + Commands(steps, "    ")
}

// Commands is the list of commands themselves, each indented by the given
// gutter. The commands are aligned and accented because they are the part meant
// to be typed; the purpose is muted because it is the part meant to be skimmed
// once.
func Commands(steps []Step, gutter string) string {
	// The column is set by the commands that have something written beside them.
	// A `cd` into a long absolute path has nothing beside it, and letting it set
	// the width pushes every purpose off to the right of an empty gutter.
	width := 0
	for _, s := range steps {
		if s.Purpose != "" && len(s.Command) > width {
			width = len(s.Command)
		}
	}

	var out strings.Builder
	for _, s := range steps {
		if s.Purpose == "" {
			out.WriteString(gutter + commandStyle.Render(s.Command) + "\n")
			continue
		}
		out.WriteString(gutter + commandStyle.Width(width+2).Render(s.Command))
		out.WriteString(taglineStyle.Render(s.Purpose))
		out.WriteString("\n")
	}
	return out.String()
}

// FileTree renders slash-separated paths as the directory tree they describe.
//
// A tree rather than the flat list these paths arrive as: the point of --dry-run
// is to see the shape of what would be written, and forty sorted paths sharing
// long prefixes is the one view that hides it.
//
// Chains of single-child directories are collapsed into one line, because a Java
// source tree is mostly such chains — src/main/java/io/binarycodes/noteharbor is
// six levels holding nothing but each other, and drawn in full it is a staircase
// that pushes the filenames off to the right for no information at all.
func FileTree(root string, paths []string) string {
	top := &directoryNode{}
	for _, path := range paths {
		top.add(strings.Split(path, "/"))
	}
	top.collapse()

	node := tree.Root(pathStyle.Render(root))
	top.attach(node)

	return node.
		Enumerator(tree.RoundedEnumerator).
		// A space after the branch: without it the glyphs run straight into the
		// filename and the column of names loses its left edge.
		EnumeratorStyle(lipgloss.NewStyle().Foreground(Border).PaddingRight(1)).
		String()
}

// directoryNode is one directory while the tree is being assembled: its
// subdirectories in the order first seen, and the files directly in it.
type directoryNode struct {
	name        string
	directories []*directoryNode
	files       []string
}

func (d *directoryNode) add(segments []string) {
	if len(segments) == 1 {
		d.files = append(d.files, segments[0])
		return
	}
	for _, existing := range d.directories {
		if existing.name == segments[0] {
			existing.add(segments[1:])
			return
		}
	}
	child := &directoryNode{name: segments[0]}
	d.directories = append(d.directories, child)
	child.add(segments[1:])
}

// collapse folds a directory that holds exactly one subdirectory and no files
// into that subdirectory, repeatedly, so the chain becomes one labelled line.
func (d *directoryNode) collapse() {
	for _, child := range d.directories {
		child.collapse()
	}
	for len(d.directories) == 1 && len(d.files) == 0 && d.name != "" {
		only := d.directories[0]
		d.name += "/" + only.name
		d.directories = only.directories
		d.files = only.files
	}
}

// attach renders this directory's contents under a tree node, directories
// first: a directory is a heading for what follows, and a file listed above one
// reads as belonging to it.
func (d *directoryNode) attach(node *tree.Tree) {
	sort.Slice(d.directories, func(i, j int) bool { return d.directories[i].name < d.directories[j].name })
	sort.Strings(d.files)

	for _, child := range d.directories {
		branch := tree.Root(labelStyle.Render(child.name + "/"))
		child.attach(branch)
		node.Child(branch)
	}
	for _, file := range d.files {
		node.Child(file)
	}
}

// Heading is a bare section title, for output that is a list rather than a box.
func Heading(text string) string {
	return headingStyle.Render(text)
}

// Cancelled is the message for a user who backed out.
func Cancelled() string {
	return taglineStyle.Render("Cancelled. Nothing was written.")
}

// Error formats a failure.
func Error(err error) string {
	return noticeStyle.Render("✗ " + err.Error())
}

// Join puts a middle dot between things that belong on one line.
func Join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, bullet)
}

// Section titles for the full-screen form, where every section is on screen at
// once and only one of them is being answered.
//
// The active one is accented and the rest recede, on the same principle the
// theme applies to fields: what the cursor is in should be the brightest thing
// on the screen, and the sections beside it are there to be read, not worked in.
var (
	sectionTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	sectionTitleRestStyle  = lipgloss.NewStyle().Bold(true).Foreground(Faint)
	sectionAboutStyle      = lipgloss.NewStyle().Foreground(Muted)
	sectionAboutRestStyle  = lipgloss.NewStyle().Foreground(Faint)
	boxStyle               = lipgloss.NewStyle().Foreground(Accent)
	boxRestStyle           = lipgloss.NewStyle().Foreground(Border)
	shortcutKeyStyle       = lipgloss.NewStyle().Foreground(Accent)
	shortcutKeyRestStyle   = lipgloss.NewStyle().Foreground(Muted)
	shortcutLabelStyle     = lipgloss.NewStyle().Foreground(Faint)
	shortcutSeparatorStyle = lipgloss.NewStyle().Foreground(Border)
)

// SectionFrame is what a section's box costs it in width: two borders, and the
// space inside each of them. Exported because the layout has to take it off the
// width before it tells the questions how wide they may be.
const SectionFrame = 4

// SectionBox is one section of the full-screen form: its questions in a box, with
// its name in the top edge of that box.
//
// A box rather than a heading over a column. Five columns of questions side by
// side are five things the eye has to keep apart on the strength of alignment
// alone, and alignment is exactly what a long wrapped answer breaks. A border
// says where a section ends whatever is inside it, and the name sitting in the
// border says which one it is without spending a row on a heading.
//
// The whole frame changes with focus, not just the title: the section being
// answered is the only accented thing on the screen, which is the same rule the
// theme applies to fields inside it.
//
// A height of zero is whatever the questions need. Given one, the box is drawn to
// it — which is how a row of boxes ends on the same line rather than in a ragged
// edge that reads as five unrelated shapes.
func SectionBox(title, description, content string, active bool, width, height int) string {
	frame, titleStyle, aboutStyle := boxRestStyle, sectionTitleRestStyle, sectionAboutRestStyle
	if active {
		frame, titleStyle, aboutStyle = boxStyle, sectionTitleStyle, sectionAboutStyle
	}

	border := lipgloss.RoundedBorder()
	inner := width - 2 // between the two upright borders
	text := inner - 2  // and inside the space kept either side of the text
	if text < 1 {
		return content
	}

	var body []string
	if description != "" {
		// No blank line after it: the border is what separates this box from the
		// next, and a row spent on air here is a row the questions do not get.
		body = append(body, strings.Split(aboutStyle.Width(text).Render(description), "\n")...)
	}
	body = append(body, strings.Split(content, "\n")...)
	for len(body) < height-2 {
		body = append(body, "")
	}

	// The name sits one cell in from the corner, and the rest of the edge is
	// drawn to whatever width is left.
	label := ""
	if title != "" {
		label = " " + title + " "
	}
	fill := max(inner-1-lipgloss.Width(label), 0)

	lines := []string{
		frame.Render(border.TopLeft+border.Top) + titleStyle.Render(label) +
			frame.Render(strings.Repeat(border.Top, fill)+border.TopRight),
	}
	for _, line := range body {
		// Padded by hand rather than by a style with a width: the content is
		// already styled, and re-rendering it at a width re-wraps it — which tears
		// the accent bar off the lines a field has pushed down.
		pad := max(text-lipgloss.Width(line), 0)
		lines = append(lines,
			frame.Render(border.Left)+" "+line+strings.Repeat(" ", pad)+" "+frame.Render(border.Right))
	}
	lines = append(lines,
		frame.Render(border.BottomLeft+strings.Repeat(border.Bottom, inner)+border.BottomRight))

	return strings.Join(lines, "\n")
}

// CommandInput is what the bar turns into once there is a project: the tool's
// caret, the script that will run, and the part left to type.
//
// The input itself is the caller's, because it is a live model that has to be
// given keys; what it looks like is this package's, like everything else on the
// screen.
func CommandInput() textinput.Model {
	in := textinput.New()
	in.Prompt = Caret + "run.sh " + Caret
	in.Placeholder = "a task name"
	in.PromptStyle = lipgloss.NewStyle().Foreground(Accent)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(Faint)
	in.Cursor.Style = lipgloss.NewStyle().Foreground(Accent)
	return in
}

// Echoed is a command as it goes into the log, above the output it produced.
func Echoed(command string) string {
	return commandStyle.Render(Caret + command)
}

// Finished is the line that closes a task's output off.
func Finished(text string) string {
	return successStyle.Render("· " + text)
}

// Failed is the same line for a task that did not work out.
func Failed(text string) string {
	return noticeStyle.Render("· " + text)
}

// Warning is a line that is not an error but should not be missed — a project
// written, but its git repository not.
func Warning(text string, width int) string {
	return noticeStyle.Width(width).Render(text)
}

// Working is what the bar says while the tool is doing something rather than
// waiting for a key.
func Working(text string) string {
	return taglineStyle.Render(text)
}

// Rule is the line that separates the bar at the bottom of the screen from the
// form above it. Drawn in the border colour, because it is structure: it says
// where the questions stop and the keys that move between them start.
func Rule(width int) string {
	if width <= 0 {
		return ""
	}
	return shortcutSeparatorStyle.Render(strings.Repeat("─", width))
}

// Shortcut is one key and where it goes.
type Shortcut struct {
	Key   string
	Label string
	// Active marks the section the cursor is already in, so the row of keys
	// doubles as the answer to "where am I".
	Active bool
}

// Bar is the line along the bottom of the screen: what the keys under the cursor
// do, and where the jump keys go.
//
// One line and one place, always. The keys that work here change with the field
// the cursor is in — enter to move on, x to toggle, ←/→ to choose — and huh draws
// them under whichever group is active, which on a screen of boxes means they
// move every time the cursor does.
func Bar(help string, items []Shortcut, width int) string {
	if help == "" {
		return Shortcuts(items, width)
	}
	separator := shortcutSeparatorStyle.Render(bullet)
	// The help goes first and is never dropped: it is the only thing on the line
	// that says what the key under the user's finger will do right now.
	rest := Shortcuts(items, width-lipgloss.Width(help)-lipgloss.Width(separator))
	if rest == "" {
		return help
	}
	return help + separator + rest
}

// Shortcuts is the row of keys under the form.
//
// Dropped from the end rather than wrapped when it does not fit: this line is a
// reminder of what is possible, and a second line of it costs a row of the form
// on exactly the terminals that have no rows to spare.
func Shortcuts(items []Shortcut, width int) string {
	separator := shortcutSeparatorStyle.Render(bullet)

	var out strings.Builder
	printed := 0
	for _, item := range items {
		keyStyle := shortcutKeyRestStyle
		if item.Active {
			keyStyle = shortcutKeyStyle
		}
		part := keyStyle.Render(item.Key) + " " + shortcutLabelStyle.Render(item.Label)

		next := part
		if printed > 0 {
			next = separator + part
		}
		if width > 0 && lipgloss.Width(out.String()+next) > width {
			break
		}
		out.WriteString(next)
		printed++
	}
	return out.String()
}
