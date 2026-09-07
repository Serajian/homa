package ui

import (
	"fmt"
	"strings"
)

// humanBytes renders a size the way a person reads it. Powers of 1024,
// since that is what a file manager shows and what people compare against.
func humanBytes(n int64) string {
	const unit = 1024

	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// percent is progress as a whole number, guarding against the unknown
// total a peer could report as zero.
func percent(done, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(done * 100 / total)
}

// reason is an error as a person should read it: without the package prefix
// that tells them which part of homa noticed.
//
// Every package prefixes its errors with its own name, so "session: reading
// from the peer: ..." becomes "reading from the peer: ...". The prefixes are
// listed rather than matched by shape, because stripping anything before the
// first colon would eat the start of a message that happens to contain one.
//
// This was three near-identical functions before a fourth was needed.
func reason(err error) string {
	if err == nil {
		return ""
	}

	s := err.Error()
	for _, p := range []string{"session: ", "contacts: ", "config: ", "peer: ", "paths: ", "proto: ", "ui: "} {
		if strings.HasPrefix(s, p) {
			return strings.TrimPrefix(s, p)
		}
	}
	return s
}
