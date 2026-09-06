package proto

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Hello introduces a peer. Nick is self-declared and therefore not proof of
// identity: the node key underneath the tunnel is what actually identifies
// the other side.
type Hello struct {
	Nick    string `json:"nick"`
	Version int    `json:"version"`
}

// FileOffer proposes a transfer. Nothing is sent until the receiver replies
// with FileAccept.
type FileOffer struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// FileAccept agrees to receive the offered file.
type FileAccept struct {
	ID uint32 `json:"id"`
}

// FileReject declines the offered file, optionally saying why.
type FileReject struct {
	ID     uint32 `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// FileDone ends a transfer. SHA256 is the hex digest of the whole file, so
// the receiver can check what it assembled.
type FileDone struct {
	ID     uint32 `json:"id"`
	SHA256 string `json:"sha256"`
}

// DecodeJSON unmarshals a frame's payload into v.
func DecodeJSON(f Frame, v any) error {
	if err := json.Unmarshal(f.Payload, v); err != nil {
		return fmt.Errorf("proto: decoding %s: %w", f.Type, err)
	}
	return nil
}

// DecodeChunk splits a FILE_CHUNK payload back into its id and data. The
// returned slice aliases f.Payload, so copy it if you keep it beyond the
// current loop iteration.
func DecodeChunk(f Frame) (id uint32, data []byte, err error) {
	if len(f.Payload) < 4 {
		return 0, nil, fmt.Errorf("proto: chunk frame of %d bytes is too short", len(f.Payload))
	}
	return binary.BigEndian.Uint32(f.Payload[:4]), f.Payload[4:], nil
}
