// Package ui holds everything about how vaadin-init looks: one palette, one huh
// theme, and the renderers for what the tool prints around the form.
//
// It lives apart from the prompt and from main so that the form, the banner and
// the closing summary cannot drift into looking like three different tools. A
// colour or a glyph is defined once here and used by name everywhere else.
package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// The palette.
//
// Every colour is adaptive, because a terminal's background is not knowable in
// advance and a single hex value that reads well on one is unreadable on the
// other — mid-grey on white, or deep blue on black.
//
// Each is also complete: the value for a 16-colour terminal is stated rather
// than derived. Left to a nearest-match, every grey here lands on bright blue,
// because that is genuinely the closest of sixteen — and the result is a form
// with no hierarchy at all, worse than having chosen no colours. Terminals that
// report sixteen colours are common enough (plenty of CI, and any TERM=xterm)
// that this is a case to answer rather than to hope about.
//
// lipgloss still handles the ends of the range on its own: NO_COLOR or a dumb
// terminal gets no escape codes, so nothing here needs a monochrome fallback.
var (
	// Accent carries the tool's identity: titles, the active field, commands the
	// user is meant to type next.
	Accent = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#0B5FCC", ANSI256: "26", ANSI: "4"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#5AB0FF", ANSI256: "75", ANSI: "12"},
	}

	// Success marks what happened rather than what to do — a selected option, a
	// finished run.
	Success = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#0F7B3F", ANSI256: "29", ANSI: "2"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#4ADE80", ANSI256: "78", ANSI: "10"},
	}

	// Danger is only ever a validation failure.
	Danger = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#B42318", ANSI256: "124", ANSI: "1"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#FF8A80", ANSI256: "210", ANSI: "9"},
	}

	// Muted is supporting prose: descriptions, labels, the help line. On sixteen
	// colours it is plain white or black — the default foreground — since the
	// distinction it carries is worth less than being legible.
	Muted = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#5C6773", ANSI256: "243", ANSI: "0"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#8B95A5", ANSI256: "247", ANSI: "7"},
	}

	// Faint is for text that should be legible only when looked for: an inactive
	// field, an unselected option, a path.
	Faint = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#8A93A0", ANSI256: "247", ANSI: "8"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#6B7280", ANSI256: "243", ANSI: "8"},
	}

	// Border is structure, never content.
	Border = lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "#C9D0DA", ANSI256: "251", ANSI: "8"},
		Dark:  lipgloss.CompleteColor{TrueColor: "#39414F", ANSI256: "238", ANSI: "8"},
	}
)

// Caret is the prompt a text field shows. Exported because the field owns its
// prompt string, not the theme — the theme only colours it.
const Caret = "❯ "

const (
	selected   = "✓ "
	unselected = "· "
	crossmark  = " ✗"
	bullet     = " · "
)

// Theme is the form's look.
//
// Built on huh.ThemeBase rather than one of the shipped themes: the base sets
// the structural styles — the thick left border on the active field, the
// separators — and leaves the colours unset, which is exactly the split wanted
// here. Starting from ThemeCharm would mean overriding its palette everywhere
// and inheriting whatever it changes next.
func Theme() *huh.Theme {
	t := huh.ThemeBase()

	t.Group.Title = lipgloss.NewStyle().Bold(true).Foreground(Accent).MarginBottom(0)
	t.Group.Description = lipgloss.NewStyle().Foreground(Muted).MarginBottom(1)

	// The active field. Its left border is the same weight and colour as the
	// banner's, so the eye reads them as the same object speaking.
	t.Focused.Base = t.Focused.Base.BorderForeground(Accent)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	t.Focused.NoteTitle = t.Focused.Title
	t.Focused.Description = lipgloss.NewStyle().Foreground(Muted)
	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(Danger).SetString(crossmark)
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(Danger)

	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(Accent).SetString(Caret)
	t.Focused.MultiSelectSelector = t.Focused.SelectSelector
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(Success).SetString(selected)
	t.Focused.SelectedOption = lipgloss.NewStyle()
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(Faint).SetString(unselected)
	t.Focused.UnselectedOption = lipgloss.NewStyle()
	t.Focused.Option = lipgloss.NewStyle()
	t.Focused.NextIndicator = lipgloss.NewStyle().Foreground(Faint).MarginLeft(1).SetString("→")
	t.Focused.PrevIndicator = lipgloss.NewStyle().Foreground(Faint).MarginRight(1).SetString("←")

	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(Accent)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(Accent)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(Faint)
	t.Focused.TextInput.Text = lipgloss.NewStyle()

	t.Focused.FocusedButton = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(Accent).
		Padding(0, 2).
		MarginRight(1).
		Bold(true)
	t.Focused.BlurredButton = lipgloss.NewStyle().
		Foreground(Muted).
		Padding(0, 2).
		MarginRight(1)

	// Everything not being answered right now recedes: same layout, less
	// contrast, no border. Copied from Focused first so a style added above is
	// inherited rather than silently missing here.
	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = lipgloss.NewStyle().Foreground(Faint)
	t.Blurred.Description = lipgloss.NewStyle().Foreground(Faint)
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(Faint)
	t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(Muted)
	t.Blurred.MultiSelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Help.ShortKey = lipgloss.NewStyle().Foreground(Muted)
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(Faint)
	t.Help.ShortSeparator = lipgloss.NewStyle().Foreground(Border)
	t.Help.FullKey = t.Help.ShortKey
	t.Help.FullDesc = t.Help.ShortDesc
	t.Help.FullSeparator = t.Help.ShortSeparator

	return t
}
