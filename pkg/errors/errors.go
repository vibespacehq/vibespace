package errors

import "errors"

// Cluster errors
var (
	ErrClusterNotInitialized = errors.New("cluster not initialized")
	ErrClusterNotRunning     = errors.New("cluster not running")
	ErrClusterUnreachable    = errors.New("cluster unreachable")
)

// Vibespace errors
var (
	ErrVibespaceNotFound   = errors.New("vibespace not found")
	ErrVibespaceNotRunning = errors.New("vibespace not running")
	ErrVibespaceExists     = errors.New("vibespace already exists")
	ErrInvalidName         = errors.New("invalid name")
)

// Deployment/Kubernetes errors
var (
	ErrDeploymentManagerNotInitialized = errors.New("deployment manager not initialized")
	ErrKubernetesNotAvailable          = errors.New("kubernetes not available")
)

// Daemon errors
var (
	ErrDaemonNotRunning     = errors.New("daemon not running")
	ErrDaemonAlreadyRunning = errors.New("daemon already running")
	ErrDaemonStartTimeout   = errors.New("daemon failed to start within timeout")
)

// Port-forward errors
var (
	ErrForwardNotFound = errors.New("forward not found")
	ErrAgentNotFound   = errors.New("agent not found")
	ErrUnknownAgent    = errors.New("unknown agent")
)

// TUI/Agent connection errors
var (
	ErrNotConnected   = errors.New("not connected")
	ErrSSHKeyNotFound = errors.New("SSH key not found")
	ErrNoAgents       = errors.New("no agents available")
)
