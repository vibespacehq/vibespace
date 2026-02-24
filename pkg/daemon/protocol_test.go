package daemon

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	tests := []struct {
		name string
		req  Request
	}{
		{"list forwards", Request{Type: RequestListForwards, Vibespace: "myproject"}},
		{"add forward", Request{Type: RequestAddForward, Vibespace: "myproject", Agent: "claude-1", Port: 8080, Local: 3000}},
		{"remove forward", Request{Type: RequestRemoveForward, Vibespace: "myproject", Agent: "claude-1", Port: 8080}},
		{"status", Request{Type: RequestStatus, Vibespace: "myproject"}},
		{"shutdown", Request{Type: RequestShutdown}},
		{"ping", Request{Type: RequestPing}},
		{"refresh", Request{Type: RequestRefresh, Vibespace: "myproject"}},
		{"with dns", Request{Type: RequestAddForward, Vibespace: "test", Agent: "claude-1", Port: 80, DNS: true, DNSName: "web.test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var decoded Request
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if !reflect.DeepEqual(decoded, tt.req) {
				t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, tt.req)
			}
		})
	}
}

func TestResponseMarshal(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		resp := NewSuccessResponse(map[string]int{"port": 8080})

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var decoded Response
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if !decoded.Success {
			t.Error("Success should be true")
		}
		if decoded.Error != "" {
			t.Errorf("Error should be empty, got %q", decoded.Error)
		}
		if decoded.Data == nil {
			t.Error("Data should not be nil")
		}
	})

	t.Run("error", func(t *testing.T) {
		resp := NewErrorResponse(fmt.Errorf("something failed"))

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var decoded Response
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if decoded.Success {
			t.Error("Success should be false")
		}
		if decoded.Error != "something failed" {
			t.Errorf("Error = %q, want %q", decoded.Error, "something failed")
		}
	})

	t.Run("nil data", func(t *testing.T) {
		resp := NewSuccessResponse(nil)
		if !resp.Success {
			t.Error("Success should be true")
		}
		if resp.Data != nil {
			t.Error("Data should be nil for nil input")
		}
	})
}

func TestStatusResponseMarshal(t *testing.T) {
	status := StatusResponse{
		Vibespace:   "myproject",
		Running:     true,
		StartedAt:   "2024-01-01T00:00:00Z",
		Uptime:      "2h30m",
		TotalPorts:  5,
		ActivePorts: 3,
		Agents: []AgentStatus{
			{
				Name:    "claude-1",
				PodName: "vibespace-abc-pod",
				Forwards: []ForwardInfo{
					{LocalPort: 3000, RemotePort: 8080, Type: "tcp", Status: "active"},
				},
			},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded StatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(decoded, status) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, status)
	}
}
