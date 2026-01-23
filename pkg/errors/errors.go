// Package errors provides sentinel errors for the vibespace CLI.
// Use [errors.Is] to check for specific error conditions.
package errors

import "errors"

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
var ErrDeploymentManagerNotInitialized = errors.New("deployment manager not initialized")

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
var ErrUnknownAgent = errors.New("unknown agent")

// ErrNotConnected indicates the agent connection is not established.
var ErrNotConnected = errors.New("not connected")

// ErrSSHKeyNotFound indicates the SSH private key for agent connections is missing.
var ErrSSHKeyNotFound = errors.New("SSH key not found")

// ErrNoAgents indicates no agents are available for the operation.
var ErrNoAgents = errors.New("no agents available")
