// Package proto implements homa's wire protocol: length-prefixed frames
// carried over any io.ReadWriter. It knows nothing about networks or files.
package proto

import "fmt"

// Type identifies what a frame carries.
type Type uint8

// The frame types homa exchanges.
const (
	TypeHello      Type = 0x01
	TypeText       Type = 0x02
	TypeFileOffer  Type = 0x03
	TypeFileAccept Type = 0x04
	TypeFileReject Type = 0x05
	TypeFileChunk  Type = 0x06
	TypeFileDone   Type = 0x07
	TypeBye        Type = 0x08
	TypeAccept     Type = 0x09
	TypeAddress    Type = 0x0a
	TypeFileCancel Type = 0x0b
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
	case TypeAddress:
		return "ADDRESS"
	case TypeFileCancel:
		return "FILE_CANCEL"
	case TypeBye:
		return "BYE"
	case TypeAccept:
		return "ACCEPT"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", uint8(t))
	}
}

// Frame is one decoded message off the wire.
type Frame struct {
	Type    Type
	Payload []byte
}
