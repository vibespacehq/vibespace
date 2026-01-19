package platform

import (
	"runtime"
)

// Platform represents the detected operating system and architecture
type Platform struct {
	OS   string // "darwin", "linux", "windows"
	Arch string // "amd64", "arm64"
}

// Detect returns the current platform
func Detect() Platform {
	return Platform{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// IsMacOS returns true if running on macOS
func (p Platform) IsMacOS() bool {
	return p.OS == "darwin"
}

// IsLinux returns true if running on Linux
func (p Platform) IsLinux() bool {
	return p.OS == "linux"
}

// IsARM returns true if running on ARM architecture
func (p Platform) IsARM() bool {
	return p.Arch == "arm64"
}

// String returns a human-readable platform description
func (p Platform) String() string {
	osName := map[string]string{
		"darwin":  "macOS",
		"linux":   "Linux",
		"windows": "Windows",
	}[p.OS]
	if osName == "" {
		osName = p.OS
	}
	return osName + " (" + p.Arch + ")"
}
