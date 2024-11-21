package node

import (
	"strings"
	"testing"
)

func TestCpuMicroArchitecture(t *testing.T) {
	// Mock data for testing
	mockData := `
- uarch: "Skylake"
  family: "6"
  model: "85"
  stepping: "4"
- uarch: "Willow Cove"
  family: "6"
  model: "(140|141)"
  stepping: "4"
- uarch: Coffee Lake
  family: 6
  model: (142|158)
  stepping: (10|11|12|13)
- uarch: "Broadwell"
  family: "6"
  model: "61"
  stepping: ""
`

	tests := []struct {
		family   string
		model    string
		stepping string
		expected string
	}{
		{"6", "85", "4", "Skylake"},
		{"6", "140", "4", "Willow Cove"},
		{"6", "141", "4", "Willow Cove"},
		{"6", "142", "10", "Coffee Lake"},
		{"6", "158", "13", "Coffee Lake"},
		{"6", "61", "3", "Broadwell"},
		{"6", "99", "1", unknownCPUArch}, // No match case
	}

	for _, test := range tests {
		uarch, err := cpuMicroArchitectureFromModel([]byte(mockData), test.family, test.model, test.stepping)
		if err != nil && test.expected != "unknown" {
			t.Errorf("Unexpected error: %v", err)
		}
		if !strings.Contains(uarch, test.expected) {
			t.Errorf("Expected %s, got %s", test.expected, uarch)
		}
	}
}
