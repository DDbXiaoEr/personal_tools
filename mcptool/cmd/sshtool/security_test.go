package main

import "testing"

func TestIsHighRisk(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"ls -la", false},
		{"cat /etc/hosts", false},
		{"rm -rf /", true},
		{"/usr/bin/rm file", true},
		{"sudo ls", true},
		{"echo $(whoami)", true},
		{"echo `id`", true},
		{"echo ${HOME}", true},
		{"", false},
		{"   ", false},
	}
	for _, tt := range tests {
		if got := isHighRisk(tt.cmd); got != tt.want {
			t.Errorf("isHighRisk(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

func TestHighRiskReason(t *testing.T) {
	if got := highRiskReason("rm -rf /"); got == "command blocked" {
		t.Fatalf("expected specific reason, got %q", got)
	}
	if got := highRiskReason("echo $(id)"); got == "command blocked" {
		t.Fatalf("expected shell pattern reason, got %q", got)
	}
}
