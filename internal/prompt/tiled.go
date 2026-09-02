package prompt

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/binarycodes/vaadin-init/internal/ui"
)

// section is one part of the conversation: a heading, the questions asked under
// it, and — for the escape hatches — whether it is being asked at all.
//
// The fields are kept beside the group because huh hands neither back: the
// layout has to know which section the cursor is in to draw the others as
// inactive, and the identity of the focused field is the only way to tell.
type section struct {
	title       string
	description string
	group       *huh.Group
	fields      []huh.Field
	hide        func() bool

	// span gives this section a row of its own, the width of the screen, under
	// the columns. Where a section belongs is a question about the questions —
	// the directory the project goes in, and the button that starts the whole
	// thing, follow everything above them rather than sitting beside one column
	// of it — and not something a layout can work out from heights.
	span bool
}

// newSection builds the group and keeps hold of what the layout will need.
//
// The title and description are given to the group as well as kept here, so that
// a section still introduces itself when the terminal is too small to tile and
// the sections are shown one at a time by huh's own layout.
func newSection(title, description string, hide func() bool, fields ...huh.Field) section {
	group := huh.NewGroup(fields...).Title(title).Description(description)
	if hide != nil {
		group = group.WithHideFunc(hide)
	}
	return section{
		title:       title,
		description: description,
		group:       group,
		fields:      fields,
		hide:        hide,
	}
}

// shown reports whether this section is being asked. An escape hatch that has
// not been reached for is not a column: it would be an empty heading taking
// width from the sections that do have something to say.
func (s section) shown() bool { return s.hide == nil || !s.hide() }

func (s section) holds(field huh.Field) bool {
	for _, candidate := range s.fields {
		if candidate == field {
			return true
		}
	}
	return false
}

// The gap between columns, and the narrowest a column may be before tiling is
// not worth doing. Below this, "Spring Boot version" is three lines of heading
// above a list of versions that no longer fit on one line each, and one question
// at a time is the better screen.
const (
	// The gap between boxes. One column, because the borders already say where
	// one section stops and the next starts; more would only be width the
	// questions do not get.
	columnGap = 1
	minColumn = 26

	// What the theme puts to the left of a field: the bar marking the one being
	// answered, and the space after it. Taken out of the width the questions are
	// given, because a field laid out to the full width of its column is two
	// columns wider than that once the bar is drawn — and the wrapping that
	// follows tears the bar off the lines it has pushed down.
	fieldFrame = 2
)

// tiled puts every section on screen at once, side by side.
//
// The point is the whole picture: the coordinates, the versions and the stack
// are one decision made together, and reviewing them meant walking back through
// the questions one at a time and trusting memory for the rest. Tiled, the form
// is its own summary, and the confirm at the end is the only thing left to do.
//
// A layout of this package's own rather than huh's LayoutColumns because that
// one shows the sections a page at a time, drops every section's heading but the
// active one, and renders a hidden group as an empty column.
type tiled struct {
	sections []section

	// column is the width each section was last given, perRow how many of them go
	// side by side, and width the whole terminal a spanning section is drawn to.
	// All three are worked out when the form asks for a group's width, which is
	// the only moment the terminal's own width is offered, and remembered for the
	// drawing that follows.
	column int
	perRow int
	width  int
}

func (l *tiled) shown() []section {
	kept := make([]section, 0, len(l.sections))
	for _, s := range l.sections {
		if s.shown() {
			kept = append(kept, s)
		}
	}
	return kept
}

// active is the index, among the sections on screen, of the one the cursor is
// in. Negative if the form has not focused anything yet.
func (l *tiled) active(f *huh.Form) int {
	field := f.GetFocusedField()
	if field == nil {
		return -1
	}
	for i, s := range l.shown() {
		if s.holds(field) {
			return i
		}
	}
	return -1
}

// columns are the sections that stand side by side, and rows the ones that run
// under them the whole way across.
func (l *tiled) columns() (columns, rows []int) {
	for i, s := range l.shown() {
		if s.span {
			rows = append(rows, i)
			continue
		}
		columns = append(columns, i)
	}
	return columns, rows
}

// GroupWidth divides the terminal between the columns on screen.
//
// As many side by side as will stay readable, and the rest wrapped underneath. A
// terminal wide enough for every column gets one row of them; a narrower one gets
// fewer and taller, which is still the whole picture and still one screen. Every
// column is the same width, so a short last one simply ends early rather than
// stretching to fill.
func (l *tiled) GroupWidth(_ *huh.Form, g *huh.Group, w int) int {
	columns, _ := l.columns()
	if len(columns) <= 0 {
		return w
	}

	l.perRow = len(columns)
	for l.perRow > 1 && width(w, l.perRow) < minColumn {
		l.perRow--
	}
	l.column = width(w, l.perRow)
	l.width = w

	// A section with a row to itself is as wide as the screen, whatever the
	// columns above it came to.
	for _, s := range l.shown() {
		if s.span && s.group == g {
			return w - ui.SectionFrame - fieldFrame
		}
	}
	return l.column - ui.SectionFrame - fieldFrame
}

// width is what each of n columns gets out of w, once the gaps between them are
// taken out.
func width(w, n int) int {
	return (w - columnGap*(n-1)) / n
}

// pack puts the sections into columns, in the order they are asked, each column
// taking as near as it can get to an equal share of the height.
//
// A grid of even rows would be simpler and worse: the sections are not the same
// size, so a row is as tall as the tallest thing in it and the short sections
// leave holes underneath themselves. Filling each column instead — the stack
// under the output, say — is what lets a terminal one column narrower than five
// still show everything.
func pack(heights []int, columns int) [][]int {
	if columns >= len(heights) {
		single := make([][]int, 0, len(heights))
		for i := range heights {
			single = append(single, []int{i})
		}
		return single
	}

	total := len(heights) - 1 // the blank line between two stacked sections
	for _, h := range heights {
		total += h
	}
	target := (total + columns - 1) / columns

	var packed [][]int
	var current []int
	height := 0

	for i, h := range heights {
		full := len(current) > 0 && height+1+h > target
		room := len(packed) < columns-1 && len(heights)-i >= columns-len(packed)-1
		if full && room {
			packed = append(packed, current)
			current, height = nil, 0
		}
		if len(current) > 0 {
			height++
		}
		current = append(current, i)
		height += h
	}
	return append(packed, current)
}

// Fits reports whether the sections can be tiled in the space given.
//
// Asked by rendering rather than by estimating: what a section costs depends on
// how its headings wrap and how many versions the lookup returned, and a guess
// that is wrong either wastes the screen or cuts the last option off a list.
func (l *tiled) Fits(f *huh.Form, width, height int) bool {
	if l.column < minColumn {
		return false
	}
	view := l.View(f)
	return lipgloss.Height(view) <= height && lipgloss.Width(view) <= width
}

// columnHeight is what a column of stacked units comes to.
func columnHeight(heights []int, indexes []int) int {
	total := 0
	for _, i := range indexes {
		total += heights[i]
	}
	return total
}

func (l *tiled) View(f *huh.Form) string {
	shown := l.shown()
	if len(shown) == 0 {
		return ""
	}
	active := l.active(f)
	columns, rows := l.columns()

	// Drawn once to find out how tall each section wants to be, because that is
	// what decides how they are shared out between the columns.
	box := func(i, height int) string {
		width := l.column
		if shown[i].span {
			width = l.width
		}
		return ui.SectionBox(shown[i].title, shown[i].description, shown[i].group.Content(),
			i == active, width, height)
	}

	heights := make([]int, 0, len(columns))
	for _, i := range columns {
		heights = append(heights, lipgloss.Height(box(i, 0)))
	}

	perRow := l.perRow
	if perRow < 1 {
		perRow = len(columns)
	}
	packed := pack(heights, perRow)

	tallest := 0
	for _, indexes := range packed {
		tallest = max(tallest, columnHeight(heights, indexes))
	}

	var side []string
	for _, indexes := range packed {
		// The slack goes to the last box in the column, so every column ends on
		// the same line and the row reads as one thing rather than four.
		slack := tallest - columnHeight(heights, indexes)

		var stacked []string
		for at, unit := range indexes {
			height := 0
			if at == len(indexes)-1 && slack > 0 {
				height = heights[unit] + slack
			}
			stacked = append(stacked, box(columns[unit], height))
		}

		column := strings.Join(stacked, "\n")
		if len(side) > 0 {
			column = lipgloss.NewStyle().MarginLeft(columnGap).Render(column)
		}
		side = append(side, column)
	}

	// Top, not centre: the names sit in the top edge of each box, and a short
	// column floated to the middle of a tall one takes its name with it.
	view := lipgloss.JoinHorizontal(lipgloss.Top, side...)

	for _, i := range rows {
		view += "\n" + box(i, 0)
	}

	// A validation error belongs to the section being answered, and goes under
	// the boxes rather than inside one: it is about the answer just given, and a
	// box that grows by a line when an answer is refused moves everything beside
	// it. (The help that huh would put here is drawn in the bar instead.)
	if active >= 0 {
		if footer := shown[active].group.Footer(); footer != "" {
			view += "\n\n" + footer
		}
	}
	return view
}
