package cli

import (
	"errors"
	"testing"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
)

func TestErrorHints(t *testing.T) {
	tests := []struct {
		err      error
		wantHint string
	}{
		{vserrors.ErrVibespaceNotFound, "vibespace list"},
		{vserrors.ErrAgentNotFound, "agent list"},
		{vserrors.ErrClusterNotInitialized, "vibespace init"},
		{vserrors.ErrClusterNotRunning, "vibespace init"},
		{vserrors.ErrDaemonNotRunning, "auto-start"},
		{vserrors.ErrForwardNotFound, "forward list"},
		{vserrors.ErrNoAgents, "agent create"},
		{vserrors.ErrRemoteNotConnected, "remote connect"},
		{vserrors.ErrWireGuardNotAvailable, "wireguard-tools"},
		{vserrors.ErrRemoteAlreadyConnected, "remote disconnect"},
		{vserrors.ErrInvalidToken, "generate-token"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			hint := getErrorHint(tt.err)
			if hint == "" {
				t.Errorf("getErrorHint(%v) returned empty string", tt.err)
			}
			if len(hint) < 5 {
				t.Errorf("getErrorHint(%v) returned suspiciously short hint: %q", tt.err, hint)
			}
		})
	}
}

func TestErrorHintUnknown(t *testing.T) {
	hint := getErrorHint(errors.New("unknown error"))
	if hint != "" {
		t.Errorf("getErrorHint(unknown) = %q, want empty string", hint)
	}
}
