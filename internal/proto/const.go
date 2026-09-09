package proto

import "time"

// Version is the protocol version announced in Hello. Peers running different
// versions may still connect; each side can warn its user.
//
// 1: the original framing.
// 2: TypeAccept, so a caller learns when the other person actually took the
//
//	call rather than assuming the handshake meant yes.
//
// 3: TypeAddress, so somebody who was called can be given the address they
//
//	need to call back; and TypeFileCancel, so a transfer can be stopped
//	from either end instead of only by leaving.
const Version = 3

// VersionAccept is the first version whose peers send TypeAccept. A caller
// talking to anything older has nothing to wait for, and must not wait.
const VersionAccept = 2

// VersionCancel is the first version whose peers understand TypeFileCancel.
// An older one keeps sending, or keeps waiting for chunks that will never
// come, so the person is told what stopping will and will not reach.
const VersionCancel = 3

// VersionAddress is the first version whose peers understand TypeAddress.
// An older peer drops what it does not recognize, silently and by design, so
// the sender has to know before offering rather than after being ignored.
const VersionAddress = 3

// AnswerTimeout is how long the person being called may take to answer before
// the caller is entitled to give up. It is an agreement between two peers, not
// a setting: one side promises to answer or hang up within it, and the other
// relies on that promise.
const AnswerTimeout = 60 * time.Second

// AnswerGrace is how much longer the calling side waits before giving up on
// its own. The other side promises to answer or hang up inside AnswerTimeout,
// so this is only the margin for that message being in flight, and for a peer
// whose clock or scheduler is slower than ours. Giving up first would turn
// somebody reaching for the keyboard into a dropped call.
const AnswerGrace = 15 * time.Second

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
