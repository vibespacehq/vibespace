package platform

import "testing"

func TestPlatformString(t *testing.T) {
	tests := []struct {
		p    Platform
		want string
	}{
		{Platform{OS: "darwin", Arch: "arm64"}, "macOS (arm64)"},
		{Platform{OS: "linux", Arch: "amd64"}, "Linux (amd64)"},
		{Platform{OS: "windows", Arch: "amd64"}, "Windows (amd64)"},
		{Platform{OS: "freebsd", Arch: "arm64"}, "freebsd (arm64)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("Platform.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsMacOS(t *testing.T) {
	if !((Platform{OS: "darwin"}).IsMacOS()) {
		t.Error("Platform{OS:darwin}.IsMacOS() = false, want true")
	}
	if (Platform{OS: "linux"}).IsMacOS() {
		t.Error("Platform{OS:linux}.IsMacOS() = true, want false")
	}
}

func TestIsLinux(t *testing.T) {
	if !((Platform{OS: "linux"}).IsLinux()) {
		t.Error("Platform{OS:linux}.IsLinux() = false, want true")
	}
	if (Platform{OS: "darwin"}).IsLinux() {
		t.Error("Platform{OS:darwin}.IsLinux() = true, want false")
	}
}

func TestIsARM(t *testing.T) {
	if !((Platform{Arch: "arm64"}).IsARM()) {
		t.Error("Platform{Arch:arm64}.IsARM() = false, want true")
	}
	if (Platform{Arch: "amd64"}).IsARM() {
		t.Error("Platform{Arch:amd64}.IsARM() = true, want false")
	}
}
