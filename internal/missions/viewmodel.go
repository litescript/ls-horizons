package missions

import (
	"fmt"
	"time"
)

// FormatCountdown formats a duration as a human-readable countdown string.
func FormatCountdown(d time.Duration) string {
	if d < 0 {
		return "T+ " + fmtDur(-d)
	}
	return "T- " + fmtDur(d)
}

// FormatMET formats mission elapsed time.
func FormatMET(d time.Duration) string {
	if d < 0 {
		return "T- " + fmtDur(-d)
	}
	return "T+ " + fmtDur(d)
}

func fmtDur(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm %02ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%02dh %02dm %02ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02dm %02ds", minutes, seconds)
}

// CrewBadge returns a glyph indicating crewed status.
func CrewBadge(crewed bool) string {
	if crewed {
		return "★"
	}
	return ""
}
