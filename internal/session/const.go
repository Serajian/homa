package session

import "time"

// handshakeTimeout bounds the opening exchange. A peer that connects and
// then says nothing must not hold a slot forever.
const handshakeTimeout = 15 * time.Second

// MaxTextLen bounds one chat message. Long enough for a paragraph, short
// enough that nobody can flood a terminal with a single line.
const MaxTextLen = 4096

// MaxNickLen bounds how much of a peer's announced name we will display.
const MaxNickLen = 32

// offerTimeout is how long a sender waits for the other person to answer a
// file offer. Generous, because answering means a human noticing a prompt.
const offerTimeout = 5 * time.Minute

// MaxFileNameLen bounds a received file name. Most filesystems stop at 255
// bytes for one path element.
const MaxFileNameLen = 200
