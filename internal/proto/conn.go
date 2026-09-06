package proto

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Conn reads and writes frames on a stream.
//
// Writes are serialized by a mutex: several goroutines (the chat loop and a
// file transfer, say) may write concurrently and their frames will not
// interleave. Reads are not: exactly one goroutine should call Read.
type Conn struct {
	r *bufio.Reader
	w io.Writer

	mu sync.Mutex // guards writes to w
}

// NewConn wraps a stream so frames can be read from and written to it.
func NewConn(rw io.ReadWriter) *Conn {
	return &Conn{
		r: bufio.NewReaderSize(rw, readBufSize),
		w: rw,
	}
}

// Write emits one frame. Concurrent calls are safe.
func (c *Conn) Write(t Type, payload []byte) error {
	if len(payload) > MaxPayload {
		return fmt.Errorf("proto: %s payload of %d bytes exceeds max %d", t, len(payload), MaxPayload)
	}

	var hdr [headerSize]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(len(payload)))
	hdr[4] = byte(t)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.w.Write(hdr[:]); err != nil {
		return fmt.Errorf("proto: writing %s header: %w", t, err)
	}
	if len(payload) > 0 {
		if _, err := c.w.Write(payload); err != nil {
			return fmt.Errorf("proto: writing %s payload: %w", t, err)
		}
	}
	return nil
}

// WriteJSON marshals v and sends it as one frame of type t.
func (c *Conn) WriteJSON(t Type, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("proto: encoding %s: %w", t, err)
	}
	return c.Write(t, b)
}

// WriteText sends a chat message.
func (c *Conn) WriteText(s string) error {
	return c.Write(TypeText, []byte(s))
}

// WriteChunk sends one piece of the transfer with the given id. The payload is
// the 4-byte id followed by raw bytes, so chunks stay cheap: no base64 and no
// JSON around binary data.
func (c *Conn) WriteChunk(id uint32, data []byte) error {
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[:4], id)
	copy(buf[4:], data)
	return c.Write(TypeFileChunk, buf)
}

// WriteBye announces a clean shutdown, so the peer can tell a deliberate
// goodbye from a dropped connection.
func (c *Conn) WriteBye() error {
	return c.Write(TypeBye, nil)
}

// Read blocks until a whole frame arrives. It returns io.EOF when the peer
// closed the stream, and io.ErrUnexpectedEOF if it vanished mid-frame.
func (c *Conn) Read() (Frame, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(c.r, hdr[:]); err != nil {
		return Frame{}, err
	}

	n := binary.BigEndian.Uint32(hdr[:4])
	if n > MaxPayload {
		return Frame{}, fmt.Errorf("proto: peer announced %d bytes, over the %d limit", n, MaxPayload)
	}

	f := Frame{Type: Type(hdr[4])}
	if n > 0 {
		f.Payload = make([]byte, n)
		if _, err := io.ReadFull(c.r, f.Payload); err != nil {
			return Frame{}, fmt.Errorf("proto: reading %s payload of %d bytes: %w", f.Type, n, err)
		}
	}
	return f, nil
}

// io.Reader = «از این می‌توانم byte بخوانم»
// io.Writer = «در این می‌توانم byte بنویسم»
// io.ReadWriter = «هم می‌توانم بخوانم، هم بنویسم»
// bufio.Reader = «یک Reader دارم، ولی جلویش یک buffer گذاشته‌ام تا خواندن راحت‌تر/کارآمدتر شود»
// io.ReadFull=«تا وقتی تعداد byte موردنظر کامل نشده، برنگرد»
