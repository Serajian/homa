package ui

import "fmt"

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
