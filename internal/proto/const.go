package proto

// Version is the protocol version announced in Hello. Peers running different
// versions may still connect; each side can warn its user.
const Version = 1

// headerSize is the fixed prefix on every frame: a 4-byte big-endian payload
// length followed by a 1-byte type.
//
//	+----------+--------+------------------+
//	| 4 bytes  | 1 byte |   N bytes        |
//	| N (BE)   |  type  |   payload        |
//	+----------+--------+------------------+
const headerSize = 5

// MaxPayload caps a single frame's payload. Without a cap, a peer could
// announce a huge length and make us allocate that much in one read. It sits
// about thirty times above the largest frame homa actually produces.
const MaxPayload = 1 << 20 // 1 MiB

// ChunkSize is how much file data one FILE_CHUNK frame carries. Small enough
// that chat messages slip between chunks without a noticeable pause.
const ChunkSize = 32 * 1024

// readBufSize lets several small frames arrive in one syscall.
const readBufSize = 64 * 1024
