//go:build live

package live

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// screen is what a terminal would be showing: a grid of cells that the
// bytes homa writes are applied to, cursor movement and erasing included,
// styling ignored. The full-screen interface redraws in place, so what a
// person sees is this grid and not the stream of bytes; a test that waits
// for text has to look here.
//
// It understands what bubbletea's renderer emits: \r, \n (a bare line
// feed: the terminal is raw, so the column does not move), \b, \t, and
// ESC M (up a line, which is how the renderer climbs), ESC D and E (down),
// and the CSI sequences for cursor position (H, f, A, B, C, D, G, d), tabs (I
// forward, Z backward, at every eighth column, which is what ?5W resets
// them to), erasing (J, K, X, P) and everything styling-related (m), plus
// the private-mode and query sequences it ignores. An OSC string is
// skipped to its terminator.
type screen struct {
	mu   sync.Mutex
	rows [][]rune
	r, c int
	cols int

	// pending is an escape sequence a read boundary cut in two: kept until
	// the rest arrives, or the color code would be drawn as text.
	pending []rune

	// finals counts the control sequences seen, by final byte, so a
	// failure can say which ones the interpreter met.
	finals map[rune]int

	// bells counts the terminal bells rung: nothing to draw, but a thing
	// homa promises to do when something arrives.
	bells int
}

func newScreen(rows, cols int) *screen {
	s := &screen{cols: cols, finals: make(map[rune]int)}
	s.rows = make([][]rune, rows)
	for i := range s.rows {
		s.rows[i] = blank(cols)
	}
	return s
}

func blank(n int) []rune {
	row := make([]rune, n)
	for i := range row {
		row[i] = ' '
	}
	return row
}

// rung is how many bells have been rung so far.
func (s *screen) rung() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bells
}

// seen is the control sequences met so far, by final byte.
func (s *screen) seen() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprint(s.finals)
}

// text is the grid as lines, trailing spaces cut.
func (s *screen) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines := make([]string, len(s.rows))
	for i, row := range s.rows {
		lines[i] = strings.TrimRight(string(row), " ")
	}
	return strings.Join(lines, "\n")
}

// write applies bytes to the grid.
func (s *screen) write(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	in := append(s.pending, []rune(string(b))...)
	s.pending = nil
	for i := 0; i < len(in); i++ {
		switch ch := in[i]; {
		case ch == '\x1b':
			n, complete := s.escape(in[i:])
			if !complete {
				s.pending = append([]rune(nil), in[i:]...)
				return
			}
			i += n - 1
		case ch == '\r':
			s.c = 0
		case ch == '\n':
			s.newline()
		case ch == '\b':
			if s.c > 0 {
				s.c--
			}
		case ch == '\t':
			s.tab(1)
		case ch == '\a':
			s.bells++
		case ch < ' ':
			// other control bytes: nothing to draw
		default:
			s.put(ch)
		}
	}
}

func (s *screen) put(ch rune) {
	if s.c >= s.cols {
		s.c = 0
		s.newline()
	}
	s.rows[s.r][s.c] = ch
	s.c++
}

// tab moves n tab stops forward (n > 0) or back (n < 0); stops are every
// eight columns.
func (s *screen) tab(n int) {
	for ; n > 0; n-- {
		s.c = min((s.c/8+1)*8, s.cols-1)
	}
	for ; n < 0; n++ {
		if s.c == 0 {
			return
		}
		s.c = ((s.c - 1) / 8) * 8
	}
}

func (s *screen) newline() {
	if s.r < len(s.rows)-1 {
		s.r++
		return
	}
	// the bottom row: the grid scrolls, as a terminal does
	copy(s.rows, s.rows[1:])
	s.rows[len(s.rows)-1] = blank(s.cols)
}

// escape handles one escape sequence starting at in[0] and returns how
// many runes it used, and false when the sequence is not all there yet.
func (s *screen) escape(in []rune) (int, bool) {
	if len(in) < 2 {
		return len(in), false
	}
	switch in[1] {
	case ']': // OSC: to BEL or ST
		for i := 2; i < len(in); i++ {
			if in[i] == '\a' {
				return i + 1, true
			}
			if in[i] == '\x1b' && i+1 < len(in) && in[i+1] == '\\' {
				return i + 2, true
			}
		}
		return len(in), false
	case '[':
		// CSI: parameters, then one final byte in @..~
		i := 2
		for i < len(in) && (in[i] < '@' || in[i] > '~') {
			i++
		}
		if i >= len(in) {
			return len(in), false
		}
		s.finals[in[i]]++
		s.csi(string(in[2:i]), in[i])
		return i + 1, true
	case 'M': // reverse index: up one line — what the renderer moves up with
		if s.r > 0 {
			s.r--
		}
		return 2, true
	case 'D': // index: down one line
		s.newline()
		return 2, true
	case 'E': // next line
		s.c = 0
		s.newline()
		return 2, true
	case '7', '8': // save and restore the cursor: not used for drawing here
		return 2, true
	case '(', ')': // a charset selection: ESC ( B and the like
		if len(in) < 3 {
			return len(in), false
		}
		return 3, true
	default:
		return 2, true // ESC + one byte: anything else
	}
}

// csi applies one control sequence. Private ones (a leading ?, >, = or a
// trailing $) are ignored.
func (s *screen) csi(params string, final rune) {
	if params != "" &&
		(params[0] == '?' || params[0] == '>' || params[0] == '=' || strings.HasSuffix(params, "$")) {
		return
	}
	nums := []int{}
	for _, p := range strings.Split(params, ";") {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		nums = append(nums, n)
	}
	arg := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		return def
	}
	clamp := func(v, hi int) int { return max(0, min(v, hi)) }

	switch final {
	case 'H', 'f':
		s.r, s.c = clamp(arg(0, 1)-1, len(s.rows)-1), clamp(arg(1, 1)-1, s.cols-1)
	case 'A':
		s.r = clamp(s.r-arg(0, 1), len(s.rows)-1)
	case 'B':
		s.r = clamp(s.r+arg(0, 1), len(s.rows)-1)
	case 'C':
		s.c = clamp(s.c+arg(0, 1), s.cols-1)
	case 'D':
		s.c = clamp(s.c-arg(0, 1), s.cols-1)
	case 'G':
		s.c = clamp(arg(0, 1)-1, s.cols-1)
	case 'I':
		s.tab(arg(0, 1))
	case 'Z':
		s.tab(-arg(0, 1))
	case 'd':
		s.r = clamp(arg(0, 1)-1, len(s.rows)-1)
	case 'J':
		switch arg(0, 0) {
		case 0:
			s.rows[s.r] = append(s.rows[s.r][:s.c:s.c], blank(s.cols-s.c)...)
			for i := s.r + 1; i < len(s.rows); i++ {
				s.rows[i] = blank(s.cols)
			}
		case 1:
			for i := 0; i < s.r; i++ {
				s.rows[i] = blank(s.cols)
			}
			copy(s.rows[s.r][:s.c+1], blank(s.c+1))
		default:
			for i := range s.rows {
				s.rows[i] = blank(s.cols)
			}
		}
	case 'K':
		switch arg(0, 0) {
		case 0:
			copy(s.rows[s.r][s.c:], blank(s.cols-s.c))
		case 1:
			copy(s.rows[s.r][:s.c+1], blank(s.c+1))
		default:
			s.rows[s.r] = blank(s.cols)
		}
	case 'L': // insert n blank lines at the cursor, pushing the rest down
		n := min(arg(0, 1), len(s.rows)-s.r)
		copy(s.rows[s.r+n:], s.rows[s.r:len(s.rows)-n])
		for i := s.r; i < s.r+n; i++ {
			s.rows[i] = blank(s.cols)
		}
	case 'M': // delete n lines at the cursor, pulling the rest up
		n := min(arg(0, 1), len(s.rows)-s.r)
		copy(s.rows[s.r:], s.rows[s.r+n:])
		for i := len(s.rows) - n; i < len(s.rows); i++ {
			s.rows[i] = blank(s.cols)
		}
	case 'S': // scroll up n lines
		n := min(arg(0, 1), len(s.rows))
		copy(s.rows, s.rows[n:])
		for i := len(s.rows) - n; i < len(s.rows); i++ {
			s.rows[i] = blank(s.cols)
		}
	case 'T': // scroll down n lines
		n := min(arg(0, 1), len(s.rows))
		copy(s.rows[n:], s.rows[:len(s.rows)-n])
		for i := range n {
			s.rows[i] = blank(s.cols)
		}
	case 'X': // erase n cells
		n := min(arg(0, 1), s.cols-s.c)
		copy(s.rows[s.r][s.c:s.c+n], blank(n))
	case 'P': // delete n cells, shifting the rest left
		n := min(arg(0, 1), s.cols-s.c)
		row := s.rows[s.r]
		copy(row[s.c:], row[s.c+n:])
		copy(row[s.cols-n:], blank(n))
	}
}
