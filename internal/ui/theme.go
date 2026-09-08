package ui

import "strings"

// What color means here comes from the logo: the prompt is cream and the
// bubble is green, so cream is you and green is them. Grey is homa itself
// talking — explanations, hints, the marks around a name — and the
// terminal's own yellow is a warning. Nothing else is colored: the words
// people type belong to them, not to the interface.
//
// Color is never the only signal. Every one of these paints a mark or a name
// that reads the same with color off, and the tests hold the two outputs to
// be the same text.

// you paints something that is yours: the prompt, your label, a key to press.
func (u *UI) you(s string) string { return paint(u.st, colorCream, s) }

// them paints the far side: a name, a message label, a file arriving.
func (u *UI) them(s string) string { return paint(u.st, colorGreen, s) }

// dim paints homa's own voice: information, hints, metadata.
func (u *UI) dim(s string) string { return paint(u.st, colorMuted, s) }

// warn paints a warning in the terminal's own yellow.
func (u *UI) warn(s string) string { return paint(u.st, colorWarn, s) }

// peer paints a name the way the far side is always shown: green, with the
// unknownMark in grey when the name is one they chose for themselves, so
// "this name is theirs, not yours" reads at a glance and the mark itself
// stays for anyone reading without color.
//
// The name is text the far side typed, already through session's sanitizer.
// It is wrapped in a color, never formatted into one; see the note on the
// color constants.
func (u *UI) peer(name string) string {
	if strings.HasPrefix(name, unknownMark) {
		return u.dim(unknownMark) + u.them(strings.TrimPrefix(name, unknownMark))
	}
	return u.them(name)
}

// meLabel is what sits in front of your own line in a conversation.
func (u *UI) meLabel() string {
	return u.dim("[") + u.you(selfNick) + u.dim("]") + " "
}

// promptMark is the mark in front of a question, painted as yours.
func (u *UI) promptMark() string { return u.you(markPrompt) }

// sep joins the parts of a status line, with a middle dot where the
// terminal can show one.
func (u *UI) sep() string {
	if u.st.unicode {
		return sepUnicode
	}
	return sepASCII
}
