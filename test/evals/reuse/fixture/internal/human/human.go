// Package human formats byte counts for the people reading the output.
package human

import "fmt"

// Bytes renders a byte count with one decimal in IEC units:
//
//	Bytes(512)     == "512 B"
//	Bytes(1536)    == "1.5 KiB"
//	Bytes(2097152) == "2.0 MiB"
//
// The report subcommand has printed sizes this way since it shipped, and
// anything else that shows a size is expected to agree with it.
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
