package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Serajian/homa/internal/proto"
)

// FileHandler is the part of a Handler that deals with incoming files. A
// Handler that does not implement it refuses every offer, which is how a
// session works before a UI is wired up.
//
// Its methods are called from the session's read goroutine. OnFileOffer in
// particular blocks that goroutine while the person decides, so nothing
// else from the peer is processed until it returns. That is deliberate: the
// sender is waiting anyway, and it keeps the prompt unambiguous.
type FileHandler interface {
	// OnFileOffer asks whether to accept a file, and where to put it. The
	// name has already been made safe. Returning accept false declines
	// the transfer, with reason shown to the sender.
	OnFileOffer(name string, size int64) (dir string, accept bool, reason string)

	// OnFileProgress reports how much of an incoming file has arrived.
	OnFileProgress(name string, received, total int64)

	// OnFileDone reports a completed and verified file at path.
	OnFileDone(name, path string)

	// OnFileError reports a transfer that failed or was abandoned.
	OnFileError(name string, err error)
}

// fileState tracks transfers in both directions. Its zero value is ready to
// use: the maps are created on first use so Session needs no constructor.
type fileState struct {
	mu      sync.Mutex
	nextID  uint32
	pending map[uint32]chan offerReply // ours, waiting for an answer
	active  map[uint32]*incoming       // theirs, being written
}

// offerReply is the peer's answer to one of our offers.
type offerReply struct {
	accepted bool
	reason   string
}

// incoming is a file being written to disk right now.
type incoming struct {
	name      string
	tmpPath   string
	finalPath string
	file      *os.File
	hash      hash.Hash
	size      int64
	got       int64
}

// close releases the file and, unless keep is true, deletes the partial
// download. A half a file is worse than none: it looks complete in a
// listing and only fails when someone opens it.
func (in *incoming) close(keep bool) {
	if in.file != nil {
		_ = in.file.Close()
		in.file = nil
	}
	if !keep {
		_ = os.Remove(in.tmpPath)
	}
}

// ---------------------------------------------------------------- sending

// SendFile offers a file to the peer and, if they accept, sends it.
//
// progress may be nil; when set it is called from this goroutine, not the
// read loop, so an implementation shared with a Handler must be safe for
// both.
func (s *Session) SendFile(
	ctx context.Context,
	path string,
	progress func(sent, total int64),
) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("session: opening %s: %w", path, err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("session: reading %s: %w", path, err)
	}
	if st.IsDir() {
		return fmt.Errorf("session: %s is a directory; send an archive instead", path)
	}

	name := filepath.Base(path)
	size := st.Size()

	id, reply := s.files.newOffer()
	defer s.files.dropOffer(id)

	offer := proto.FileOffer{ID: id, Name: name, Size: size}
	if err := s.c.WriteJSON(proto.TypeFileOffer, offer); err != nil {
		return fmt.Errorf("session: offering %s: %w", name, err)
	}
	logger.Info("file offered", "name", name, "size", size)

	if err := waitForAnswer(ctx, reply, name); err != nil {
		return err
	}

	return s.sendBody(ctx, f, id, name, size, progress)
}

// waitForAnswer blocks until the peer accepts, declines, the person gives
// up, or the offer times out.
func waitForAnswer(ctx context.Context, reply <-chan offerReply, name string) error {
	timer := time.NewTimer(offerTimeout)
	defer timer.Stop()

	select {
	case r, ok := <-reply:
		switch {
		case !ok:
			return fmt.Errorf("session: the conversation ended before %s was answered", name)
		case !r.accepted && r.reason != "":
			return fmt.Errorf("session: the peer declined %s: %s", name, sanitizeText(r.reason))
		case !r.accepted:
			return fmt.Errorf("session: the peer declined %s", name)
		}
		return nil

	case <-timer.C:
		return fmt.Errorf("session: the peer never answered the offer of %s", name)

	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendBody streams the file in chunks and finishes with its digest.
func (s *Session) sendBody(
	ctx context.Context,
	f *os.File,
	id uint32,
	name string,
	size int64,
	progress func(sent, total int64),
) error {
	sum := sha256.New()
	buf := make([]byte, proto.ChunkSize)
	var sent int64

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			sum.Write(chunk)

			if err := s.c.WriteChunk(id, chunk); err != nil {
				return fmt.Errorf("session: sending %s: %w", name, err)
			}

			sent += int64(n)
			if progress != nil {
				progress(sent, size)
			}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("session: reading %s: %w", name, readErr)
		}
	}

	done := proto.FileDone{ID: id, SHA256: hex.EncodeToString(sum.Sum(nil))}
	if err := s.c.WriteJSON(proto.TypeFileDone, done); err != nil {
		return fmt.Errorf("session: finishing %s: %w", name, err)
	}

	logger.Info("file sent", "name", name, "bytes", sent)
	return nil
}

// -------------------------------------------------------------- receiving

// handleFileFrame processes a file frame, reporting whether it recognized
// it. Failures are told to the person rather than returned, because one bad
// transfer should not end a conversation.
func (s *Session) handleFileFrame(f proto.Frame) bool {
	switch f.Type {
	case proto.TypeFileOffer:
		s.onOffer(f)
	case proto.TypeFileAccept, proto.TypeFileReject:
		s.onAnswer(f)
	case proto.TypeFileChunk:
		s.onChunk(f)
	case proto.TypeFileDone:
		s.onDone(f)
	default:
		return false
	}
	return true
}

// onAnswer wakes the SendFile goroutine waiting on this offer.
func (s *Session) onAnswer(f proto.Frame) {
	var (
		id     uint32
		reply  offerReply
		reason string
	)

	if f.Type == proto.TypeFileAccept {
		var msg proto.FileAccept
		if err := proto.DecodeJSON(f, &msg); err != nil {
			logger.Warn("unreadable file answer", "err", err)
			return
		}
		id, reply = msg.ID, offerReply{accepted: true}
	} else {
		var msg proto.FileReject
		if err := proto.DecodeJSON(f, &msg); err != nil {
			logger.Warn("unreadable file answer", "err", err)
			return
		}
		id, reason = msg.ID, msg.Reason
		reply = offerReply{accepted: false, reason: reason}
	}

	if !s.files.answer(id, reply) {
		logger.Debug("answer for an unknown offer", "id", id)
	}
}

// onOffer asks the person about an incoming file and sets up its download.
func (s *Session) onOffer(f proto.Frame) {
	var msg proto.FileOffer
	if err := proto.DecodeJSON(f, &msg); err != nil {
		logger.Warn("unreadable file offer", "err", err)
		return
	}

	name := safeFileName(msg.Name)

	fh, ok := s.handler.(FileHandler)
	if !ok {
		s.decline(msg.ID, "this peer cannot receive files")
		return
	}
	if msg.Size < 0 {
		s.decline(msg.ID, "the offer had an impossible size")
		fh.OnFileError(name, errors.New("session: the offer had a negative size"))
		return
	}

	dir, accept, reason := fh.OnFileOffer(name, msg.Size)
	if !accept {
		s.decline(msg.ID, reason)
		return
	}

	in, err := startDownload(dir, name, msg.Size)
	if err != nil {
		s.decline(msg.ID, "the file could not be created here")
		fh.OnFileError(name, err)
		return
	}

	s.files.begin(msg.ID, in)

	if err := s.c.WriteJSON(proto.TypeFileAccept, proto.FileAccept{ID: msg.ID}); err != nil {
		s.files.finish(msg.ID)
		in.close(false)
		fh.OnFileError(name, fmt.Errorf("session: accepting %s: %w", name, err))
		return
	}

	logger.Info("file accepted", "name", name, "size", msg.Size)
}

// startDownload creates the partial file the chunks will be written into.
func startDownload(dir, name string, size int64) (*incoming, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("session: no directory was given for the download")
	}

	final := uniquePath(filepath.Join(dir, name))
	tmp := final + ".part"

	// O_EXCL so a name that appeared between the check and now is not
	// silently overwritten.
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("session: creating %s: %w", tmp, err)
	}

	return &incoming{
		name:      name,
		tmpPath:   tmp,
		finalPath: final,
		file:      file,
		hash:      sha256.New(),
		size:      size,
	}, nil
}

// uniquePath finds a free name near path, so an incoming file never
// destroys one already there.
func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s (%d)%s", base, time.Now().UnixNano(), ext)
}

// onChunk appends one piece of an accepted file.
func (s *Session) onChunk(f proto.Frame) {
	id, data, err := proto.DecodeChunk(f)
	if err != nil {
		logger.Warn("unreadable file chunk", "err", err)
		return
	}

	in, ok := s.files.lookup(id)
	if !ok {
		// A chunk for a transfer we declined or already ended. Dropping
		// it is correct: we never agreed to it.
		logger.Debug("chunk for an unknown transfer", "id", id)
		return
	}

	if in.size > 0 && in.got+int64(len(data)) > in.size {
		s.failTransfer(id, in, errors.New("session: the peer sent more than it offered"))
		return
	}

	if _, err := in.file.Write(data); err != nil {
		s.failTransfer(id, in, fmt.Errorf("session: writing %s: %w", in.name, err))
		return
	}
	in.hash.Write(data)
	in.got += int64(len(data))

	if fh, ok := s.handler.(FileHandler); ok {
		fh.OnFileProgress(in.name, in.got, in.size)
	}
}

// onDone verifies a finished file and puts it in place.
func (s *Session) onDone(f proto.Frame) {
	var msg proto.FileDone
	if err := proto.DecodeJSON(f, &msg); err != nil {
		logger.Warn("unreadable end of transfer", "err", err)
		return
	}

	in, ok := s.files.lookup(msg.ID)
	if !ok {
		logger.Debug("end of an unknown transfer", "id", msg.ID)
		return
	}
	s.files.finish(msg.ID)

	fh, hasHandler := s.handler.(FileHandler)

	if in.size > 0 && in.got != in.size {
		in.close(false)
		if hasHandler {
			fh.OnFileError(in.name, fmt.Errorf(
				"session: %s arrived incomplete: %d of %d bytes", in.name, in.got, in.size,
			))
		}
		return
	}

	got := hex.EncodeToString(in.hash.Sum(nil))
	if !strings.EqualFold(got, msg.SHA256) {
		// TCP catches damage in transit, so a mismatch means the two
		// sides disagree about the content. Keeping the file would be
		// handing over something unverified.
		in.close(false)
		if hasHandler {
			fh.OnFileError(in.name, fmt.Errorf(
				"session: %s failed its checksum and was discarded", in.name,
			))
		}
		return
	}

	in.close(true)
	if err := os.Rename(in.tmpPath, in.finalPath); err != nil {
		_ = os.Remove(in.tmpPath)
		if hasHandler {
			fh.OnFileError(in.name, fmt.Errorf("session: saving %s: %w", in.name, err))
		}
		return
	}

	logger.Info("file received", "name", in.name, "bytes", in.got)
	if hasHandler {
		fh.OnFileDone(in.name, in.finalPath)
	}
}

// decline answers an offer with no. A failure to send the refusal is only
// logged: the peer will time out, and there is nothing else to try.
func (s *Session) decline(id uint32, reason string) {
	msg := proto.FileReject{ID: id, Reason: reason}
	if err := s.c.WriteJSON(proto.TypeFileReject, msg); err != nil {
		logger.Debug("could not send refusal", "err", err)
	}
	logger.Info("file declined", "reason", reason)
}

// failTransfer ends one download badly, cleaning up after it.
func (s *Session) failTransfer(id uint32, in *incoming, err error) {
	s.files.finish(id)
	in.close(false)

	if fh, ok := s.handler.(FileHandler); ok {
		fh.OnFileError(in.name, err)
	}
	logger.Warn("transfer failed", "name", in.name, "err", err)
}

// abortTransfers is called when the conversation ends. It throws away
// partial downloads and releases anyone waiting for an answer, so no
// goroutine is left blocked on a peer that has gone.
func (s *Session) abortTransfers() {
	incomplete, waiting := s.files.drain()

	fh, hasHandler := s.handler.(FileHandler)
	for _, in := range incomplete {
		in.close(false)
		if hasHandler {
			fh.OnFileError(in.name, errors.New("session: the conversation ended mid-transfer"))
		}
	}

	for _, ch := range waiting {
		close(ch)
	}
}

// ------------------------------------------------------------ bookkeeping

func (fs *fileState) newOffer() (id uint32, reply <-chan offerReply) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.pending == nil {
		fs.pending = make(map[uint32]chan offerReply)
	}

	fs.nextID++
	id = fs.nextID

	// Buffered, so answering never blocks the read loop even if the
	// sender has already given up.
	ch := make(chan offerReply, 1)
	fs.pending[id] = ch
	return id, ch
}

func (fs *fileState) dropOffer(id uint32) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.pending, id)
}

func (fs *fileState) answer(id uint32, r offerReply) bool {
	fs.mu.Lock()
	ch, ok := fs.pending[id]
	fs.mu.Unlock()

	if !ok {
		return false
	}
	select {
	case ch <- r:
	default: // already answered
	}
	return true
}

func (fs *fileState) begin(id uint32, in *incoming) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.active == nil {
		fs.active = make(map[uint32]*incoming)
	}
	fs.active[id] = in
}

func (fs *fileState) lookup(id uint32) (*incoming, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	in, ok := fs.active[id]
	return in, ok
}

func (fs *fileState) finish(id uint32) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.active, id)
}

// drain empties both maps and returns what was in them, so the caller can
// clean up without holding the lock.
func (fs *fileState) drain() (incomplete []*incoming, waiting []chan offerReply) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	incomplete = make([]*incoming, 0, len(fs.active))
	for id, in := range fs.active {
		incomplete = append(incomplete, in)
		delete(fs.active, id)
	}

	waiting = make([]chan offerReply, 0, len(fs.pending))
	for id, ch := range fs.pending {
		waiting = append(waiting, ch)
		delete(fs.pending, id)
	}

	return incomplete, waiting
}
