// Package errors provides sentinel errors for the vibespace CLI.
// Use [errors.Is] to check for specific error conditions.
package errors

import "errors"

// Exit codes for CLI
const (
	ExitSuccess     = 0  // Successful execution
	ExitInternal    = 1  // Internal/unknown error
	ExitUsage       = 2  // Invalid usage (bad flags, args)
	ExitNotFound    = 10 // Resource not found (vibespace, agent)
	ExitConflict    = 11 // Resource conflict (already exists)
	ExitPermission  = 12 // Permission denied
	ExitTimeout     = 13 // Operation timed out
	ExitCancelled   = 14 // Operation cancelled by user
	ExitUnavailable = 15 // Service unavailable (cluster/daemon not running)
)

// ErrorCode returns the appropriate exit code and error code string for an error.
func ErrorCode(err error) (exitCode int, code string) {
	switch {
	case errors.Is(err, ErrVibespaceNotFound), errors.Is(err, ErrAgentNotFound), errors.Is(err, ErrForwardNotFound):
		return ExitNotFound, "NOT_FOUND"
	case errors.Is(err, ErrVibespaceExists):
		return ExitConflict, "CONFLICT"
	case errors.Is(err, ErrClusterNotRunning), errors.Is(err, ErrClusterNotInitialized), errors.Is(err, ErrClusterUnreachable):
		return ExitUnavailable, "CLUSTER_UNAVAILABLE"
	case errors.Is(err, ErrDaemonNotRunning):
		return ExitUnavailable, "DAEMON_UNAVAILABLE"
	case errors.Is(err, ErrKubernetesNotAvailable):
		return ExitUnavailable, "K8S_UNAVAILABLE"
	case errors.Is(err, ErrDaemonStartTimeout):
		return ExitTimeout, "TIMEOUT"
	case errors.Is(err, ErrInvalidName):
		return ExitUsage, "INVALID_INPUT"
	case errors.Is(err, ErrSSHKeyNotFound):
		return ExitNotFound, "SSH_KEY_NOT_FOUND"
	case errors.Is(err, ErrNoAgents):
		return ExitNotFound, "NO_AGENTS"
	case errors.Is(err, ErrNotConnected):
		return ExitUnavailable, "NOT_CONNECTED"
	case errors.Is(err, ErrRemoteNotConnected):
		return ExitUnavailable, "REMOTE_NOT_CONNECTED"
	case errors.Is(err, ErrWireGuardNotAvailable):
		return ExitUnavailable, "WIREGUARD_NOT_AVAILABLE"
	case errors.Is(err, ErrRemoteAlreadyConnected):
		return ExitConflict, "REMOTE_ALREADY_CONNECTED"
	case errors.Is(err, ErrInvalidToken), errors.Is(err, ErrInviteTokenInvalid), errors.Is(err, ErrInviteTokenExpired), errors.Is(err, ErrInviteTokenSignatureInvalid):
		return ExitPermission, "INVALID_TOKEN"
	default:
		return ExitInternal, "INTERNAL"
	}
}

// ErrClusterNotInitialized indicates the Kubernetes cluster has not been initialized.
// Run 'vibespace init' to initialize.
var ErrClusterNotInitialized = errors.New("cluster not initialized")

// ErrClusterNotRunning indicates the Kubernetes cluster is not running.
var ErrClusterNotRunning = errors.New("cluster not running")

// ErrClusterUnreachable indicates the Kubernetes cluster cannot be contacted.
var ErrClusterUnreachable = errors.New("cluster unreachable")

// ErrVibespaceNotFound indicates the requested vibespace does not exist.
var ErrVibespaceNotFound = errors.New("vibespace not found")

// ErrVibespaceNotRunning indicates the vibespace exists but is not running.
var ErrVibespaceNotRunning = errors.New("vibespace not running")

// ErrVibespaceExists indicates a vibespace with the given name already exists.
var ErrVibespaceExists = errors.New("vibespace already exists")

// ErrInvalidName indicates the provided name is not valid.
var ErrInvalidName = errors.New("invalid name")

// ErrDeploymentManagerNotInitialized indicates the deployment manager is nil.
var ErrDeploymentManagerNotInitialized = errors.New("vibespace service is not ready — run 'vibespace init' first")

// ErrKubernetesNotAvailable indicates Kubernetes is not installed or accessible.
var ErrKubernetesNotAvailable = errors.New("kubernetes not available")

// ErrDaemonNotRunning indicates the port-forward daemon is not running.
var ErrDaemonNotRunning = errors.New("daemon not running")

// ErrDaemonAlreadyRunning indicates a daemon is already running for the vibespace.
var ErrDaemonAlreadyRunning = errors.New("daemon already running")

// ErrDaemonStartTimeout indicates the daemon failed to start within the timeout period.
var ErrDaemonStartTimeout = errors.New("daemon failed to start within timeout")

// ErrForwardNotFound indicates the requested port-forward does not exist.
var ErrForwardNotFound = errors.New("forward not found")

// ErrAgentNotFound indicates the requested agent does not exist.
var ErrAgentNotFound = errors.New("agent not found")

// ErrUnknownAgent indicates an operation was attempted on an unregistered agent.
var ErrUnknownAgent = errors.New("unrecognized agent type")

// ErrNotConnected indicates the agent connection is not established.
var ErrNotConnected = errors.New("not connected")

// ErrSSHKeyNotFound indicates the SSH private key for agent connections is missing.
var ErrSSHKeyNotFound = errors.New("SSH key not found")

// ErrNoAgents indicates no agents are available for the operation.
var ErrNoAgents = errors.New("no agents available")

// ErrRemoteNotConnected indicates no remote connection is established.
var ErrRemoteNotConnected = errors.New("not connected to remote server")

// ErrWireGuardNotAvailable indicates WireGuard (wg-quick) is not installed.
var ErrWireGuardNotAvailable = errors.New("WireGuard not available")

// ErrRemoteAlreadyConnected indicates a remote connection already exists.
var ErrRemoteAlreadyConnected = errors.New("already connected to remote server")

// ErrInvalidToken indicates the registration token is invalid or expired.
var ErrInvalidToken = errors.New("invalid or expired token")

// ErrInviteTokenInvalid indicates the invite token is invalid.
var ErrInviteTokenInvalid = errors.New("invite token invalid")

// ErrInviteTokenExpired indicates the invite token is expired.
var ErrInviteTokenExpired = errors.New("invite token expired")

// ErrInviteTokenSignatureInvalid indicates the invite token signature is invalid.
var ErrInviteTokenSignatureInvalid = errors.New("invite token signature invalid")

// GetErrorHint returns a helpful hint for common errors.
func GetErrorHint(err error) string {
	switch {
	case errors.Is(err, ErrVibespaceNotFound):
		return "Use 'vibespace list' to see available vibespaces"
	case errors.Is(err, ErrAgentNotFound):
		return "Use 'vibespace <name> agent list' to see available agents"
	case errors.Is(err, ErrClusterNotInitialized):
		return "Run 'vibespace init' to initialize the cluster, or 'vibespace remote connect' to use a remote cluster"
	case errors.Is(err, ErrClusterNotRunning):
		return "Run 'vibespace init' to start the cluster"
	case errors.Is(err, ErrDaemonNotRunning):
		return "The daemon will auto-start on next command"
	case errors.Is(err, ErrForwardNotFound):
		return "Use 'vibespace <name> forward list' to see active forwards"
	case errors.Is(err, ErrNoAgents):
		return "Use 'vibespace <name> agent create' to add an agent"
	case errors.Is(err, ErrRemoteNotConnected):
		return "Use 'vibespace remote connect <token>' to connect to a remote server"
	case errors.Is(err, ErrWireGuardNotAvailable):
		return "WireGuard installation failed. On Linux, install wireguard-tools first: apt install wireguard-tools"
	case errors.Is(err, ErrRemoteAlreadyConnected):
		return "Use 'vibespace remote disconnect' first, then connect to a new server"
	case errors.Is(err, ErrInvalidToken):
		return "Request a new token from the server admin with: vibespace serve --generate-token"
	default:
		return ""
	}
}
