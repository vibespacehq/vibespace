package daemon

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRequestDNSConfiguration(t *testing.T) {
	req := Request{
		Type:      RequestAddForward,
		Vibespace: "test",
		Agent:     "claude-1",
		Port:      80,
		DNS:       true,
		DNSName:   "web.test.internal",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, req) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, req)
	}
}

func TestRequestZeroPorts(t *testing.T) {
	req := Request{
		Type:      RequestAddForward,
		Vibespace: "test",
		Agent:     "claude-1",
		Port:      0,
		Local:     0,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Port != 0 || decoded.Local != 0 {
		t.Errorf("zero ports should roundtrip: Port=%d, Local=%d", decoded.Port, decoded.Local)
	}
}

func TestResponseWithComplexData(t *testing.T) {
	resp := NewSuccessResponse(map[string]interface{}{
		"agents": []map[string]interface{}{
			{"name": "claude-1", "ports": []int{8080, 3000}},
		},
	})

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !decoded.Success {
		t.Error("Success should be true")
	}
	if decoded.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestListForwardsResponseMarshal(t *testing.T) {
	resp := ListForwardsResponse{
		Agents: []AgentStatus{
			{
				Name:    "claude-1",
				PodName: "pod-abc",
				Forwards: []ForwardInfo{
					{LocalPort: 3000, RemotePort: 8080, Type: "tcp", Status: "active"},
					{LocalPort: 3001, RemotePort: 22, Type: "ssh", Status: "active", DNSName: "claude-1.internal"},
				},
			},
			{
				Name: "claude-2",
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ListForwardsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, resp)
	}
}

func TestPingResponseMarshal(t *testing.T) {
	resp := NewSuccessResponse("pong")

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !decoded.Success {
		t.Error("ping response should be successful")
	}
}

func TestAddForwardResponseMarshal(t *testing.T) {
	resp := AddForwardResponse{
		LocalPort:  3000,
		RemotePort: 8080,
		Status:     "active",
		DNSName:    "app.vibespace.internal",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded AddForwardResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, resp)
	}
}

func TestDaemonStatusResponseMarshal(t *testing.T) {
	resp := DaemonStatusResponse{
		Running:   true,
		StartedAt: "2024-01-01T00:00:00Z",
		Uptime:    "1h30m",
		Pid:       12345,
		Vibespaces: map[string]*StatusResponse{
			"project-a": {
				Vibespace:   "project-a",
				Running:     true,
				TotalPorts:  3,
				ActivePorts: 2,
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded DaemonStatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !decoded.Running {
		t.Error("Running should be true")
	}
	if decoded.Pid != 12345 {
		t.Errorf("Pid = %d, want 12345", decoded.Pid)
	}
	if len(decoded.Vibespaces) != 1 {
		t.Fatalf("Vibespaces count = %d, want 1", len(decoded.Vibespaces))
	}
	vs := decoded.Vibespaces["project-a"]
	if vs == nil {
		t.Fatal("project-a not found in Vibespaces")
	}
	if vs.TotalPorts != 3 {
		t.Errorf("TotalPorts = %d, want 3", vs.TotalPorts)
	}
}

func TestPingResponseMarshalRoundtrip(t *testing.T) {
	resp := PingResponse{Pid: 99999}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded PingResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, resp) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, resp)
	}
}

func TestForwardInfoWithError(t *testing.T) {
	info := ForwardInfo{
		LocalPort:  3000,
		RemotePort: 8080,
		Type:       "tcp",
		Status:     "error",
		Error:      "connection refused",
		Reconnects: 3,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ForwardInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, info) {
		t.Errorf("roundtrip mismatch:\n got: %+v\nwant: %+v", decoded, info)
	}
}

func TestNewErrorResponseMessage(t *testing.T) {
	resp := NewErrorResponse(errForTest("port already in use"))

	if resp.Success {
		t.Error("error response should not be successful")
	}
	if resp.Error != "port already in use" {
		t.Errorf("Error = %q, want %q", resp.Error, "port already in use")
	}
	if resp.Data != nil {
		t.Error("error response should have nil Data")
	}
}

// errForTest is a simple error type for testing.
type errForTest string

func (e errForTest) Error() string { return string(e) }

func TestAllRequestTypes(t *testing.T) {
	types := []RequestType{
		RequestListForwards,
		RequestAddForward,
		RequestRemoveForward,
		RequestStatus,
		RequestShutdown,
		RequestPing,
		RequestRefresh,
	}

	for _, rt := range types {
		t.Run(string(rt), func(t *testing.T) {
			req := Request{Type: rt, Vibespace: "test"}
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded Request
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if decoded.Type != rt {
				t.Errorf("Type = %q, want %q", decoded.Type, rt)
			}
		})
	}
}
