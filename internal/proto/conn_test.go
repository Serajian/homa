package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
)

// A pipe is the whole fixture this package needs. It knows nothing about
// networks: anything that reads and writes bytes will do, which is why these
// tests need no listener, no relay and no timeout.
func pipe(t *testing.T) (client, server *Conn, done func()) {
	t.Helper()

	c, s := net.Pipe()
	t.Cleanup(func() {
		_ = c.Close()
		_ = s.Close()
	})

	return NewConn(c), NewConn(s), func() { _ = c.Close() }
}

// writeAsync sends on one side while the test reads on the other. net.Pipe is
// unbuffered, so a write blocks until somebody reads it: doing both in one
// goroutine would deadlock.
func writeAsync(t *testing.T, write func() error) {
	t.Helper()

	errs := make(chan error, 1)
	go func() { errs <- write() }()

	t.Cleanup(func() {
		if err := <-errs; err != nil {
			t.Errorf("writing: %v", err)
		}
	})
}

func TestRoundTripEveryFrameType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		send    func(*Conn) error
		want    Type
		payload []byte
	}{
		{
			name:    "hello",
			send:    func(c *Conn) error { return c.WriteJSON(TypeHello, Hello{Nick: "bob", Version: Version}) },
			want:    TypeHello,
			payload: []byte(`{"nick":"bob","version":3}`),
		},
		{
			name:    "text",
			send:    func(c *Conn) error { return c.WriteText("salam") },
			want:    TypeText,
			payload: []byte("salam"),
		},
		{
			name:    "file offer",
			send:    func(c *Conn) error { return c.WriteJSON(TypeFileOffer, FileOffer{ID: 7, Name: "a.txt", Size: 3}) },
			want:    TypeFileOffer,
			payload: []byte(`{"id":7,"name":"a.txt","size":3}`),
		},
		{
			name:    "file accept",
			send:    func(c *Conn) error { return c.WriteJSON(TypeFileAccept, FileAccept{ID: 7}) },
			want:    TypeFileAccept,
			payload: []byte(`{"id":7}`),
		},
		{
			name:    "file reject",
			send:    func(c *Conn) error { return c.WriteJSON(TypeFileReject, FileReject{ID: 7, Reason: "no"}) },
			want:    TypeFileReject,
			payload: []byte(`{"id":7,"reason":"no"}`),
		},
		{
			name:    "file chunk",
			send:    func(c *Conn) error { return c.WriteChunk(7, []byte("data")) },
			want:    TypeFileChunk,
			payload: append(binary.BigEndian.AppendUint32(nil, 7), []byte("data")...),
		},
		{
			name:    "file done",
			send:    func(c *Conn) error { return c.WriteJSON(TypeFileDone, FileDone{ID: 7, SHA256: "ab"}) },
			want:    TypeFileDone,
			payload: []byte(`{"id":7,"sha256":"ab"}`),
		},
		{
			name: "bye",
			send: func(c *Conn) error { return c.WriteBye() },
			want: TypeBye,
		},
		{
			name: "accept",
			send: func(c *Conn) error { return c.WriteAccept() },
			want: TypeAccept,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w, r, _ := pipe(t)
			writeAsync(t, func() error { return tc.send(w) })

			f, err := r.Read()
			if err != nil {
				t.Fatalf("reading: %v", err)
			}
			if f.Type != tc.want {
				t.Errorf("type = %s, want %s", f.Type, tc.want)
			}
			if !bytes.Equal(f.Payload, tc.payload) {
				t.Errorf("payload = %q, want %q", f.Payload, tc.payload)
			}
		})
	}
}

// A stream has no message boundaries, which is the whole reason this package
// exists. Handing the reader one byte at a time is the worst case of that.
func TestFrameArrivingInPieces(t *testing.T) {
	t.Parallel()

	var whole bytes.Buffer
	if err := NewConn(&whole).WriteText("salam donya"); err != nil {
		t.Fatalf("writing: %v", err)
	}

	c := NewConn(&trickle{rest: whole.Bytes()})

	f, err := c.Read()
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if got := string(f.Payload); got != "salam donya" {
		t.Errorf("payload = %q, want %q", got, "salam donya")
	}
}

// trickle hands over one byte per Read, however many were asked for.
type trickle struct{ rest []byte }

func (t *trickle) Read(p []byte) (int, error) {
	if len(t.rest) == 0 {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = t.rest[0]
	t.rest = t.rest[1:]
	return 1, nil
}

func (t *trickle) Write(p []byte) (int, error) { return len(p), nil }

// A peer announcing more than the cap must be refused before anything is
// allocated: the cap exists so a length on the wire cannot make homa reserve
// that much memory.
func TestReadRefusesAnOversizedLength(t *testing.T) {
	t.Parallel()

	var hdr [headerSize]byte
	binary.BigEndian.PutUint32(hdr[:4], MaxPayload+1)
	hdr[4] = byte(TypeText)

	_, err := NewConn(bytes.NewBuffer(hdr[:])).Read()
	if err == nil {
		t.Fatal("read a frame that announced more than MaxPayload")
	}
}

func TestWriteRefusesAnOversizedPayload(t *testing.T) {
	t.Parallel()

	var sent bytes.Buffer
	err := NewConn(&sent).Write(TypeText, make([]byte, MaxPayload+1))

	if err == nil {
		t.Fatal("wrote a payload over MaxPayload")
	}
	if sent.Len() != 0 {
		t.Errorf("wrote %d bytes of a frame it refused", sent.Len())
	}
}

// A peer that vanishes mid-frame is not the same as one that closed cleanly,
// and the caller has to be able to tell them apart.
func TestTruncatedFrame(t *testing.T) {
	t.Parallel()

	var whole bytes.Buffer
	if err := NewConn(&whole).WriteText("salam"); err != nil {
		t.Fatalf("writing: %v", err)
	}

	b := whole.Bytes()
	_, err := NewConn(bytes.NewBuffer(b[:len(b)-2])).Read()

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want it to wrap io.ErrUnexpectedEOF", err)
	}
}

func TestClosedStreamReadsAsEOF(t *testing.T) {
	t.Parallel()

	_, r, closeWriter := pipe(t)
	closeWriter()

	if _, err := r.Read(); !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("err = %v, want EOF or a closed pipe", err)
	}
}

// The chat loop and a file transfer write at the same time. Their frames must
// not interleave, or every frame after the first collision is garbage.
//
// This is the test the race detector exists for; -race is what makes it worth
// more than the assertion below.
func TestConcurrentWritersDoNotInterleave(t *testing.T) {
	t.Parallel()

	const (
		writers = 8
		each    = 25
	)

	var sink lockedBuffer
	c := NewConn(&sink)

	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				if err := c.WriteChunk(uint32(i), bytes.Repeat([]byte{byte(i)}, 64)); err != nil {
					t.Errorf("writing: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	// Every frame must read back whole, with a payload of one repeated byte
	// matching its id. A torn write shows up as either a bad length or a
	// payload with two different bytes in it.
	r := NewConn(bytes.NewBuffer(sink.Bytes()))
	for range writers * each {
		f, err := r.Read()
		if err != nil {
			t.Fatalf("reading back: %v", err)
		}

		id, data, err := DecodeChunk(f)
		if err != nil {
			t.Fatalf("decoding: %v", err)
		}
		if want := bytes.Repeat([]byte{byte(id)}, 64); !bytes.Equal(data, want) {
			t.Fatalf("frame from writer %d carried %q", id, data)
		}
	}
}

// lockedBuffer is a bytes.Buffer that survives concurrent writers. Conn
// serializes its own writes, but the buffer underneath is not the thing being
// tested and must not be the thing that fails.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Read(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

func TestDecodeChunkRefusesAShortPayload(t *testing.T) {
	t.Parallel()

	if _, _, err := DecodeChunk(Frame{Type: TypeFileChunk, Payload: []byte{1, 2, 3}}); err == nil {
		t.Fatal("decoded a chunk with no room for an id")
	}
}

func TestTypeNamesEveryFrameItCanCarry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		t    Type
		want string
	}{
		{TypeHello, "HELLO"},
		{TypeText, "TEXT"},
		{TypeFileOffer, "FILE_OFFER"},
		{TypeFileAccept, "FILE_ACCEPT"},
		{TypeFileReject, "FILE_REJECT"},
		{TypeFileChunk, "FILE_CHUNK"},
		{TypeFileDone, "FILE_DONE"},
		{TypeBye, "BYE"},
		{TypeAccept, "ACCEPT"},
		{Type(0xff), "UNKNOWN(0xff)"},
	} {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("Type(%#x).String() = %q, want %q", uint8(tc.t), got, tc.want)
		}
	}
}
