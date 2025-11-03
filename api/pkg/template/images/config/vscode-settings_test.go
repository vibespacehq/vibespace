package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// VSCodeSettings represents the structure of vscode-settings.json
type VSCodeSettings struct {
	WorkbenchColorTheme         string            `json:"workbench.colorTheme"`
	WorkbenchColorCustomizations map[string]string `json:"workbench.colorCustomizations"`
	EditorFontFamily            string            `json:"editor.fontFamily"`
	EditorFontSize              int               `json:"editor.fontSize"`
	EditorLineHeight            float64           `json:"editor.lineHeight"`
	EditorFontLigatures         bool              `json:"editor.fontLigatures"`
	TerminalFontFamily          string            `json:"terminal.integrated.fontFamily"`
	TerminalFontSize            int               `json:"terminal.integrated.fontSize"`
	TelemetryLevel              string            `json:"telemetry.telemetryLevel"`
	RedhatTelemetry             bool              `json:"redhat.telemetry.enabled"`
	WorkbenchExperiments        bool              `json:"workbench.enableExperiments"`
	WorkbenchStartupEditor      string            `json:"workbench.startupEditor"`
	WorkbenchWelcomeWalkthroughs bool             `json:"workbench.welcomePage.walkthroughs.openOnInstall"`
	SecurityTrustStartupPrompt  string            `json:"security.workspace.trust.startupPrompt"`
	SecurityTrustUntrustedFiles string            `json:"security.workspace.trust.untrustedFiles"`
}

func TestVSCodeSettingsValidJSON(t *testing.T) {
	// Read the settings file
	settingsPath := filepath.Join("vscode-settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read vscode-settings.json: %v", err)
	}

	// Parse as generic JSON first to validate syntax
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		t.Fatalf("Invalid JSON in vscode-settings.json: %v", err)
	}

	// Parse into struct
	var settings VSCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse vscode-settings.json into struct: %v", err)
	}

	t.Logf("Successfully parsed vscode-settings.json with %d color customizations", len(settings.WorkbenchColorCustomizations))
}

func TestVSCodeSettingsDesignSystemColors(t *testing.T) {
	// Read and parse settings
	settingsPath := filepath.Join("vscode-settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read vscode-settings.json: %v", err)
	}

	var settings VSCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse vscode-settings.json: %v", err)
	}

	// Design system colors from SPEC.md Section 4.1.3
	expectedColors := map[string]string{
		"Pure Black": "#000000",
		"Teal":       "#00ABAB",
		"Pink":       "#F102F3",
		"Yellow":     "#F5F50A",
		"Orange":     "#FF7D4B",
	}

	t.Run("Pure Black Backgrounds", func(t *testing.T) {
		blackBgKeys := []string{
			"activityBar.background",
			"editor.background",
			"terminal.background",
			"panel.background",
			"statusBar.noFolderBackground",
		}

		for _, key := range blackBgKeys {
			if color, exists := settings.WorkbenchColorCustomizations[key]; !exists {
				t.Errorf("Missing color customization: %s", key)
			} else if color != expectedColors["Pure Black"] {
				t.Errorf("%s = %s, want %s", key, color, expectedColors["Pure Black"])
			}
		}
	})

	t.Run("Teal Accents", func(t *testing.T) {
		tealKeys := []string{
			"activityBar.foreground",
			"editorLineNumber.activeForeground",
			"editorCursor.foreground",
			"terminalCursor.foreground",
			"focusBorder",
		}

		for _, key := range tealKeys {
			if color, exists := settings.WorkbenchColorCustomizations[key]; !exists {
				t.Errorf("Missing color customization: %s", key)
			} else if color != expectedColors["Teal"] {
				t.Errorf("%s = %s, want %s", key, color, expectedColors["Teal"])
			}
		}
	})

	t.Run("Pink Highlights", func(t *testing.T) {
		pinkKeys := []string{
			"statusBar.background",
			"button.background",
			"activityBar.activeBorder",
			"badge.background",
		}

		for _, key := range pinkKeys {
			if color, exists := settings.WorkbenchColorCustomizations[key]; !exists {
				t.Errorf("Missing color customization: %s", key)
			} else if color != expectedColors["Pink"] {
				t.Errorf("%s = %s, want %s", key, color, expectedColors["Pink"])
			}
		}
	})

	t.Run("Yellow Hover States", func(t *testing.T) {
		yellowKeys := []string{
			"button.hoverBackground",
			"statusBarItem.hoverBackground",
		}

		for _, key := range yellowKeys {
			if color, exists := settings.WorkbenchColorCustomizations[key]; !exists {
				t.Errorf("Missing color customization: %s", key)
			} else if color != expectedColors["Yellow"] {
				t.Errorf("%s = %s, want %s", key, color, expectedColors["Yellow"])
			}
		}
	})

	t.Run("Orange Debugging", func(t *testing.T) {
		if color, exists := settings.WorkbenchColorCustomizations["statusBar.debuggingBackground"]; !exists {
			t.Error("Missing color customization: statusBar.debuggingBackground")
		} else if color != expectedColors["Orange"] {
			t.Errorf("statusBar.debuggingBackground = %s, want %s", color, expectedColors["Orange"])
		}
	})
}

func TestVSCodeSettingsFontConfiguration(t *testing.T) {
	settingsPath := filepath.Join("vscode-settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read vscode-settings.json: %v", err)
	}

	var settings VSCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse vscode-settings.json: %v", err)
	}

	t.Run("JetBrains Mono Font", func(t *testing.T) {
		expectedFont := "JetBrains Mono, monospace"

		if settings.EditorFontFamily != expectedFont {
			t.Errorf("editor.fontFamily = %s, want %s", settings.EditorFontFamily, expectedFont)
		}

		if settings.TerminalFontFamily != expectedFont {
			t.Errorf("terminal.integrated.fontFamily = %s, want %s", settings.TerminalFontFamily, expectedFont)
		}
	})

	t.Run("Font Size", func(t *testing.T) {
		if settings.EditorFontSize != 14 {
			t.Errorf("editor.fontSize = %d, want 14", settings.EditorFontSize)
		}

		if settings.TerminalFontSize != 14 {
			t.Errorf("terminal.integrated.fontSize = %d, want 14", settings.TerminalFontSize)
		}
	})

	t.Run("Font Ligatures Enabled", func(t *testing.T) {
		if !settings.EditorFontLigatures {
			t.Error("editor.fontLigatures should be true")
		}
	})

	t.Run("Line Height", func(t *testing.T) {
		if settings.EditorLineHeight != 1.6 {
			t.Errorf("editor.lineHeight = %f, want 1.6", settings.EditorLineHeight)
		}
	})
}

func TestVSCodeSettingsUXConfiguration(t *testing.T) {
	settingsPath := filepath.Join("vscode-settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read vscode-settings.json: %v", err)
	}

	var settings VSCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse vscode-settings.json: %v", err)
	}

	t.Run("Telemetry Disabled", func(t *testing.T) {
		if settings.TelemetryLevel != "off" {
			t.Errorf("telemetry.telemetryLevel = %s, want 'off'", settings.TelemetryLevel)
		}

		if settings.RedhatTelemetry {
			t.Error("redhat.telemetry.enabled should be false")
		}
	})

	t.Run("Experiments Disabled", func(t *testing.T) {
		if settings.WorkbenchExperiments {
			t.Error("workbench.enableExperiments should be false")
		}
	})

	t.Run("Welcome Page Disabled", func(t *testing.T) {
		if settings.WorkbenchStartupEditor != "none" {
			t.Errorf("workbench.startupEditor = %s, want 'none'", settings.WorkbenchStartupEditor)
		}

		if settings.WorkbenchWelcomeWalkthroughs {
			t.Error("workbench.welcomePage.walkthroughs.openOnInstall should be false")
		}
	})

	t.Run("Workspace Trust Configured", func(t *testing.T) {
		if settings.SecurityTrustStartupPrompt != "never" {
			t.Errorf("security.workspace.trust.startupPrompt = %s, want 'never'", settings.SecurityTrustStartupPrompt)
		}

		if settings.SecurityTrustUntrustedFiles != "open" {
			t.Errorf("security.workspace.trust.untrustedFiles = %s, want 'open'", settings.SecurityTrustUntrustedFiles)
		}
	})
}

func TestVSCodeSettingsColorCoverage(t *testing.T) {
	settingsPath := filepath.Join("vscode-settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read vscode-settings.json: %v", err)
	}

	var settings VSCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse vscode-settings.json: %v", err)
	}

	// Ensure comprehensive theme coverage (90+ UI elements)
	minColorCustomizations := 90
	if len(settings.WorkbenchColorCustomizations) < minColorCustomizations {
		t.Errorf("Only %d color customizations found, want at least %d for comprehensive coverage",
			len(settings.WorkbenchColorCustomizations), minColorCustomizations)
	}

	// Verify critical UI areas are themed
	criticalAreas := []string{
		// Activity Bar
		"activityBar.background",
		"activityBar.foreground",
		// Sidebar
		"sideBar.background",
		"sideBar.foreground",
		// Editor
		"editor.background",
		"editor.foreground",
		"editorCursor.foreground",
		// Terminal
		"terminal.background",
		"terminalCursor.foreground",
		// Status Bar
		"statusBar.background",
		"statusBar.foreground",
		// Buttons
		"button.background",
		"button.hoverBackground",
		// Tabs
		"tab.activeBackground",
		"tab.activeBorder",
	}

	for _, area := range criticalAreas {
		if _, exists := settings.WorkbenchColorCustomizations[area]; !exists {
			t.Errorf("Missing critical UI area customization: %s", area)
		}
	}
}
