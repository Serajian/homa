// Package proto implements homa's wire protocol: length-prefixed frames
// carried over any io.ReadWriter. It knows nothing about networks or files.
package proto

import "fmt"

// Type identifies what a frame carries.
type Type uint8

const (
	TypeHello      Type = 0x01
	TypeText       Type = 0x02
	TypeFileOffer  Type = 0x03
	TypeFileAccept Type = 0x04
	TypeFileReject Type = 0x05
	TypeFileChunk  Type = 0x06
	TypeFileDone   Type = 0x07
	TypeBye        Type = 0x08
)

func (t Type) String() string {
	switch t {
	case TypeHello:
		return "HELLO"
	case TypeText:
		return "TEXT"
	case TypeFileOffer:
		return "FILE_OFFER"
	case TypeFileAccept:
		return "FILE_ACCEPT"
	case TypeFileReject:
		return "FILE_REJECT"
	case TypeFileChunk:
		return "FILE_CHUNK"
	case TypeFileDone:
		return "FILE_DONE"
	case TypeBye:
		return "BYE"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", uint8(t))
	}
}

// headerSize is the fixed prefix on every frame: a 4-byte big-endian payload
// length followed by a 1-byte type.
//
//	+----------+--------+------------------+
//	| 4 bytes  | 1 byte |   N bytes        |
//	| N (BE)   |  type  |   payload        |
//	+----------+--------+------------------+
const headerSize = 5

// MaxPayload caps a single frame's payload. Without a cap, a peer could
// announce a huge length and make us allocate that much in one read.
const MaxPayload = 1 << 20 // 1 MiB

// ChunkSize is how much file data one FILE_CHUNK frame carries. Small enough
// that chat messages slip between chunks without a noticeable pause.
const ChunkSize = 32 * 1024

// Frame is one decoded message off the wire.
type Frame struct {
	Type    Type
	Payload []byte
}
