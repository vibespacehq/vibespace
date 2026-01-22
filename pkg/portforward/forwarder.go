package portforward

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Forwarder handles a single port-forward connection using client-go
type Forwarder struct {
	config     *rest.Config
	namespace  string
	podName    string
	localPort  int
	remotePort int

	stopChan  chan struct{}
	readyChan chan struct{}
	doneChan  chan struct{}

	status     ForwardStatus
	lastError  error
	reconnects int
	mu         sync.RWMutex

	// Callback when forwarder stops (for reconnection logic)
	onStopped func(f *Forwarder)
}

// ForwarderConfig contains the configuration for creating a Forwarder
type ForwarderConfig struct {
	Config     *rest.Config
	Namespace  string
	PodName    string
	LocalPort  int
	RemotePort int
	OnStopped  func(f *Forwarder)
}

// NewForwarder creates a new Forwarder
func NewForwarder(cfg ForwarderConfig) *Forwarder {
	return &Forwarder{
		config:     cfg.Config,
		namespace:  cfg.Namespace,
		podName:    cfg.PodName,
		localPort:  cfg.LocalPort,
		remotePort: cfg.RemotePort,
		stopChan:   make(chan struct{}),
		readyChan:  make(chan struct{}),
		doneChan:   make(chan struct{}),
		status:     StatusPending,
		onStopped:  cfg.OnStopped,
	}
}

// Start begins the port-forward. This is non-blocking and returns
// once the forward is ready or an error occurs.
func (f *Forwarder) Start() error {
	f.mu.Lock()
	if f.status == StatusActive {
		f.mu.Unlock()
		return nil // Already running
	}
	f.status = StatusPending
	f.mu.Unlock()

	slog.Info("starting port-forward",
		"namespace", f.namespace,
		"pod", f.podName,
		"local_port", f.localPort,
		"remote_port", f.remotePort)

	// Build the URL for the pod's portforward subresource
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", f.namespace, f.podName)
	hostURL, err := url.Parse(f.config.Host)
	if err != nil {
		return fmt.Errorf("failed to parse host URL: %w", err)
	}

	pfURL := &url.URL{
		Scheme: hostURL.Scheme,
		Host:   hostURL.Host,
		Path:   path,
	}

	// Create SPDY transport
	transport, upgrader, err := spdy.RoundTripperFor(f.config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	// Create dialer
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", pfURL)

	// Port mapping
	ports := []string{fmt.Sprintf("%d:%d", f.localPort, f.remotePort)}

	// Create port-forwarder
	pf, err := portforward.New(
		dialer,
		ports,
		f.stopChan,
		f.readyChan,
		io.Discard, // stdout
		io.Discard, // stderr
	)
	if err != nil {
		return fmt.Errorf("failed to create port-forwarder: %w", err)
	}

	// Start forwarding in a goroutine
	errChan := make(chan error, 1)
	go func() {
		defer close(f.doneChan)
		err := pf.ForwardPorts()
		if err != nil {
			errChan <- err
		}
		close(errChan)

		f.mu.Lock()
		if f.status == StatusActive {
			f.status = StatusStopped
		}
		f.mu.Unlock()

		// Notify callback
		if f.onStopped != nil {
			f.onStopped(f)
		}
	}()

	// Wait for ready or error
	select {
	case <-f.readyChan:
		f.mu.Lock()
		f.status = StatusActive
		f.mu.Unlock()

		slog.Info("port-forward ready",
			"namespace", f.namespace,
			"pod", f.podName,
			"local_port", f.localPort,
			"remote_port", f.remotePort)
		return nil

	case err := <-errChan:
		f.mu.Lock()
		f.status = StatusError
		f.lastError = err
		f.mu.Unlock()
		return fmt.Errorf("port-forward failed: %w", err)

	case <-time.After(10 * time.Second):
		f.Stop()
		f.mu.Lock()
		f.status = StatusError
		f.lastError = fmt.Errorf("timeout waiting for port-forward to be ready")
		f.mu.Unlock()
		return f.lastError
	}
}

// Stop stops the port-forward
func (f *Forwarder) Stop() {
	f.mu.Lock()
	if f.status != StatusActive && f.status != StatusPending {
		f.mu.Unlock()
		return
	}
	f.status = StatusStopped
	f.mu.Unlock()

	slog.Info("stopping port-forward",
		"namespace", f.namespace,
		"pod", f.podName,
		"local_port", f.localPort,
		"remote_port", f.remotePort)

	close(f.stopChan)

	// Wait for done with timeout
	select {
	case <-f.doneChan:
	case <-time.After(5 * time.Second):
		slog.Warn("timeout waiting for port-forward to stop",
			"pod", f.podName,
			"local_port", f.localPort)
	}
}

// Status returns the current status
func (f *Forwarder) Status() ForwardStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.status
}

// SetStatus sets the status (used by manager for reconnection)
func (f *Forwarder) SetStatus(status ForwardStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

// LastError returns the last error
func (f *Forwarder) LastError() error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastError
}

// LocalPort returns the local port
func (f *Forwarder) LocalPort() int {
	return f.localPort
}

// RemotePort returns the remote port
func (f *Forwarder) RemotePort() int {
	return f.remotePort
}

// PodName returns the pod name
func (f *Forwarder) PodName() string {
	return f.podName
}

// SetPodName updates the pod name (used when pod is replaced)
func (f *Forwarder) SetPodName(podName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.podName = podName
}

// IncrementReconnects increments the reconnect counter
func (f *Forwarder) IncrementReconnects() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconnects++
	return f.reconnects
}

// Reconnects returns the reconnect count
func (f *Forwarder) Reconnects() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.reconnects
}

// Reset prepares the forwarder for a new connection attempt
func (f *Forwarder) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stopChan = make(chan struct{})
	f.readyChan = make(chan struct{})
	f.doneChan = make(chan struct{})
	f.status = StatusPending
	f.lastError = nil
}
