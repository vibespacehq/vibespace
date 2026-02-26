package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client is a client for communicating with the daemon via Unix socket
type Client struct {
	sockPath string
	timeout  time.Duration
}

// NewClient creates a new client for the daemon
func NewClient() (*Client, error) {
	paths, err := GetDaemonPaths()
	if err != nil {
		return nil, err
	}

	return &Client{
		sockPath: paths.SockFile,
		timeout:  10 * time.Second,
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

// Refresh re-discovers pods for agents (useful when deployments scale)
func (c *Client) Refresh() error {
	resp, err := c.sendRequest(Request{Type: RequestRefresh})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("refresh failed: %s", resp.Error)
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

// DaemonStatus gets the status of the daemon including all vibespaces
func (c *Client) DaemonStatus() (*DaemonStatusResponse, error) {
	resp, err := c.sendRequest(Request{Type: RequestStatus})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("daemon status failed: %s", resp.Error)
	}

	var result DaemonStatusResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse daemon status response: %w", err)
	}

	return &result, nil
}

// ListForwardsForVibespace lists forwards for a specific vibespace
func (c *Client) ListForwardsForVibespace(vibespace string) (*ListForwardsResponse, error) {
	resp, err := c.sendRequest(Request{
		Type:      RequestListForwards,
		Vibespace: vibespace,
	})
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

// AddForwardForVibespace adds a forward for a specific vibespace
func (c *Client) AddForwardForVibespace(vibespace, agent string, remotePort, localPort int, dns bool, dnsName string) (*AddForwardResponse, error) {
	resp, err := c.sendRequest(Request{
		Type:      RequestAddForward,
		Vibespace: vibespace,
		Agent:     agent,
		Port:      remotePort,
		Local:     localPort,
		DNS:       dns,
		DNSName:   dnsName,
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

// UpdateForwardDNS toggles DNS on an existing forward
func (c *Client) UpdateForwardDNS(vibespace, agent string, remotePort int, dnsName string) (*UpdateForwardDNSResponse, error) {
	resp, err := c.sendRequest(Request{
		Type:      RequestUpdateForwardDNS,
		Vibespace: vibespace,
		Agent:     agent,
		Port:      remotePort,
		DNSName:   dnsName,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("update forward DNS failed: %s", resp.Error)
	}

	var result UpdateForwardDNSResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse update forward DNS response: %w", err)
	}

	return &result, nil
}

// RemoveForwardForVibespace removes a forward for a specific vibespace
func (c *Client) RemoveForwardForVibespace(vibespace, agent string, remotePort int) error {
	resp, err := c.sendRequest(Request{
		Type:      RequestRemoveForward,
		Vibespace: vibespace,
		Agent:     agent,
		Port:      remotePort,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("remove forward failed: %s", resp.Error)
	}

	return nil
}
