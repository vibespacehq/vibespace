package dns

import "errors"

var (
	// ErrInvalidPort is returned when the port is not in valid range
	ErrInvalidPort = errors.New("port must be between 1 and 65535")

	// ErrEmptyDomain is returned when the domain is empty
	ErrEmptyDomain = errors.New("domain cannot be empty")

	// ErrEmptyTargetIP is returned when the target IP is empty
	ErrEmptyTargetIP = errors.New("target IP cannot be empty")

	// ErrServerNotStarted is returned when attempting operations on a non-started server
	ErrServerNotStarted = errors.New("DNS server not started")

	// ErrServerAlreadyRunning is returned when attempting to start an already running server
	ErrServerAlreadyRunning = errors.New("DNS server already running")
)
