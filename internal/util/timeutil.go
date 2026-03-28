package util

import (
	"fmt"
	"math/rand"
	"time"
)

// JitteredSleep returns a sleep duration with jitter applied.
// sleep is base seconds, jitter is percentage (0-50).
func JitteredSleep(sleep, jitter int) time.Duration {
	if jitter <= 0 || sleep <= 0 {
		return time.Duration(sleep) * time.Second
	}

	variance := float64(sleep) * float64(jitter) / 100.0
	offset := (rand.Float64() * 2 * variance) - variance
	total := float64(sleep) + offset

	if total < 1 {
		total = 1
	}

	return time.Duration(total * float64(time.Second))
}

// TimeAgo returns a human-readable "X ago" string.
func TimeAgo(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// FormatTimestamp formats a time for display.
func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
