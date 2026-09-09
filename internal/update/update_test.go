package update

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func server(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

const latest = `{"tag_name":"v0.2.1","html_url":"https://github.com/Serajian/homa/releases/tag/v0.2.1"}`

func TestANewerReleaseIsNoticed(t *testing.T) {
	t.Parallel()

	s := server(t, http.StatusOK, latest)
	cases := []struct {
		current      string
		newer, known bool
	}{
		{"v0.2.0", true, true},
		{"v0.2.1", false, true},
		{"v0.3.0", false, true},
		{"v0.2.0-3-gfb441be-dirty", true, true}, // a build from source, behind
		{"dev", false, false},
	}
	for _, c := range cases {
		r, err := Checker{URL: s.URL}.Check(t.Context(), c.current)
		if err != nil {
			t.Fatalf("%s: %v", c.current, err)
		}
		if r.Newer != c.newer || r.Known != c.known || r.Latest != "v0.2.1" {
			t.Errorf("%s: %+v", c.current, r)
		}
	}
}

func TestAnUnreachableOrOddAnswerIsAnError(t *testing.T) {
	t.Parallel()

	for _, s := range []*httptest.Server{
		server(t, http.StatusForbidden, `{"message":"rate limited"}`),
		server(t, http.StatusOK, `not json`),
		server(t, http.StatusOK, `{"tag_name":""}`),
		server(t, http.StatusOK, `{"tag_name":"nightly"}`),
	} {
		if _, err := (Checker{URL: s.URL}).Check(t.Context(), "v0.2.0"); err == nil {
			t.Errorf("no error from %s", s.URL)
		}
	}
	closed := server(t, http.StatusOK, latest)
	closed.Close()
	if _, err := (Checker{URL: closed.URL}).Check(t.Context(), "v0.2.0"); err == nil {
		t.Error("no error from a closed server")
	}
}

func TestAdviceFollowsWhereTheBinaryLives(t *testing.T) {
	t.Parallel()

	if got := Advice("/opt/homebrew/Caskroom/homa/0.2.0/homa"); got != "brew update && brew upgrade --cask homa" {
		t.Errorf("brew: %q", got)
	}
	if got := Advice("/Users/x/go/bin/homa"); got != "go install github.com/Serajian/homa/cmd/homa@latest" {
		t.Errorf("go install: %q", got)
	}
	if got := Advice("/home/x/Downloads/homa"); got == "" {
		t.Error("no advice for an archive")
	}
}
