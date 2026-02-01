// Package remote provides functionality for remote mode connections via WireGuard.
package remote

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RemoteState represents the client's remote connection state.
// Stored at ~/.vibespace/remote.json
type RemoteState struct {
	Connected       bool      `json:"connected"`
	ServerHost      string    `json:"server_host,omitempty"`      // Original SSH host (user@hostname)
	ServerEndpoint  string    `json:"server_endpoint,omitempty"`  // WireGuard endpoint (hostname:51820)
	LocalIP         string    `json:"local_ip,omitempty"`         // Client's WireGuard IP (10.100.0.2)
	ServerIP        string    `json:"server_ip,omitempty"`        // Server's WireGuard IP (10.100.0.1)
	ConnectedAt     time.Time `json:"connected_at,omitempty"`
	PublicKey       string    `json:"public_key,omitempty"`        // Client's WireGuard public key
	ServerPublicKey string    `json:"server_public_key,omitempty"` // Server's WireGuard public key
}

// ServerState represents the server's state for managing clients.
// Stored at ~/.vibespace/serve.json
type ServerState struct {
	Running        bool                 `json:"running"`
	ListenPort     int                  `json:"listen_port"`
	ServerIP       string               `json:"server_ip"`        // e.g., "10.100.0.1/24"
	PublicKey      string               `json:"public_key"`
	PrivateKeyPath string               `json:"private_key_path"` // Path to private key file
	Clients        []ClientRegistration `json:"clients"`
	NextClientIP   int                  `json:"next_client_ip"` // Next octet for client IP (starts at 2)
}

// ClientRegistration represents a registered client.
type ClientRegistration struct {
	Name         string    `json:"name"`
	PublicKey    string    `json:"public_key"`
	AssignedIP   string    `json:"assigned_ip"` // e.g., "10.100.0.2/32"
	RegisteredAt time.Time `json:"registered_at"`
}

// InviteToken contains server connection info for clients.
// Encoded as base64 JSON and shared via copy-paste.
type InviteToken struct {
	ServerPublicKey string `json:"k"` // Server's WireGuard public key
	Endpoint        string `json:"e"` // Server's public endpoint (host:port)
	AssignedIP      string `json:"i"` // Pre-allocated client IP (e.g., "10.100.0.2/32")
	ServerIP        string `json:"s"` // Server's WireGuard IP (e.g., "10.100.0.1")
}

// Default paths and values
const (
	RemoteStateFile  = "remote.json"
	ServerStateFile  = "serve.json"
	RemoteKubeconfig = "remote_kubeconfig"

	DefaultWireGuardPort  = 51820
	DefaultManagementPort = 7780 // Private, binds to WireGuard IP only
	DefaultServerIP       = "10.100.0.1"
	DefaultClientIPStart  = 2
)

var (
	stateMu sync.Mutex
)

// getVibespaceHome returns the vibespace home directory.
func getVibespaceHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".vibespace"), nil
}

// LoadRemoteState loads the remote state from disk.
func LoadRemoteState() (*RemoteState, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	vsHome, err := getVibespaceHome()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(vsHome, RemoteStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RemoteState{}, nil
		}
		return nil, fmt.Errorf("failed to read remote state: %w", err)
	}

	var state RemoteState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse remote state: %w", err)
	}

	return &state, nil
}

// Save writes the remote state to disk.
func (s *RemoteState) Save() error {
	stateMu.Lock()
	defer stateMu.Unlock()

	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(vsHome, 0755); err != nil {
		return fmt.Errorf("failed to create vibespace directory: %w", err)
	}

	path := filepath.Join(vsHome, RemoteStateFile)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal remote state: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write remote state: %w", err)
	}

	return nil
}

// IsConnected returns true if the client is connected to a remote server.
// This is a convenience function that loads state and checks the Connected field.
func IsConnected() bool {
	state, err := LoadRemoteState()
	if err != nil {
		return false
	}
	return state.Connected
}

// GetRemoteKubeconfigPath returns the path to the remote kubeconfig file.
func GetRemoteKubeconfigPath() (string, error) {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(vsHome, RemoteKubeconfig), nil
}

// LoadServerState loads the server state from disk.
func LoadServerState() (*ServerState, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	vsHome, err := getVibespaceHome()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(vsHome, ServerStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ServerState{
				ListenPort:   DefaultWireGuardPort,
				ServerIP:     DefaultServerIP + "/24",
				NextClientIP: DefaultClientIPStart,
			}, nil
		}
		return nil, fmt.Errorf("failed to read server state: %w", err)
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse server state: %w", err)
	}

	return &state, nil
}

// Save writes the server state to disk.
func (s *ServerState) Save() error {
	stateMu.Lock()
	defer stateMu.Unlock()

	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(vsHome, 0755); err != nil {
		return fmt.Errorf("failed to create vibespace directory: %w", err)
	}

	path := filepath.Join(vsHome, ServerStateFile)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal server state: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write server state: %w", err)
	}

	return nil
}

// AllocateClientIP allocates the next available client IP.
func (s *ServerState) AllocateClientIP() string {
	ip := fmt.Sprintf("10.100.0.%d/32", s.NextClientIP)
	s.NextClientIP++
	return ip
}

// EncodeInviteToken encodes an invite token to a base64 string.
func EncodeInviteToken(token *InviteToken) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return "vs-" + base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeInviteToken decodes a base64 invite token.
func DecodeInviteToken(encoded string) (*InviteToken, error) {
	// Strip "vs-" prefix if present
	encoded = strings.TrimPrefix(encoded, "vs-")

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	var token InviteToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("invalid token data: %w", err)
	}

	return &token, nil
}

// FindClientByPublicKey finds a client by their public key.
func (s *ServerState) FindClientByPublicKey(pubKey string) *ClientRegistration {
	for i := range s.Clients {
		if s.Clients[i].PublicKey == pubKey {
			return &s.Clients[i]
		}
	}
	return nil
}

// AddClient adds a new client registration.
func (s *ServerState) AddClient(name, pubKey, assignedIP string) {
	s.Clients = append(s.Clients, ClientRegistration{
		Name:         name,
		PublicKey:    pubKey,
		AssignedIP:   assignedIP,
		RegisteredAt: time.Now(),
	})
}

// DetectPublicIP attempts to detect the machine's public IP address.
func DetectPublicIP() (string, error) {
	// Try multiple services in case one is down
	services := []string{
		"https://ifconfig.me",
		"https://api.ipify.org",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(body))
		if ip != "" {
			return ip, nil
		}
	}

	return "", fmt.Errorf("could not detect public IP")
}
