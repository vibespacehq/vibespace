package cli

import (
	"reflect"
	"testing"
)

func TestExcludedToolsFromAllowed(t *testing.T) {
	tests := []struct {
		name      string
		supported []string
		allowed   []string
		want      []string
	}{
		{
			"all allowed",
			[]string{"Bash", "Read", "Write"},
			[]string{"Bash", "Read", "Write"},
			nil,
		},
		{
			"some excluded",
			[]string{"Bash", "Read", "Write", "Edit"},
			[]string{"Read", "Write"},
			[]string{"Bash", "Edit"},
		},
		{
			"none allowed",
			[]string{"Bash", "Read"},
			nil,
			[]string{"Bash", "Read"},
		},
		{
			"with params",
			[]string{"Bash", "Read", "Write"},
			[]string{"Bash(npm run *)", "Read"},
			[]string{"Write"},
		},
		{
			"empty supported",
			nil,
			[]string{"Bash"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excludedToolsFromAllowed(tt.supported, tt.allowed)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("excludedToolsFromAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
