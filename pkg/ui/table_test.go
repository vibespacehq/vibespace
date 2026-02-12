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

func TestNewTableColumnAligned(t *testing.T) {
	headers := []string{"NAME", "STATUS", "AGENTS"}
	rows := [][]string{
		{"myproject", "running", "2"},
		{"testenv", "stopped", "1"},
	}

	got := NewTable(headers, rows, false)

	// Headers should be present
	stripped := StripAnsi(got)
	if !strings.Contains(stripped, "NAME") {
		t.Errorf("column-aligned table should contain NAME header, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "STATUS") {
		t.Errorf("column-aligned table should contain STATUS header, got:\n%s", stripped)
	}

	// Data rows should be present
	if !strings.Contains(stripped, "myproject") {
		t.Errorf("column-aligned table should contain data row 'myproject', got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "running") {
		t.Errorf("column-aligned table should contain data row 'running', got:\n%s", stripped)
	}
}

func TestTableColumnsAligned(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"short", "running"},
		{"muchlonger", "stopped"},
	}

	got := NewTable(headers, rows, false)
	stripped := StripAnsi(got)
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 rows), got %d", len(lines))
	}

	// Find the column offset of STATUS in each line
	offsets := make([]int, len(lines))
	for i, line := range lines {
		offsets[i] = strings.Index(line, "STATUS")
		if i == 0 {
			continue
		}
		// For data rows, find the second column value
		if rows[i-1][1] != "" {
			offsets[i] = strings.Index(line, rows[i-1][1])
		}
	}

	// All second-column values should start at the same offset
	headerOffset := offsets[0]
	if headerOffset < 0 {
		t.Fatal("could not find STATUS header")
	}
	for i := 1; i < len(offsets); i++ {
		if offsets[i] != headerOffset {
			t.Errorf("line %d: second column at offset %d, want %d\nlines:\n%s", i, offsets[i], headerOffset, stripped)
		}
	}
}

func TestTableWithAnsiColorsInCells(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	// Simulate pre-colored cells
	greenRunning := "\x1b[32mrunning\x1b[0m"
	rows := [][]string{
		{"myproject", greenRunning},
		{"test", "stopped"},
	}

	got := NewTable(headers, rows, false)
	stripped := StripAnsi(got)
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Second column should still be aligned despite ANSI in cells
	headerOffset := strings.Index(lines[0], "STATUS")
	row1Offset := strings.Index(lines[1], "running")
	row2Offset := strings.Index(lines[2], "stopped")

	if headerOffset < 0 || row1Offset < 0 || row2Offset < 0 {
		t.Fatalf("could not find column values in output:\n%s", stripped)
	}

	if row1Offset != headerOffset {
		t.Errorf("ANSI-colored row misaligned: offset %d, want %d", row1Offset, headerOffset)
	}
	if row2Offset != headerOffset {
		t.Errorf("plain row misaligned: offset %d, want %d", row2Offset, headerOffset)
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"bold", "\x1b[1mhello\x1b[0m", "hello"},
		{"color", "\x1b[32mgreen\x1b[0m", "green"},
		{"multiple codes", "\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"nested", "\x1b[1m\x1b[32mhello\x1b[0m\x1b[0m", "hello"},
		{"empty", "", ""},
		{"no escape", "plain text here", "plain text here"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("StripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewTableEmptyRows(t *testing.T) {
	headers := []string{"NAME", "STATUS", "AGENTS"}
	var rows [][]string

	got := NewTable(headers, rows, false)
	stripped := StripAnsi(got)

	// Should have header line only
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("empty table should have 1 line (header only), got %d:\n%s", len(lines), stripped)
	}
	if !strings.Contains(stripped, "NAME") {
		t.Errorf("empty table should still contain headers, got:\n%s", stripped)
	}
}

func TestNewTableNoBoxCharacters(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"alpha", "running"},
		{"beta", "stopped"},
	}

	got := NewTable(headers, rows, false)

	// Check for box-drawing characters (both thin and rounded)
	boxChars := []string{
		"─", "│", "┌", "┐", "└", "┘", "├", "┤", "┬", "┴", "┼",
		"╭", "╮", "╯", "╰", "═", "║", "╔", "╗", "╚", "╝",
	}
	for _, ch := range boxChars {
		if strings.Contains(got, ch) {
			t.Errorf("column-aligned table should not contain box character %q, got:\n%s", ch, got)
		}
	}
}
