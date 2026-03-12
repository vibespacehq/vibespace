package tui

import (
	"strings"
	"testing"
)

func TestRenderWelcomeNotInstalled(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusNotInstalled, false, true, false, "")
	plain := stripAnsi(out)

	if !strings.Contains(plain, "Not installed") {
		t.Error("expected 'Not installed' in output")
	}
	if !strings.Contains(plain, "vibespace") {
		t.Error("expected 'vibespace' logo in output")
	}
}

func TestRenderWelcomeRunning(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusRunning, true, true, false, "")
	plain := stripAnsi(out)

	if !strings.Contains(plain, "Running") {
		t.Error("expected 'Running' in output")
	}
}

func TestRenderWelcomeCompactNoPanic(t *testing.T) {
	out := renderWelcome(80, 15, clusterStatusStopped, false, true, false, "")
	plain := stripAnsi(out)

	if !strings.Contains(plain, "Stopped") {
		t.Error("expected 'Stopped' in compact output")
	}
	if strings.Contains(plain, "Quick Start") {
		t.Error("compact mode should not show Quick Start")
	}
}

func TestRenderWelcomeQuickStartClusterDone(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusRunning, false, true, false, "")
	plain := stripAnsi(out)

	if !strings.Contains(plain, "Quick Start") {
		t.Error("expected 'Quick Start' section in output")
	}
	if !strings.Contains(plain, "Connect to a cluster") {
		t.Error("expected step 1 in output")
	}
}

func TestRenderWelcomeTinyTerminal(t *testing.T) {
	out := renderWelcome(20, 5, clusterStatusUnknown, false, true, false, "")
	if out == "" {
		t.Error("expected non-empty output even for tiny terminal")
	}
}

func TestRenderWelcomeBlinkOff(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusRunning, true, false, false, "")
	plain := stripAnsi(out)

	if strings.Contains(plain, "●") {
		t.Error("expected no filled dot when blink is off")
	}
	if !strings.Contains(plain, "Running") {
		t.Error("expected 'Running' in output")
	}
}

func TestRenderWelcomeUpdateAvailable(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusRunning, true, true, true, "v0.5.0")
	plain := stripAnsi(out)

	if !strings.Contains(plain, "Update") {
		t.Error("expected 'Update' label in output")
	}
	if !strings.Contains(plain, "v0.5.0 available") {
		t.Error("expected version available text in output")
	}
}

func TestRenderWelcomeNoUpdateWhenNotAvailable(t *testing.T) {
	out := renderWelcome(120, 40, clusterStatusRunning, true, true, false, "")
	plain := stripAnsi(out)

	if strings.Contains(plain, "Update") {
		t.Error("should not show Update line when no update available")
	}
}
