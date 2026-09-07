package ui

import (
	"sync"
	"time"
)

// countdown redraws a line once a second with the time left on it, and stops
// when the returned function is called.
//
// The line is a prompt rather than a printed line, so it replaces itself
// instead of filling the screen, and anything printed from elsewhere steps
// around it the way it steps around any other prompt.
//
// The caller decides what the line says, because the two places that need one
// want different shapes: a caller waiting to be let in, and a question with a
// deadline on it.
//
// stop is safe to call more than once, and nothing is drawn after it returns.
// That matters: a tick landing after the person has answered would put a stale
// question back on a screen that has moved on.
func (u *UI) countdown(deadline time.Time, line func(left time.Duration) string) (stop func()) {
	var (
		mu   sync.Mutex
		done bool
	)

	draw := func() {
		mu.Lock()
		defer mu.Unlock()

		if done {
			return
		}
		u.Prompt("%s", line(remaining(deadline)))
	}

	draw()

	t := time.NewTicker(countdownStep)
	go func() {
		defer t.Stop()
		for range t.C {
			mu.Lock()
			finished := done
			mu.Unlock()

			if finished {
				return
			}
			draw()
		}
	}()

	return func() {
		mu.Lock()
		defer mu.Unlock()
		done = true
	}
}

// remaining is how long is left, rounded to a whole second and never negative.
// A countdown showing "-1s" is a countdown nobody trusts again.
func remaining(deadline time.Time) time.Duration {
	left := time.Until(deadline).Round(time.Second)
	if left < 0 {
		return 0
	}
	return left
}
