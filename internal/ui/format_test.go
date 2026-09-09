package ui

import (
	"testing"
	"time"
)

func TestPerSecondAndTimeLeft(t *testing.T) {
	t.Parallel()

	// Under a second is not a measurement.
	if got := perSecond(1<<20, 400*time.Millisecond); got != "" {
		t.Errorf("perSecond too early: %q", got)
	}
	if got := perSecond(2<<20, 2*time.Second); got != "1.0 MB/s" {
		t.Errorf("perSecond = %q", got)
	}
	if got := perSecond(0, 5*time.Second); got != "" {
		t.Errorf("nothing moved, yet: %q", got)
	}

	cases := []struct {
		remaining, done int64
		d               time.Duration
		want            string
	}{
		{0, 100, time.Minute, ""},              // finished
		{100, 0, time.Minute, ""},              // nothing to measure
		{100, 100, 500 * time.Millisecond, ""}, // too early
		{50, 100, 10 * time.Second, "a moment left"},
		{500, 100, 10 * time.Second, "about 50s left"},
		{6000, 100, 10 * time.Second, "about 10m left"},
		{40000, 100, 10 * time.Second, "about 1h6m left"},
	}
	for _, c := range cases {
		if got := leftText(c.remaining, c.done, c.d); got != c.want {
			t.Errorf("leftText(%d, %d, %v) = %q, want %q", c.remaining, c.done, c.d, got, c.want)
		}
	}
}

// A size a tenth under the next unit reads as the next unit, not as four
// digits of the smaller one.
func TestHumanBytesStepsUpAtTheBoundary(t *testing.T) {
	t.Parallel()

	cases := map[int64]string{
		0:          "0 B",
		1023:       "1023 B",
		1024:       "1.0 KB",
		1048550:    "1.0 MB", // 1023.97 KB
		1 << 20:    "1.0 MB",
		1<<30 - 10: "1.0 GB",
		5 << 30:    "5.0 GB",
	}
	for n, want := range cases {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
