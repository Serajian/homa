package ui

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// perSecond is how fast a transfer is moving, in the same units its size is
// shown in. Under a second of it is not a measurement, so it says nothing.
func perSecond(n int64, d time.Duration) string {
	if d < time.Second || n <= 0 {
		return ""
	}
	return humanBytes(int64(float64(n)/d.Seconds())) + "/s"
}

// leftText is how much longer a transfer has, given what it has done so far
// and how long that took. It is deliberately vague: a rate that has been
// steady for two seconds is not a promise about the next two minutes.
func leftText(remaining, done int64, d time.Duration) string {
	if d < time.Second || done <= 0 || remaining <= 0 {
		return ""
	}
	left := time.Duration(float64(remaining) / (float64(done) / d.Seconds()) * float64(time.Second))
	switch {
	case left < 10*time.Second:
		return "a moment left"
	case left < time.Minute:
		return fmt.Sprintf("about %ds left", int(left.Seconds()))
	case left < time.Hour:
		return fmt.Sprintf("about %dm left", int(left.Minutes()))
	}
	return fmt.Sprintf("about %dh%dm left", int(left.Hours()), int(left.Minutes())%60)
}

// humanBytes renders a size the way a person reads it. Powers of 1024,
// since that is what a file manager shows and what people compare against.
func humanBytes(n int64) string {
	const unit = 1024

	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	const units = "KMGTPE"

	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}

	// A tenth under the next unit rounds to it: 1023.97 KB would print as
	// "1024.0 KB", which is a megabyte written the long way round.
	value := float64(n) / float64(div)
	if math.Round(value*10)/10 >= unit && exp < len(units)-1 {
		value /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", value, units[exp])
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
