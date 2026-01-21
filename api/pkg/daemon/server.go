package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"vibespace/pkg/portforward"
)

// RefreshCallback is called to re-discover pods when deployments scale (may block waiting for pods)
type RefreshCallback func() (map[string]string, error)

// HealthCheckCallback is called to quickly check current pod state (non-blocking)
// Returns map of agent name -> pod name for currently running pods
type HealthCheckCallback func() (map[string]string, error)

// Server is the daemon server that listens on a Unix socket
type Server struct {
	vibespace           string
	sockPath            string
	listener            net.Listener
	manager             *portforward.Manager
	state               *DaemonState
	onRefresh           RefreshCallback
	onHealthCheck       HealthCheckCallback
	healthCheckInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ServerConfig contains configuration for creating a Server
type ServerConfig struct {
	Vibespace           string
	Manager             *portforward.Manager
	State               *DaemonState
	OnRefresh           RefreshCallback
	OnHealthCheck       HealthCheckCallback // Quick non-blocking pod check (falls back to OnRefresh if nil)
	HealthCheckInterval time.Duration       // How often to check for stale pods (default: 30s)
}

// NewServer creates a new daemon server
func NewServer(cfg ServerConfig) (*Server, error) {
	paths, err := GetDaemonPaths(cfg.Vibespace)
	if err != nil {
		return nil, err
	}

	// Ensure daemon directory exists
	if err := EnsureDaemonDir(); err != nil {
		return nil, err
	}

	// Remove existing socket if present
	os.Remove(paths.SockFile)

	// Default health check interval
	healthCheckInterval := cfg.HealthCheckInterval
	if healthCheckInterval == 0 {
		healthCheckInterval = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Use OnHealthCheck if provided, otherwise fall back to OnRefresh
	healthCheckCallback := cfg.OnHealthCheck
	if healthCheckCallback == nil && cfg.OnRefresh != nil {
		// Wrap OnRefresh as HealthCheckCallback (same signature)
		healthCheckCallback = func() (map[string]string, error) {
			return cfg.OnRefresh()
		}
	}

	return &Server{
		vibespace:           cfg.Vibespace,
		sockPath:            paths.SockFile,
		manager:             cfg.Manager,
		state:               cfg.State,
		onRefresh:           cfg.OnRefresh,
		onHealthCheck:       healthCheckCallback,
		healthCheckInterval: healthCheckInterval,
		ctx:                 ctx,
		cancel:              cancel,
	}, nil
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	if err := os.Chmod(s.sockPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	slog.Info("daemon server started", "socket", s.sockPath)

	s.wg.Add(2)
	go s.acceptLoop()
	go s.healthCheckLoop()

	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	slog.Info("stopping daemon server")

	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for accept loop to finish
	s.wg.Wait()

	// Cleanup socket file
	os.Remove(s.sockPath)

	slog.Info("daemon server stopped")
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				slog.Error("accept error", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// healthCheckLoop periodically checks if pods have changed and triggers a refresh
func (s *Server) healthCheckLoop() {
	defer s.wg.Done()

	if s.onHealthCheck == nil {
		slog.Debug("health check disabled: no health check callback configured")
		return
	}

	ticker := time.NewTicker(s.healthCheckInterval)
	defer ticker.Stop()

	slog.Info("health check started", "interval", s.healthCheckInterval)

	for {
		select {
		case <-s.ctx.Done():
			slog.Debug("health check stopped")
			return
		case <-ticker.C:
			s.checkPodHealth()
		}
	}
}

// checkPodHealth checks if any pods have changed and triggers a refresh if needed
func (s *Server) checkPodHealth() {
	// Get current pods from k8s (quick, non-blocking check)
	currentPods, err := s.onHealthCheck()
	if err != nil {
		slog.Debug("health check: failed to get current pods", "error", err)
		return
	}

	// If no pods returned, might be a transient state (pod recreating)
	// Don't take action yet - wait for next health check
	if len(currentPods) == 0 {
		slog.Debug("health check: no running pods found, will retry")
		return
	}

	// Compare against cached pods
	needsRefresh := false

	// Check if any current pods differ from cached
	for agentName, currentPod := range currentPods {
		cachedPod, exists := s.manager.GetAgentPod(agentName)
		if !exists {
			slog.Info("health check: new agent detected", "agent", agentName, "pod", currentPod)
			needsRefresh = true
			break
		}
		if cachedPod != currentPod {
			slog.Info("health check: pod changed", "agent", agentName, "old_pod", cachedPod, "new_pod", currentPod)
			needsRefresh = true
			break
		}
	}

	if !needsRefresh {
		return
	}

	slog.Info("health check: triggering refresh due to pod changes")

	// Update pod mappings
	for agentName, podName := range currentPods {
		s.manager.SetAgentPod(agentName, podName)
		s.state.SetAgentPod(agentName, podName)
	}

	// Restart all forwards with new pod mappings
	if err := s.manager.RestartAll(); err != nil {
		slog.Error("health check: failed to restart forwards", "error", err)
	}

	s.state.Save()
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		slog.Error("read error", "error", err)
		return
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.writeResponse(conn, NewErrorResponse(fmt.Errorf("invalid request: %w", err)))
		return
	}

	slog.Debug("received request",
		"type", req.Type,
		"agent", req.Agent,
		"port", req.Port)

	resp := s.handleRequest(req)
	s.writeResponse(conn, resp)
}

// writeResponse writes a response to the connection
func (s *Server) writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		return
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.Write(append(data, '\n'))
}

// handleRequest dispatches a request to the appropriate handler
func (s *Server) handleRequest(req Request) Response {
	switch req.Type {
	case RequestPing:
		return s.handlePing()
	case RequestStatus:
		return s.handleStatus()
	case RequestListForwards:
		return s.handleListForwards()
	case RequestAddForward:
		return s.handleAddForward(req)
	case RequestRemoveForward:
		return s.handleRemoveForward(req)
	case RequestStartForward:
		return s.handleStartForward(req)
	case RequestStopForward:
		return s.handleStopForward(req)
	case RequestRestartForward:
		return s.handleRestartForward(req)
	case RequestRestartAll:
		return s.handleRestartAll()
	case RequestRefresh:
		return s.handleRefresh()
	case RequestShutdown:
		return s.handleShutdown()
	default:
		return NewErrorResponse(fmt.Errorf("unknown request type: %s", req.Type))
	}
}

// handlePing handles a ping request
func (s *Server) handlePing() Response {
	return NewSuccessResponse(PingResponse{
		Vibespace: s.vibespace,
		Pid:       os.Getpid(),
	})
}

// handleStatus handles a status request
func (s *Server) handleStatus() Response {
	agents := s.buildAgentStatusList()

	var totalPorts, activePorts int
	for _, agent := range agents {
		for _, fwd := range agent.Forwards {
			totalPorts++
			if fwd.Status == string(portforward.StatusActive) {
				activePorts++
			}
		}
	}

	uptime := time.Since(s.state.StartedAt)

	return NewSuccessResponse(StatusResponse{
		Vibespace:   s.vibespace,
		Running:     true,
		StartedAt:   s.state.StartedAt.Format(time.RFC3339),
		Uptime:      uptime.Round(time.Second).String(),
		Agents:      agents,
		TotalPorts:  totalPorts,
		ActivePorts: activePorts,
	})
}

// handleListForwards handles a list_forwards request
func (s *Server) handleListForwards() Response {
	agents := s.buildAgentStatusList()
	return NewSuccessResponse(ListForwardsResponse{
		Agents: agents,
	})
}

// buildAgentStatusList builds the list of agent statuses
func (s *Server) buildAgentStatusList() []AgentStatus {
	allForwards := s.manager.ListAllForwards()
	var agents []AgentStatus

	for agentName, forwards := range allForwards {
		podName, _ := s.manager.GetAgentPod(agentName)

		var fwdInfos []ForwardInfo
		for _, fwd := range forwards {
			info := ForwardInfo{
				LocalPort:  fwd.LocalPort,
				RemotePort: fwd.RemotePort,
				Status:     string(fwd.Status),
				Reconnects: fwd.Reconnects,
			}
			if fwd.Error != nil {
				info.Error = fwd.Error.Error()
			}

			// Get type from state
			if stateFwd := s.state.GetForward(agentName, fwd.RemotePort); stateFwd != nil {
				info.Type = string(stateFwd.Type)
			}

			fwdInfos = append(fwdInfos, info)
		}

		agents = append(agents, AgentStatus{
			Name:     agentName,
			PodName:  podName,
			Forwards: fwdInfos,
		})
	}

	return agents
}

// handleAddForward handles an add_forward request
func (s *Server) handleAddForward(req Request) Response {
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	localPort, err := s.manager.AddForward(req.Agent, req.Port, portforward.TypeManual, req.Local)
	if err != nil {
		return NewErrorResponse(err)
	}

	// Update state
	s.state.AddForward(req.Agent, &ForwardState{
		LocalPort:  localPort,
		RemotePort: req.Port,
		Type:       portforward.TypeManual,
		Status:     portforward.StatusActive,
	})
	s.state.Save()

	return NewSuccessResponse(AddForwardResponse{
		LocalPort:  localPort,
		RemotePort: req.Port,
		Status:     string(portforward.StatusActive),
	})
}

// handleRemoveForward handles a remove_forward request
func (s *Server) handleRemoveForward(req Request) Response {
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	if err := s.manager.RemoveForward(req.Agent, req.Port); err != nil {
		return NewErrorResponse(err)
	}

	// Update state
	s.state.RemoveForward(req.Agent, req.Port)
	s.state.Save()

	return NewSuccessResponse(nil)
}

// handleStartForward handles a start_forward request
func (s *Server) handleStartForward(req Request) Response {
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	if err := s.manager.StartForward(req.Agent, req.Port); err != nil {
		return NewErrorResponse(err)
	}

	// Update state
	s.state.UpdateForwardStatus(req.Agent, req.Port, portforward.StatusActive, "")
	s.state.Save()

	return NewSuccessResponse(nil)
}

// handleStopForward handles a stop_forward request
func (s *Server) handleStopForward(req Request) Response {
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	if err := s.manager.StopForward(req.Agent, req.Port); err != nil {
		return NewErrorResponse(err)
	}

	// Update state
	s.state.UpdateForwardStatus(req.Agent, req.Port, portforward.StatusStopped, "")
	s.state.Save()

	return NewSuccessResponse(nil)
}

// handleRestartForward handles a restart_forward request
func (s *Server) handleRestartForward(req Request) Response {
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	if err := s.manager.RestartForward(req.Agent, req.Port); err != nil {
		return NewErrorResponse(err)
	}

	// Update state
	s.state.UpdateForwardStatus(req.Agent, req.Port, portforward.StatusActive, "")
	s.state.Save()

	return NewSuccessResponse(nil)
}

// handleRestartAll handles a restart_all request
func (s *Server) handleRestartAll() Response {
	if err := s.manager.RestartAll(); err != nil {
		return NewErrorResponse(err)
	}

	return NewSuccessResponse(nil)
}

// handleRefresh handles a refresh request - re-discovers pods and restarts forwards
func (s *Server) handleRefresh() Response {
	if s.onRefresh == nil {
		return NewErrorResponse(fmt.Errorf("refresh not configured"))
	}

	slog.Info("refreshing pods")

	// Re-discover pods
	agents, err := s.onRefresh()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to discover pods: %w", err))
	}

	// Update pod mappings and restart forwards
	for agentName, podName := range agents {
		oldPod, exists := s.manager.GetAgentPod(agentName)
		if !exists || oldPod != podName {
			slog.Info("updating agent pod", "agent", agentName, "old_pod", oldPod, "new_pod", podName)
			s.manager.SetAgentPod(agentName, podName)
			s.state.SetAgentPod(agentName, podName)
		}
	}

	// Restart all forwards with new pod mappings
	if err := s.manager.RestartAll(); err != nil {
		slog.Error("failed to restart forwards after refresh", "error", err)
	}

	s.state.Save()

	return NewSuccessResponse(nil)
}

// handleShutdown handles a shutdown request
func (s *Server) handleShutdown() Response {
	slog.Info("shutdown requested")

	// Stop all forwards
	s.manager.StopAll()

	// Schedule server stop (after sending response)
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
	}()

	return NewSuccessResponse(nil)
}
