package logx

import (
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// Logging is off until main says otherwise, and that is not a detail: homa
// draws a terminal interface, and a log line landing mid-conversation would
// scramble it. A package that logged before being switched on would be found
// by somebody's ruined screen rather than by a test.
func TestNothingIsWrittenUntilItIsSwitchedOn(t *testing.T) {
	t.Cleanup(Silence)
	Silence()

	var out strings.Builder
	For("peer").Info("this should go nowhere")

	if out.Len() != 0 {
		t.Errorf("wrote %q before being switched on", out.String())
	}
}

func TestSetSendsEverythingSomewhere(t *testing.T) {
	t.Cleanup(Silence)

	var out strings.Builder
	ToWriter(&out, slog.LevelDebug)

	For("session").Info("hello", "count", 3)

	got := out.String()
	for _, want := range []string{"hello", "count=3", "pkg=session"} {
		if !strings.Contains(got, want) {
			t.Errorf("output %q is missing %q", got, want)
		}
	}
}

// Every package asks for its logger once, at package level, long before main
// decides where output goes. A logger handed out early has to follow the
// switch, or -debug would only affect packages loaded afterwards.
func TestALoggerHandedOutEarlyFollowsTheSwitch(t *testing.T) {
	t.Cleanup(Silence)
	Silence()

	early := For("peer")
	early.Info("into the void")

	var out strings.Builder
	ToWriter(&out, slog.LevelDebug)
	early.Info("after the switch")

	got := out.String()
	if strings.Contains(got, "into the void") {
		t.Error("a line written while silent turned up later")
	}
	if !strings.Contains(got, "after the switch") {
		t.Errorf("the early logger did not follow the switch: %q", got)
	}
}

func TestTheLevelIsRespected(t *testing.T) {
	t.Cleanup(Silence)

	var out strings.Builder
	ToWriter(&out, slog.LevelInfo)

	lg := For("proto")
	lg.Debug("too quiet to print")
	lg.Warn("loud enough")

	got := out.String()
	if strings.Contains(got, "too quiet") {
		t.Error("a debug line was printed at info level")
	}
	if !strings.Contains(got, "loud enough") {
		t.Errorf("a warning was swallowed: %q", got)
	}
}

func TestSetWithNothingIsTheSameAsSilence(t *testing.T) {
	t.Cleanup(Silence)

	var out strings.Builder
	ToWriter(&out, slog.LevelDebug)
	Set(nil)

	For("ui").Error("after being handed nothing")

	if out.Len() != 0 {
		t.Errorf("wrote %q after Set(nil)", out.String())
	}
}

// A session's read goroutine logs while the interface logs, and -debug can be
// switched on from another. This is the test -race exists for.
func TestLoggingFromSeveralGoroutines(t *testing.T) {
	t.Cleanup(Silence)

	var mu sync.Mutex
	ToWriter(&lockedWriter{mu: &mu}, slog.LevelDebug)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lg := For("pkg")
			for range 50 {
				lg.Info("a line", "from", i)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			Silence()
			ToWriter(&lockedWriter{mu: &mu}, slog.LevelDebug)
		}
	}()

	wg.Wait()
}

type lockedWriter struct{ mu *sync.Mutex }

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(p), nil
}
