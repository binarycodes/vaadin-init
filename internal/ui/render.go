package ui

import (
	"fmt"
	"sort"
	"strings"

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
	width := 0
	for _, r := range rows {
		if len(r.Label) > width {
			width = len(r.Label)
		}
	}
	labels := width + 2

	var body strings.Builder
	body.WriteString(successStyle.Render("✓ " + title))
	body.WriteString("\n")
	for _, r := range rows {
		body.WriteString("\n")
		body.WriteString(labelStyle.Width(labels).Render(r.Label))
		// Wrapped, and every line after the first indented to the value column,
		// so that a long value grows the box downwards rather than sideways and
		// still reads as one value. One absolute path is otherwise enough to push
		// the border past the edge of the terminal.
		body.WriteString(hangingIndent(
			valueStyle.Width(contentWidth-labels).Render(r.Value), labels))
	}
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

// NextSteps is the list of commands that follow. The commands are aligned and
// accented because they are the part meant to be typed; the purpose is muted
// because it is the part meant to be skimmed once.
func NextSteps(heading string, steps []Step) string {
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
	out.WriteString("  " + headingStyle.Render(heading) + "\n")
	for _, s := range steps {
		if s.Purpose == "" {
			out.WriteString("    " + commandStyle.Render(s.Command) + "\n")
			continue
		}
		out.WriteString("    " + commandStyle.Width(width+2).Render(s.Command))
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
