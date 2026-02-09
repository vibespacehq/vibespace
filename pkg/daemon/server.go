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

	vsdns "github.com/yagizdagabak/vibespace/pkg/dns"
	"github.com/yagizdagabak/vibespace/pkg/portforward"
)

const defaultDNSPort = 5553

// Server is the daemon server that manages all vibespaces
type Server struct {
	sockPath   string
	listener   net.Listener
	watcher    *PodWatcher
	reconciler *Reconciler
	state      *DaemonState
	desiredMgr *DesiredStateManager
	manager    *portforward.Manager
	dnsServer  *vsdns.Server

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ServerConfig contains configuration for creating a Server
type ServerConfig struct {
	Watcher    *PodWatcher
	Reconciler *Reconciler
	State      *DaemonState
	DesiredMgr *DesiredStateManager
	Manager    *portforward.Manager
}

// NewServer creates a new daemon server
func NewServer(cfg ServerConfig) (*Server, error) {
	paths, err := GetDaemonPaths()
	if err != nil {
		return nil, err
	}

	// Remove existing socket if present
	os.Remove(paths.SockFile)

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		sockPath:   paths.SockFile,
		watcher:    cfg.Watcher,
		reconciler: cfg.Reconciler,
		state:      cfg.State,
		desiredMgr: cfg.DesiredMgr,
		manager:    cfg.Manager,
		ctx:        ctx,
		cancel:     cancel,
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

	// Start embedded DNS server for *.vibespace.internal
	s.dnsServer = vsdns.NewServer(defaultDNSPort)
	if err := s.dnsServer.Start(); err != nil {
		slog.Warn("failed to start DNS server", "error", err)
	}

	s.wg.Add(2)
	go s.acceptLoop()
	go s.watchLoop()

	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	slog.Info("stopping daemon server")

	s.cancel()

	if s.dnsServer != nil {
		s.dnsServer.Stop()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	if s.watcher != nil {
		s.watcher.Stop()
	}

	// Wait for loops to finish
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

// watchLoop watches for pod changes and triggers reconciliation
func (s *Server) watchLoop() {
	defer s.wg.Done()

	// Start watcher in background
	go func() {
		if err := s.watcher.Start(s.ctx); err != nil {
			slog.Error("watcher error", "error", err)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.watcher.Events():
			slog.Debug("pod event received", "type", event.Type, "vibespace", event.Vibespace, "agent", event.Agent)
			if err := s.reconciler.ReconcileVibespace(s.ctx, event.Vibespace); err != nil {
				slog.Error("reconciliation failed", "vibespace", event.Vibespace, "error", err)
			}
		case <-ticker.C:
			// Periodic full reconciliation
			if err := s.reconciler.Reconcile(s.ctx); err != nil {
				slog.Error("periodic reconciliation failed", "error", err)
			}
		}
	}
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
		"vibespace", req.Vibespace,
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
		return s.handleListForwards(req)
	case RequestAddForward:
		return s.handleAddForward(req)
	case RequestRemoveForward:
		return s.handleRemoveForward(req)
	case RequestRefresh:
		return s.handleRefresh(req)
	case RequestShutdown:
		return s.handleShutdown()
	default:
		return NewErrorResponse(fmt.Errorf("unknown request type: %s", req.Type))
	}
}

// handlePing handles a ping request
func (s *Server) handlePing() Response {
	return NewSuccessResponse(PingResponse{
		Pid: os.Getpid(),
	})
}

// handleStatus handles a status request
func (s *Server) handleStatus() Response {
	uptime := time.Since(s.state.StartedAt)

	vibespaces := make(map[string]*StatusResponse)
	for _, vsName := range s.state.GetAllVibespaces() {
		vsState := s.state.GetVibespace(vsName)
		if vsState == nil {
			continue
		}

		agents := s.buildAgentStatusList(vsName)
		var totalPorts, activePorts int
		for _, agent := range agents {
			for _, fwd := range agent.Forwards {
				totalPorts++
				if fwd.Status == string(portforward.StatusActive) {
					activePorts++
				}
			}
		}

		vibespaces[vsName] = &StatusResponse{
			Vibespace:   vsName,
			Running:     true,
			StartedAt:   s.state.StartedAt.Format(time.RFC3339),
			Uptime:      uptime.Round(time.Second).String(),
			Agents:      agents,
			TotalPorts:  totalPorts,
			ActivePorts: activePorts,
		}
	}

	return NewSuccessResponse(DaemonStatusResponse{
		Running:    true,
		StartedAt:  s.state.StartedAt.Format(time.RFC3339),
		Uptime:     uptime.Round(time.Second).String(),
		Pid:        os.Getpid(),
		Vibespaces: vibespaces,
	})
}

// handleListForwards handles a list_forwards request
func (s *Server) handleListForwards(req Request) Response {
	if req.Vibespace == "" {
		return NewErrorResponse(fmt.Errorf("vibespace name required"))
	}

	agents := s.buildAgentStatusList(req.Vibespace)
	return NewSuccessResponse(ListForwardsResponse{
		Agents: agents,
	})
}

// buildAgentStatusList builds the list of agent statuses for a vibespace
func (s *Server) buildAgentStatusList(vibespace string) []AgentStatus {
	vsState := s.state.GetVibespace(vibespace)
	if vsState == nil {
		return nil
	}

	allForwards := s.manager.ListAllForwards()
	var agents []AgentStatus

	for _, agentName := range vsState.GetAllAgentNames() {
		podName, _ := vsState.GetAgentPod(agentName)

		// Get agent state to look up forward types
		agentState := vsState.GetAgentState(agentName)

		// Use composite key to look up forwards (vibespace/agentName)
		key := vibespace + "/" + agentName

		var fwdInfos []ForwardInfo
		if forwards, ok := allForwards[key]; ok {
			for _, fwd := range forwards {
				// Look up type from daemon state by matching remote port
				fwdType := ""
				if agentState != nil {
					for _, fs := range agentState.Forwards {
						if fs.RemotePort == fwd.RemotePort {
							fwdType = string(fs.Type)
							break
						}
					}
				}

				info := ForwardInfo{
					LocalPort:  fwd.LocalPort,
					RemotePort: fwd.RemotePort,
					Type:       fwdType,
					Status:     string(fwd.Status),
					Reconnects: fwd.Reconnects,
				}
				if fwd.Error != nil {
					info.Error = fwd.Error.Error()
				}
				fwdInfos = append(fwdInfos, info)
			}
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
	if req.Vibespace == "" {
		return NewErrorResponse(fmt.Errorf("vibespace name required"))
	}
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	// Use composite key for manager
	key := req.Vibespace + "/" + req.Agent
	localPort, err := s.manager.AddForward(key, req.Port, portforward.TypeManual, req.Local)
	if err != nil {
		return NewErrorResponse(err)
	}

	// Update desired state (uses simple agent name)
	desired := s.desiredMgr.GetOrCreate(req.Vibespace)
	desired.AddForward(req.Agent, DesiredForward{
		ContainerPort: req.Port,
		LocalPort:     localPort,
	})
	s.desiredMgr.Save(req.Vibespace)

	// Update runtime state (uses simple agent name)
	vsState := s.state.GetOrCreateVibespace(req.Vibespace)
	vsState.AddForward(req.Agent, &ForwardState{
		LocalPort:  localPort,
		RemotePort: req.Port,
		Type:       portforward.TypeManual,
		Status:     portforward.StatusActive,
	})

	// Register DNS record if requested
	var dnsName string
	if req.DNS && s.dnsServer != nil {
		if req.DNSName != "" {
			dnsName = req.DNSName
		} else {
			dnsName = req.Agent + "." + req.Vibespace
		}
		s.dnsServer.AddRecord(dnsName, "127.0.0.1")
	}

	return NewSuccessResponse(AddForwardResponse{
		LocalPort:  localPort,
		RemotePort: req.Port,
		Status:     string(portforward.StatusActive),
		DNSName:    dnsName,
	})
}

// handleRemoveForward handles a remove_forward request
func (s *Server) handleRemoveForward(req Request) Response {
	if req.Vibespace == "" {
		return NewErrorResponse(fmt.Errorf("vibespace name required"))
	}
	if req.Agent == "" {
		return NewErrorResponse(fmt.Errorf("agent name required"))
	}
	if req.Port == 0 {
		return NewErrorResponse(fmt.Errorf("remote port required"))
	}

	// Use composite key for manager
	key := req.Vibespace + "/" + req.Agent
	if err := s.manager.RemoveForward(key, req.Port); err != nil {
		return NewErrorResponse(err)
	}

	// Update desired state
	desired := s.desiredMgr.Get(req.Vibespace)
	if desired != nil {
		desired.RemoveForward(req.Agent, req.Port)
		s.desiredMgr.Save(req.Vibespace)
	}

	// Remove DNS records for the agent (try both formats)
	if s.dnsServer != nil {
		s.dnsServer.RemoveRecord(req.Agent + "." + req.Vibespace)
		if req.DNSName != "" {
			s.dnsServer.RemoveRecord(req.DNSName)
		}
	}

	return NewSuccessResponse(nil)
}

// handleRefresh handles a refresh request - triggers reconciliation
func (s *Server) handleRefresh(req Request) Response {
	slog.Info("refresh requested", "vibespace", req.Vibespace)

	if req.Vibespace != "" {
		// Reconcile specific vibespace
		if err := s.reconciler.ReconcileVibespace(s.ctx, req.Vibespace); err != nil {
			return NewErrorResponse(err)
		}
	} else {
		// Reconcile all
		if err := s.reconciler.Reconcile(s.ctx); err != nil {
			return NewErrorResponse(err)
		}
	}

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
