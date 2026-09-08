package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// field is one question on a form: its label, what Enter alone answers,
// and what makes an answer acceptable.
type field struct {
	label string
	def   string             // shown in brackets; taken by an empty Enter
	check func(string) error // nil accepts anything; an error is shown under the field

	// word, when set, is the only answer that completes the field; any
	// other answer cancels the form. For the things there is no undo for,
	// a single letter is answered by reflex and a typed word is not.
	word string

	in  textinput.Model
	err string
}

// form is a few questions in a row, each a text input, walked with Enter.
// It replaces Ask, askUntilValid and ConfirmBy from version 1: the same
// wording, drawn in the frame instead of on a line.
type form struct {
	title  string
	warn   []string // lines said in yellow above the fields: what this will do
	lines  []string // lines said in grey above the fields
	fields []field
	cur    int
}

func newForm(title string, fields ...field) *form {
	f := &form{title: title, fields: fields}
	for i := range f.fields {
		in := textinput.New()
		in.Prompt = ""
		in.SetVirtualCursor(true)
		in.CharLimit = maxInputLen
		// As wide as the inside of its box: a long value — an address is
		// two hundred characters — scrolls inside rather than breaking the
		// box open, which the live tests caught on the screen grid.
		in.SetWidth(formFieldWidth - 4)
		f.fields[i].in = in
	}
	f.focus()
	return f
}

func (f *form) focus() {
	for i := range f.fields {
		f.fields[i].in.Blur()
	}
	_ = f.fields[f.cur].in.Focus()
}

// answers is what each field ended up with, defaults included.
func (f *form) answers() []string {
	out := make([]string, len(f.fields))
	for i := range f.fields {
		out[i] = f.answer(i)
	}
	return out
}

func (f *form) answer(i int) string {
	s := strings.TrimSpace(f.fields[i].in.Value())
	if s == "" {
		return f.fields[i].def
	}
	return s
}

// update takes a key. done is every field answered and accepted; cancel
// is the person backing out, which leaves everything as it was.
func (f *form) update(_ *styles, msg tea.Msg) (done, cancel bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, false
	}

	switch key.String() {
	case "esc":
		return false, true

	case keyEnter:
		fld := &f.fields[f.cur]
		got := f.answer(f.cur)

		if fld.word != "" && got != fld.word {
			return false, true
		}
		if fld.check != nil {
			if err := fld.check(got); err != nil {
				fld.err = trimPrefix(err)
				return false, false
			}
		}
		fld.err = ""

		if f.cur == len(f.fields)-1 {
			return true, false
		}
		f.cur++
		f.focus()
		return false, false
	}

	var cmd tea.Cmd
	f.fields[f.cur].in, cmd = f.fields[f.cur].in.Update(msg)
	_ = cmd // a text input's commands are cursor blinks; the cursor is virtual here
	return false, false
}

// view draws the title, any warning, then each field: its label above, the
// answer in a box — the one being answered with the cursor and a plain
// frame, the ones to come faint, the ones done showing what was answered.
func (f *form) view(st *styles) string {
	var b strings.Builder
	b.WriteString("\n  ")
	b.WriteString(st.you.Render(f.title))
	b.WriteString("\n\n")
	for _, w := range f.warn {
		b.WriteString("  " + st.warn.Render(w) + "\n")
	}
	for _, l := range f.lines {
		b.WriteString("  " + st.dim.Render(l) + "\n")
	}
	if len(f.warn)+len(f.lines) > 0 {
		b.WriteString("\n")
	}

	for i := range f.fields {
		fld := &f.fields[i]
		b.WriteString("  " + st.dim.Render(fld.label))
		if fld.def != "" && i >= f.cur {
			b.WriteString(st.dim.Render("  [" + fld.def + "]"))
		}
		b.WriteString("\n")

		var inside string
		frame := *st.boxDim()
		switch {
		case i < f.cur:
			inside = st.dim.Render(f.answer(i))
			frame = frame.Faint(true)
		case i == f.cur:
			inside = fld.in.View()
		default:
			inside = st.dim.Render(fld.def)
			frame = frame.Faint(true)
		}
		b.WriteString(
			"  " + strings.ReplaceAll(
				st.box(&frame, formFieldWidth, "", inside),
				"\n",
				"\n  ",
			) + "\n",
		)
		if i == f.cur && fld.err != "" {
			b.WriteString("  " + st.warn.Render(fld.err) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// trimPrefix drops the package prefix from an error before showing it. A
// person reading "display name is empty" does not need to know which Go
// package noticed.
func trimPrefix(err error) string {
	const prefix = "config: "

	s := err.Error()
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
