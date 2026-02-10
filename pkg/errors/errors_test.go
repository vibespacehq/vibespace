package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCode(t *testing.T) {
	tests := []struct {
		err      error
		wantExit int
		wantCode string
	}{
		{ErrVibespaceNotFound, ExitNotFound, "NOT_FOUND"},
		{ErrAgentNotFound, ExitNotFound, "NOT_FOUND"},
		{ErrForwardNotFound, ExitNotFound, "NOT_FOUND"},
		{ErrVibespaceExists, ExitConflict, "CONFLICT"},
		{ErrClusterNotRunning, ExitUnavailable, "CLUSTER_UNAVAILABLE"},
		{ErrClusterNotInitialized, ExitUnavailable, "CLUSTER_UNAVAILABLE"},
		{ErrClusterUnreachable, ExitUnavailable, "CLUSTER_UNAVAILABLE"},
		{ErrDaemonNotRunning, ExitUnavailable, "DAEMON_UNAVAILABLE"},
		{ErrKubernetesNotAvailable, ExitUnavailable, "K8S_UNAVAILABLE"},
		{ErrDaemonStartTimeout, ExitTimeout, "TIMEOUT"},
		{ErrInvalidName, ExitUsage, "INVALID_INPUT"},
		{ErrSSHKeyNotFound, ExitNotFound, "SSH_KEY_NOT_FOUND"},
		{ErrNoAgents, ExitNotFound, "NO_AGENTS"},
		{ErrNotConnected, ExitUnavailable, "NOT_CONNECTED"},
		{ErrRemoteNotConnected, ExitUnavailable, "REMOTE_NOT_CONNECTED"},
		{ErrWireGuardNotAvailable, ExitUnavailable, "WIREGUARD_NOT_AVAILABLE"},
		{ErrRemoteAlreadyConnected, ExitConflict, "REMOTE_ALREADY_CONNECTED"},
		{ErrInvalidToken, ExitPermission, "INVALID_TOKEN"},
		{ErrInviteTokenInvalid, ExitPermission, "INVALID_TOKEN"},
		{ErrInviteTokenExpired, ExitPermission, "INVALID_TOKEN"},
		{ErrInviteTokenSignatureInvalid, ExitPermission, "INVALID_TOKEN"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			gotExit, gotCode := ErrorCode(tt.err)
			if gotExit != tt.wantExit {
				t.Errorf("ErrorCode(%v) exitCode = %d, want %d", tt.err, gotExit, tt.wantExit)
			}
			if gotCode != tt.wantCode {
				t.Errorf("ErrorCode(%v) code = %q, want %q", tt.err, gotCode, tt.wantCode)
			}
		})
	}
}

func TestErrorCodeWrapped(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", ErrVibespaceNotFound)
	gotExit, gotCode := ErrorCode(wrapped)
	if gotExit != ExitNotFound || gotCode != "NOT_FOUND" {
		t.Errorf("ErrorCode(wrapped) = (%d, %q), want (%d, %q)", gotExit, gotCode, ExitNotFound, "NOT_FOUND")
	}
}

func TestErrorCodeUnknown(t *testing.T) {
	unknown := errors.New("something unexpected")
	gotExit, gotCode := ErrorCode(unknown)
	if gotExit != ExitInternal || gotCode != "INTERNAL" {
		t.Errorf("ErrorCode(unknown) = (%d, %q), want (%d, %q)", gotExit, gotCode, ExitInternal, "INTERNAL")
	}
}
