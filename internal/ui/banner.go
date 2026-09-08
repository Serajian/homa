package ui

import "strings"

// bannerBlock is the logo in half-block cells, the prompt painted as yours
// and the bubble as theirs, followed by the wordmark. It needs a UTF-8
// terminal and bannerCols of room; otherwise it is the one plain line.
func bannerBlock(st *styles, width int) string {
	if !st.unicode || width < len(bannerIndent)+bannerCols {
		return bannerIndent + st.you.Render(bannerPlain) + "   " + st.dim.Render(bannerTagline) + "\n"
	}

	var b strings.Builder
	for _, row := range strings.Split(bannerArt, "\n") {
		r := []rune(row)
		prompt, bubble := r, []rune(nil)
		if len(r) > bannerSplit {
			prompt, bubble = r[:bannerSplit], r[bannerSplit:]
		}
		b.WriteString(bannerIndent + st.you.Render(string(prompt)) + st.them.Render(string(bubble)) + "\n")
	}
	b.WriteString("\n" + bannerIndent + st.them.Render(bannerWordmark) + "   " + st.dim.Render(bannerTagline) + "\n")
	return b.String()
}
