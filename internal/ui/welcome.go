package ui

import "strings"

// Welcome draws the first thing on the screen: the logo in block characters
// and color when the terminal can show them, and one plain line when it
// cannot or when output is not a terminal at all.
func (u *UI) Welcome() {
	u.mu.Lock()
	st := u.st
	u.mu.Unlock()

	u.Printf("%s", banner(st))
}

// DisableColor turns color off whatever the terminal said, for the
// -no-color flag. It is for before anything has been drawn; it does not
// repaint what is already on the screen.
func (u *UI) DisableColor() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.st.color = false
}

// banner renders the welcome for a given style. It is a function of style
// alone so it can be tested for every kind of terminal without one.
func banner(st style) string {
	if !st.unicode || st.width < len(bannerIndent)+bannerCols {
		return bannerIndent + paint(st, colorCream, bannerPlain) +
			"   " + paint(st, colorMuted, bannerTagline)
	}

	var b strings.Builder
	for _, row := range strings.Split(bannerArt, "\n") {
		// Split by rune, not byte: every block character is three bytes.
		r := []rune(row)
		prompt, bubble := r, []rune(nil)
		if len(r) > bannerSplit {
			prompt, bubble = r[:bannerSplit], r[bannerSplit:]
		}

		b.WriteString(bannerIndent)
		b.WriteString(paint(st, colorCream, string(prompt)))
		b.WriteString(paint(st, colorGreen, string(bubble)))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(bannerIndent)
	b.WriteString(paint(st, colorGreen, bannerWordmark))
	b.WriteString("   ")
	b.WriteString(paint(st, colorMuted, bannerTagline))

	return b.String()
}
