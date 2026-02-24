package tui

import (
	"testing"

	"github.com/vibespacehq/vibespace/pkg/metrics"
)

func TestComputePercentagesNodes(t *testing.T) {
	tab := &MonitorTab{
		nodes: []metrics.NodeMetrics{
			{
				CPUMillis:              500,
				MemoryBytes:            256 * 1024 * 1024,
				CPUAllocatableMillis:   1000,
				MemoryAllocatableBytes: 1024 * 1024 * 1024,
			},
		},
	}

	cpu, mem := tab.computePercentages()
	if cpu != 50.0 {
		t.Errorf("cpu = %f, want 50.0", cpu)
	}
	expectedMem := 25.0
	if mem != expectedMem {
		t.Errorf("mem = %f, want %f", mem, expectedMem)
	}
}

func TestComputePercentagesZeroAllocatable(t *testing.T) {
	tab := &MonitorTab{
		nodes: []metrics.NodeMetrics{
			{CPUMillis: 100, MemoryBytes: 100},
		},
	}

	cpu, mem := tab.computePercentages()
	if cpu != 0 {
		t.Errorf("cpu = %f, want 0 (zero allocatable)", cpu)
	}
	if mem != 0 {
		t.Errorf("mem = %f, want 0 (zero allocatable)", mem)
	}
}

func TestComputePercentagesFilteredPods(t *testing.T) {
	tab := &MonitorTab{
		filterVS: "my-vs",
		pods: []metrics.PodMetrics{
			{
				VibspaceName:     "my-vs",
				CPUMillis:        200,
				MemoryBytes:      100 * 1024 * 1024,
				CPULimitMillis:   400,
				MemoryLimitBytes: 200 * 1024 * 1024,
			},
			{
				VibspaceName: "other-vs",
				CPUMillis:    999,
				MemoryBytes:  999,
			},
		},
	}

	cpu, mem := tab.computePercentages()
	if cpu != 50.0 {
		t.Errorf("cpu = %f, want 50.0", cpu)
	}
	if mem != 50.0 {
		t.Errorf("mem = %f, want 50.0", mem)
	}
}

func TestChartSize(t *testing.T) {
	tab := &MonitorTab{width: 100}
	w, h := tab.chartSize()
	if w != 50 {
		t.Errorf("w = %d, want 50 (capped)", w)
	}
	if h != 5 {
		t.Errorf("h = %d, want 5", h)
	}

	tab.width = 60
	w, _ = tab.chartSize()
	if w != 30 {
		t.Errorf("w = %d, want 30 (half of 60)", w)
	}

	tab.width = 10
	w, _ = tab.chartSize()
	if w != 20 {
		t.Errorf("w = %d, want 20 (min)", w)
	}
}
