package ui

import (
	"strings"
	"testing"
	"time"
)

func TestTheBarCountsDownAndSaysWhatToPress(t *testing.T) {
	t.Parallel()

	st := newStyles(true)
	now := time.Now()
	b := callBar{incoming: &line{name: "~bob"}, deadline: now.Add(47 * time.Second)}
	got := stripANSI(b.view(st, now))
	for _, want := range []string{"~bob is calling", "47s", "y take it", "n not now"} {
		if !strings.Contains(got, want) {
			t.Errorf("incoming bar %q lacks %q", got, want)
		}
	}

	b = callBar{outgoing: "alice", deadline: now.Add(55 * time.Second)}
	got = stripANSI(b.view(st, now))
	for _, want := range []string{"calling alice", "waiting for them to answer", "55s", "Enter to give up"} {
		if !strings.Contains(got, want) {
			t.Errorf("outgoing bar %q lacks %q", got, want)
		}
	}

	var empty callBar
	if empty.view(st, now) != "" {
		t.Error("an empty bar draws something")
	}
}
