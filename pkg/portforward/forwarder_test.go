package portforward

import "testing"

func TestNewForwarder(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		Namespace:  "vibespace",
		PodName:    "test-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	if fwd == nil {
		t.Fatal("NewForwarder returned nil")
	}
	if fwd.LocalPort() != 8080 {
		t.Errorf("LocalPort() = %d, want 8080", fwd.LocalPort())
	}
	if fwd.RemotePort() != 3000 {
		t.Errorf("RemotePort() = %d, want 3000", fwd.RemotePort())
	}
	if fwd.PodName() != "test-pod" {
		t.Errorf("PodName() = %q, want %q", fwd.PodName(), "test-pod")
	}
	if fwd.Status() != StatusPending {
		t.Errorf("Status() = %q, want %q", fwd.Status(), StatusPending)
	}
}

func TestForwarderSetStatus(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		PodName:    "test-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	fwd.SetStatus(StatusActive)
	if fwd.Status() != StatusActive {
		t.Errorf("Status() = %q, want %q", fwd.Status(), StatusActive)
	}

	fwd.SetStatus(StatusError)
	if fwd.Status() != StatusError {
		t.Errorf("Status() = %q, want %q", fwd.Status(), StatusError)
	}
}

func TestForwarderSetPodName(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		PodName:    "old-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	fwd.SetPodName("new-pod")
	if fwd.PodName() != "new-pod" {
		t.Errorf("PodName() = %q, want %q", fwd.PodName(), "new-pod")
	}
}

func TestForwarderIncrementReconnects(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		PodName:    "test-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	if fwd.Reconnects() != 0 {
		t.Errorf("initial Reconnects() = %d, want 0", fwd.Reconnects())
	}

	n := fwd.IncrementReconnects()
	if n != 1 {
		t.Errorf("first IncrementReconnects() = %d, want 1", n)
	}
	if fwd.Reconnects() != 1 {
		t.Errorf("Reconnects() = %d, want 1", fwd.Reconnects())
	}

	n = fwd.IncrementReconnects()
	if n != 2 {
		t.Errorf("second IncrementReconnects() = %d, want 2", n)
	}
}

func TestForwarderReset(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		PodName:    "test-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	fwd.SetStatus(StatusError)
	fwd.Reset()

	if fwd.Status() != StatusPending {
		t.Errorf("after Reset, Status() = %q, want %q", fwd.Status(), StatusPending)
	}
	if fwd.LastError() != nil {
		t.Errorf("after Reset, LastError() = %v, want nil", fwd.LastError())
	}
}

func TestForwarderLastError(t *testing.T) {
	fwd := NewForwarder(ForwarderConfig{
		PodName:    "test-pod",
		LocalPort:  8080,
		RemotePort: 3000,
	})

	if fwd.LastError() != nil {
		t.Errorf("initial LastError() = %v, want nil", fwd.LastError())
	}
}
