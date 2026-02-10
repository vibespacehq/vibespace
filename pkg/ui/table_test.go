package ui

import (
	"strings"
	"testing"
)

func TestNewTablePlain(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"alpha", "running"},
		{"beta", "stopped"},
	}

	got := NewTable(headers, rows, true)

	if !strings.Contains(got, "NAME\tSTATUS") {
		t.Errorf("plain table should contain tab-separated headers, got:\n%s", got)
	}
	if !strings.Contains(got, "alpha\trunning") {
		t.Errorf("plain table should contain tab-separated rows, got:\n%s", got)
	}
	if !strings.Contains(got, "beta\tstopped") {
		t.Errorf("plain table should contain all rows, got:\n%s", got)
	}
}

func TestPlainTableRows(t *testing.T) {
	rows := [][]string{
		{"one", "two"},
		{"three", "four"},
	}

	got := PlainTableRows(rows)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")

	if len(lines) != 2 {
		t.Errorf("PlainTableRows should return 2 lines, got %d", len(lines))
	}
	if lines[0] != "one\ttwo" {
		t.Errorf("first row = %q, want %q", lines[0], "one\ttwo")
	}
}

func TestPlainTableWithHeader(t *testing.T) {
	headers := []string{"A", "B"}
	rows := [][]string{{"1", "2"}}

	withHeader := PlainTableWithHeader(headers, rows, true)
	if !strings.HasPrefix(withHeader, "A\tB\n") {
		t.Errorf("includeHeader=true should start with headers, got:\n%s", withHeader)
	}

	withoutHeader := PlainTableWithHeader(headers, rows, false)
	if strings.Contains(withoutHeader, "A\tB") {
		t.Errorf("includeHeader=false should not contain headers, got:\n%s", withoutHeader)
	}
	if !strings.Contains(withoutHeader, "1\t2") {
		t.Errorf("includeHeader=false should still contain data rows, got:\n%s", withoutHeader)
	}
}
