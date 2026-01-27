package ui

// Message prefix icons (Unicode)
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "⚠"
	IconStep    = "→"
	IconInfo    = "●"
)

// Plain text fallbacks for NO_COLOR mode
const (
	PlainSuccess = "[ok]"
	PlainError   = "[error]"
	PlainWarning = "[!]"
	PlainStep    = "[->]"
	PlainInfo    = "[i]"
)

// Prefix returns the appropriate prefix icon based on noColor setting.
func SuccessPrefix(noColor bool) string {
	if noColor {
		return PlainSuccess
	}
	return IconSuccess
}

func ErrorPrefix(noColor bool) string {
	if noColor {
		return PlainError
	}
	return IconError
}

func WarningPrefix(noColor bool) string {
	if noColor {
		return PlainWarning
	}
	return IconWarning
}

func StepPrefix(noColor bool) string {
	if noColor {
		return PlainStep
	}
	return IconStep
}

func InfoPrefix(noColor bool) string {
	if noColor {
		return PlainInfo
	}
	return IconInfo
}
