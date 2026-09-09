package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Serajian/homa/internal/proto"
)

// These are end-to-end tests over a loopback connection: two real sessions, a
// real handshake, real frames. Nothing leaves the machine, so they belong in
// the ordinary test run rather than behind a tag. See connPair for why this
// is not net.Pipe.

// recorder is a Handler that remembers what it was told, so a test can wait
// for something to arrive rather than sleep and hope.
type recorder struct {
	mu    sync.Mutex
	texts []string
	got   chan string

	// dir, accept and reason answer an offer. They are read from the
	// session's goroutine, so they are set before Run starts.
	dir    string
	accept bool
	reason string

	done   chan string // a completed file's path
	failed chan error
}

func newRecorder(t *testing.T) *recorder {
	t.Helper()

	return &recorder{
		got:    make(chan string, 16),
		done:   make(chan string, 4),
		failed: make(chan error, 4),
		dir:    t.TempDir(),
		accept: true,
	}
}

func (r *recorder) OnText(text string) {
	r.mu.Lock()
	r.texts = append(r.texts, text)
	r.mu.Unlock()
	r.got <- text
}

func (r *recorder) OnFileOffer(string, int64) (dir string, accept bool, reason string) {
	return r.dir, r.accept, r.reason
}
func (r *recorder) OnFileProgress(string, int64, int64) {}
func (r *recorder) OnFileDone(_, path string)           { r.done <- path }
func (r *recorder) OnFileError(_ string, err error)     { r.failed <- err }

// textOnly implements Handler but not FileHandler, which is how a session
// behaves before a UI is wired up: every offer is refused.
type textOnly struct{ got chan string }

func (t textOnly) OnText(s string) { t.got <- s }

// talk brings up two sessions on a pipe and runs both read loops.
func talk(t *testing.T, a, b Handler) (as, bs *Session) {
	t.Helper()

	ac, bc := connPair(t)

	type result struct {
		s   *Session
		err error
	}
	ch := make(chan result, 1)
	go func() {
		s, err := Start(bc, "bob", b)
		ch <- result{s, err}
	}()

	as, err := Start(ac, "alice", a)
	if err != nil {
		t.Fatalf("starting alice: %v", err)
	}
	res := <-ch
	if res.err != nil {
		t.Fatalf("starting bob: %v", res.err)
	}
	bs = res.s

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = as.Run(ctx) }()
	go func() { defer wg.Done(); _ = bs.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return as, bs
}

func waitFor(t *testing.T, ch <-chan string, what string) string {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return ""
	}
}

func TestHandshakeTellsEachSideAboutTheOther(t *testing.T) {
	t.Parallel()

	as, bs := talk(t, newRecorder(t), newRecorder(t))

	if got := as.Peer().Nick; got != "bob" {
		t.Errorf("alice sees %q, want bob", got)
	}
	if got := bs.Peer().Nick; got != "alice" {
		t.Errorf("bob sees %q, want alice", got)
	}
	if got := as.Peer().Version; got != proto.Version {
		t.Errorf("version = %d, want %d", got, proto.Version)
	}
}

func TestAMessageEachWay(t *testing.T) {
	t.Parallel()

	ra, rb := newRecorder(t), newRecorder(t)
	as, bs := talk(t, ra, rb)

	if err := as.SendText("salam"); err != nil {
		t.Fatalf("alice sending: %v", err)
	}
	if got := waitFor(t, rb.got, "bob to hear alice"); got != "salam" {
		t.Errorf("bob heard %q", got)
	}

	if err := bs.SendText("salam, chetori"); err != nil {
		t.Fatalf("bob sending: %v", err)
	}
	if got := waitFor(t, ra.got, "alice to hear bob"); got != "salam, chetori" {
		t.Errorf("alice heard %q", got)
	}
}

// A message from a peer reaches the handler already sanitized. The rest of
// homa prints it without checking again, so this is where that guarantee is.
func TestAMessageArrivesSanitized(t *testing.T) {
	t.Parallel()

	ra, _ := newRecorder(t), 0
	_, bs := talk(t, ra, newRecorder(t))

	// Sent under the session's nose, so the sanitizing being tested is the
	// receiver's and not SendText's.
	if err := bs.c.WriteText("before\x1b[2Jafter\rforged"); err != nil {
		t.Fatalf("writing: %v", err)
	}

	got := waitFor(t, ra.got, "the message")
	if strings.ContainsRune(got, 0x1b) || strings.ContainsRune(got, '\r') {
		t.Errorf("handler was given %q, which a terminal would obey", got)
	}
}

func TestAnEmptyMessageIsNotSent(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	_, bs := talk(t, ra, newRecorder(t))

	if err := bs.SendText("   \n"); err != nil {
		t.Fatalf("sending: %v", err)
	}
	if err := bs.SendText("real"); err != nil {
		t.Fatalf("sending: %v", err)
	}

	if got := waitFor(t, ra.got, "a message"); got != "real" {
		t.Errorf("first message through was %q, want the empty one dropped", got)
	}
}

func TestAMessageOverTheLimitIsRefused(t *testing.T) {
	t.Parallel()

	_, bs := talk(t, newRecorder(t), newRecorder(t))

	if err := bs.SendText(strings.Repeat("a", MaxTextLen+1)); err == nil {
		t.Error("sent a message longer than the limit")
	}
}

func TestAFileOfferAcceptedArrivesWhole(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	_, bs := talk(t, ra, newRecorder(t))

	content := []byte(strings.Repeat("homa", 20000)) // several chunks
	src := writeFile(t, "poster.png", content)

	if err := bs.SendFile(t.Context(), src, nil); err != nil {
		t.Fatalf("sending: %v", err)
	}

	path := waitFor(t, ra.done, "the file to land")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading what arrived: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("the file that arrived is not the file that was sent")
	}
	if filepath.Base(path) != "poster.png" {
		t.Errorf("landed as %q", filepath.Base(path))
	}
}

func TestAFileOfferRejectedTellsTheSenderWhy(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	ra.accept = false
	ra.reason = "not now"

	_, bs := talk(t, ra, newRecorder(t))
	src := writeFile(t, "poster.png", []byte("data"))

	err := bs.SendFile(t.Context(), src, nil)
	if err == nil {
		t.Fatal("sending a refused file reported success")
	}
	if !strings.Contains(err.Error(), "not now") {
		t.Errorf("err = %v, want the reason in it", err)
	}

	if left := entries(t, ra.dir); len(left) != 0 {
		t.Errorf("a refused transfer left %v behind", left)
	}
}

// A handler with no FileHandler refuses everything, which is how a session
// behaves before a UI is wired up. It must refuse rather than panic.
func TestAHandlerThatCannotTakeFilesRefusesThem(t *testing.T) {
	t.Parallel()

	_, bs := talk(t, textOnly{got: make(chan string, 4)}, newRecorder(t))
	src := writeFile(t, "poster.png", []byte("data"))

	if err := bs.SendFile(t.Context(), src, nil); err == nil {
		t.Error("a session with no file handler accepted a file")
	}
}

// TCP already catches damage in transit, so a digest that does not match
// means the two sides disagree about the content. The file is discarded
// rather than kept: a wrong file that looks finished is worse than none.
func TestAChecksumMismatchDiscardsTheFile(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	_, bs := talk(t, ra, newRecorder(t))

	// Drive the protocol by hand so the digest can be wrong while the
	// bytes are right.
	if err := bs.c.WriteJSON(proto.TypeFileOffer, proto.FileOffer{ID: 1, Name: "x.bin", Size: 4}); err != nil {
		t.Fatal(err)
	}
	if err := bs.c.WriteChunk(1, []byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := bs.c.WriteJSON(proto.TypeFileDone, proto.FileDone{ID: 1, SHA256: strings.Repeat("0", 64)}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-ra.failed:
		if err == nil {
			t.Fatal("a mismatch was reported as success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a mismatched digest was never reported")
	}

	if left := entries(t, ra.dir); len(left) != 0 {
		t.Errorf("a discarded transfer left %v behind", left)
	}
}

// A file is written to a .part and renamed only once the digest matches, so
// an interrupted transfer never leaves something that looks finished.
func TestAnInterruptedTransferLeavesNoPartBehind(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	as, bs := talk(t, ra, newRecorder(t))

	if err := bs.c.WriteJSON(proto.TypeFileOffer, proto.FileOffer{ID: 1, Name: "x.bin", Size: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := bs.c.WriteChunk(1, []byte("the start of it")); err != nil {
		t.Fatal(err)
	}

	// Wait for the receiving side to have opened the file before the
	// connection goes, or the test proves nothing.
	waitForFile(t, ra.dir)

	_ = bs.Close()
	_ = as.Close()

	select {
	case <-ra.failed:
	case <-time.After(5 * time.Second):
		t.Fatal("the abandoned transfer was never reported")
	}

	for _, e := range entries(t, ra.dir) {
		if strings.HasSuffix(e, ".part") {
			t.Errorf("a partial file was left behind: %s", e)
		}
	}
}

// A name from a peer becomes a path on this machine. This is the same guard
// safeFileName carries, seen from the outside: it must hold through a whole
// transfer and not only in a unit test.
func TestAFileNameCannotEscapeTheDownloadDirectory(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	_, bs := talk(t, ra, newRecorder(t))

	content := []byte("gotcha")
	sum := sha256.Sum256(content)

	if err := bs.c.WriteJSON(proto.TypeFileOffer, proto.FileOffer{
		ID: 1, Name: "../../escaped.txt", Size: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if err := bs.c.WriteChunk(1, content); err != nil {
		t.Fatal(err)
	}
	if err := bs.c.WriteJSON(proto.TypeFileDone, proto.FileDone{
		ID: 1, SHA256: hex.EncodeToString(sum[:]),
	}); err != nil {
		t.Fatal(err)
	}

	path := waitFor(t, ra.done, "the file to land")
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(ra.dir)+string(filepath.Separator)) {
		t.Fatalf("a file landed at %q, outside %q", path, ra.dir)
	}
}

// An unknown frame type is skipped rather than treated as a failure. It is
// what lets a peer on a newer version send something this one has never
// heard of without ending the conversation.
func TestAnUnknownFrameTypeIsIgnored(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	_, bs := talk(t, ra, newRecorder(t))

	if err := bs.c.Write(proto.Type(0xfe), []byte("from the future")); err != nil {
		t.Fatal(err)
	}
	if err := bs.SendText("still here"); err != nil {
		t.Fatalf("sending after an unknown frame: %v", err)
	}

	if got := waitFor(t, ra.got, "the message after it"); got != "still here" {
		t.Errorf("got %q", got)
	}
}

func TestRunReturnsCleanlyOnAGoodbye(t *testing.T) {
	t.Parallel()

	ac, bc := connPair(t)

	ch := make(chan *Session, 1)
	go func() {
		s, err := Start(bc, "bob", newRecorder(t))
		if err != nil {
			t.Errorf("starting bob: %v", err)
		}
		ch <- s
	}()

	as, err := Start(ac, "alice", newRecorder(t))
	if err != nil {
		t.Fatalf("starting alice: %v", err)
	}
	bs := <-ch

	done := make(chan error, 1)
	go func() { done <- as.Run(t.Context()) }()

	_ = bs.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("a goodbye ended the run with %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after a goodbye")
	}
}

func TestStartRefusesNoHandler(t *testing.T) {
	t.Parallel()

	ac, _ := connPair(t)
	if _, err := Start(ac, "alice", nil); err == nil {
		t.Error("started a session with no handler")
	}
}

// A version a peer does not share is logged, not fatal: the framing is
// stable, so an older or newer peer can still hold a conversation. Cutting
// them off would fragment every future release.
func TestAVersionMismatchIsNotFatal(t *testing.T) {
	t.Parallel()

	ac, bc := connPair(t)

	// Speak the handshake by hand, claiming to be from another version.
	// The errors come back on a channel rather than through t, because a
	// goroutine reporting into a test that has finished is a panic.
	peer := make(chan error, 1)
	go func() {
		c := proto.NewConn(bc)
		if err := c.WriteJSON(proto.TypeHello, proto.Hello{Nick: "ancient", Version: 1}); err != nil {
			peer <- err
			return
		}
		_, err := c.Read()
		peer <- err
	}()

	s, err := Start(ac, "alice", newRecorder(t))
	if err != nil {
		t.Fatalf("a version mismatch ended the handshake: %v", err)
	}
	if got := s.Peer().Version; got != 1 {
		t.Errorf("version = %d, want the one the peer announced", got)
	}
	if s.SignalsAcceptance() {
		t.Error("a version 1 peer was expected to signal acceptance")
	}

	if err := <-peer; err != nil {
		t.Errorf("the peer side of the handshake: %v", err)
	}
}

func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func entries(t *testing.T, dir string) []string {
	t.Helper()

	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	out := make([]string, 0, len(des))
	for _, de := range des {
		out = append(out, de.Name())
	}
	return out
}

func waitForFile(t *testing.T, dir string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(entries(t, dir)) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("nothing was ever written to the download directory")
}

// halfSession is a real session on one end of a loopback pair and the raw
// connection on the other, so a test can send frames a real session would
// never send and read what comes back.
func halfSession(t *testing.T, h Handler) (*Session, *proto.Conn) {
	t.Helper()

	ac, bc := connPair(t)
	raw := proto.NewConn(bc)

	// Both sides greet before either reads, so the far half is one write.
	ch := make(chan error, 1)
	go func() {
		ch <- raw.WriteJSON(proto.TypeHello, proto.Hello{Nick: "bob", Version: proto.Version})
	}()
	as, err := Start(ac, "alice", h)
	if err != nil {
		t.Fatalf("starting alice: %v", err)
	}
	if err := <-ch; err != nil {
		t.Fatalf("greeting from the raw side: %v", err)
	}
	if f, err := raw.Read(); err != nil || f.Type != proto.TypeHello {
		t.Fatalf("alice's greeting: %v %v", f.Type, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); _ = as.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	return as, raw
}

// An offer whose body will not decode still carries its id, and the sender
// is told no at once rather than waiting out the offer timeout.
func TestAnUnreadableOfferIsRefusedRatherThanDropped(t *testing.T) {
	t.Parallel()

	_, raw := halfSession(t, newRecorder(t))

	// Valid JSON, but a size no FileOffer can hold. The id survives.
	if err := raw.Write(proto.TypeFileOffer, []byte(`{"id":7,"name":"x","size":"enormous"}`)); err != nil {
		t.Fatal(err)
	}

	f, err := raw.Read()
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if f.Type != proto.TypeFileReject {
		t.Fatalf("answered with %s", f.Type)
	}
	var msg proto.FileReject
	if err := proto.DecodeJSON(f, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != 7 || !strings.Contains(msg.Reason, "could not be read") {
		t.Errorf("refusal was %+v", msg)
	}
}

// A body that is not JSON at all gives up no id, so there is nothing to
// answer; it must still leave the session running.
func TestAnOfferThatIsNotJSONIsDroppedWithoutBreakingTheSession(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t)
	as, raw := halfSession(t, ra)

	if err := raw.Write(proto.TypeFileOffer, []byte("not json at all")); err != nil {
		t.Fatal(err)
	}
	if err := as.SendText("still here"); err != nil {
		t.Fatalf("sending after a broken offer: %v", err)
	}
	f, err := raw.Read()
	if err != nil {
		t.Fatalf("reading after a broken offer: %v", err)
	}
	if f.Type != proto.TypeText || string(f.Payload) != "still here" {
		t.Errorf("got %s %q", f.Type, f.Payload)
	}
}

// A refusal whose reason will not decode still carries its id, so the wait
// ends at once instead of running out five minutes later.
func TestAnUnreadableRefusalEndsTheWait(t *testing.T) {
	t.Parallel()

	as, raw := halfSession(t, newRecorder(t))
	src := writeFile(t, "poster.png", []byte("data"))

	errc := make(chan error, 1)
	go func() { errc <- as.SendFile(t.Context(), src, nil) }()

	f, err := raw.Read()
	if err != nil || f.Type != proto.TypeFileOffer {
		t.Fatalf("expected an offer, got %v %v", f.Type, err)
	}
	var offer proto.FileOffer
	if err := proto.DecodeJSON(f, &offer); err != nil {
		t.Fatal(err)
	}

	// A refusal with the right id and a reason that is not a string.
	body := fmt.Sprintf(`{"id":%d,"reason":5}`, offer.ID)
	if err := raw.Write(proto.TypeFileReject, []byte(body)); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "could not be read") {
			t.Errorf("SendFile returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sender was left waiting")
	}
}

// addressRecorder is a Handler that also takes an address.
type addressRecorder struct {
	*recorder
	addrs chan string
}

func (a addressRecorder) OnAddress(addr string) { a.addrs <- addr }

func TestAnAddressCrossesAndArrivesSanitized(t *testing.T) {
	t.Parallel()

	ra := addressRecorder{recorder: newRecorder(t), addrs: make(chan string, 2)}
	as, bs := talk(t, ra, newRecorder(t))

	if !as.CanTakeAddress() || !bs.CanTakeAddress() {
		t.Fatal("two current peers cannot hand over an address")
	}
	// The newline and the space are what a copy off the address page
	// brings along; nothing but the address survives them.
	if err := bs.SendAddress("tcpABC\n  DEF"); err != nil {
		t.Fatalf("sending an address: %v", err)
	}
	if got := waitFor(t, ra.addrs, "the address"); got != "tcpABCDEF" {
		t.Errorf("arrived as %q", got)
	}

	if err := bs.SendAddress("   "); err == nil {
		t.Error("an empty address was sent")
	}
	if err := bs.SendAddress(strings.Repeat("a", MaxAddrLen+1)); err == nil {
		t.Error("an address longer than the limit was sent")
	}
}

// A handler with nowhere to put an address ignores it and keeps talking,
// the way one that cannot take files refuses them.
func TestAnAddressToAHandlerThatCannotTakeOneIsIgnored(t *testing.T) {
	t.Parallel()

	ra := newRecorder(t) // no OnAddress
	_, bs := talk(t, ra, newRecorder(t))

	if err := bs.SendAddress("tcpABC"); err != nil {
		t.Fatal(err)
	}
	if err := bs.SendText("still here"); err != nil {
		t.Fatal(err)
	}
	if got := waitFor(t, ra.got, "the message after it"); got != "still here" {
		t.Errorf("got %q", got)
	}
}
