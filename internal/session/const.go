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

// byeLinger bounds how long Close waits, after sending its goodbye, for
// the far side to close first. The goodbye is a frame in a tunnel run by
// tailcat's own goroutines, and closing the connection the instant it was
// written could drop it unsent; a peer that never hears it has no
// connection to see break, only a tunnel that has gone quiet, and shows the
// conversation open for minutes. The far side closes as soon as it reads
// the goodbye, so the wait usually ends in milliseconds; the bound is for a
// peer that does not.
const byeLinger = 500 * time.Millisecond
