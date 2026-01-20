package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client is a client for communicating with a daemon via Unix socket
type Client struct {
	vibespace string
	sockPath  string
	timeout   time.Duration
}

// NewClient creates a new daemon client
func NewClient(vibespace string) (*Client, error) {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return nil, err
	}

	return &Client{
		vibespace: vibespace,
		sockPath:  paths.SockFile,
		timeout:   10 * time.Second,
	}, nil
}

// sendRequest sends a request to the daemon and returns the response
func (c *Client) sendRequest(req Request) (Response, error) {
	conn, err := net.DialTimeout("unix", c.sockPath, c.timeout)
	if err != nil {
		return Response{}, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	// Set deadline
	conn.SetDeadline(time.Now().Add(c.timeout))

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := conn.Write(append(data, '\n')); err != nil {
		return Response{}, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return Response{}, fmt.Errorf("failed to read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, fmt.Errorf("failed to parse response: %w", err)
	}

	return resp, nil
}

// Ping checks if the daemon is alive
func (c *Client) Ping() (*PingResponse, error) {
	resp, err := c.sendRequest(Request{Type: RequestPing})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("ping failed: %s", resp.Error)
	}

	var result PingResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ping response: %w", err)
	}

	return &result, nil
}

// Status gets the daemon status
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.sendRequest(Request{Type: RequestStatus})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("status failed: %s", resp.Error)
	}

	var result StatusResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &result, nil
}

// ListForwards lists all forwards
func (c *Client) ListForwards() (*ListForwardsResponse, error) {
	resp, err := c.sendRequest(Request{Type: RequestListForwards})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("list forwards failed: %s", resp.Error)
	}

	var result ListForwardsResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list forwards response: %w", err)
	}

	return &result, nil
}

// AddForward adds a new forward
func (c *Client) AddForward(agent string, remotePort int, localPort int) (*AddForwardResponse, error) {
	resp, err := c.sendRequest(Request{
		Type:  RequestAddForward,
		Agent: agent,
		Port:  remotePort,
		Local: localPort,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("add forward failed: %s", resp.Error)
	}

	var result AddForwardResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse add forward response: %w", err)
	}

	return &result, nil
}

// RemoveForward removes a forward
func (c *Client) RemoveForward(agent string, remotePort int) error {
	resp, err := c.sendRequest(Request{
		Type:  RequestRemoveForward,
		Agent: agent,
		Port:  remotePort,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("remove forward failed: %s", resp.Error)
	}

	return nil
}

// StartForward starts a stopped forward
func (c *Client) StartForward(agent string, remotePort int) error {
	resp, err := c.sendRequest(Request{
		Type:  RequestStartForward,
		Agent: agent,
		Port:  remotePort,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("start forward failed: %s", resp.Error)
	}

	return nil
}

// StopForward stops a running forward
func (c *Client) StopForward(agent string, remotePort int) error {
	resp, err := c.sendRequest(Request{
		Type:  RequestStopForward,
		Agent: agent,
		Port:  remotePort,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("stop forward failed: %s", resp.Error)
	}

	return nil
}

// RestartForward restarts a forward
func (c *Client) RestartForward(agent string, remotePort int) error {
	resp, err := c.sendRequest(Request{
		Type:  RequestRestartForward,
		Agent: agent,
		Port:  remotePort,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("restart forward failed: %s", resp.Error)
	}

	return nil
}

// RestartAll restarts all forwards
func (c *Client) RestartAll() error {
	resp, err := c.sendRequest(Request{Type: RequestRestartAll})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("restart all failed: %s", resp.Error)
	}

	return nil
}

// Shutdown requests daemon shutdown
func (c *Client) Shutdown() error {
	resp, err := c.sendRequest(Request{Type: RequestShutdown})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("shutdown failed: %s", resp.Error)
	}

	return nil
}

// IsRunning checks if the daemon is running
func (c *Client) IsRunning() bool {
	_, err := c.Ping()
	return err == nil
}
