package overlay

import "testing"

func TestMonitorSelection(t *testing.T) {
	t.Parallel()
	labels := []monitorLabel{
		{connector: "DP-1", description: "unique-desc"},
		{connector: "DP-2", description: "shared"},
		{connector: "HDMI-1", description: "shared"},
		{connector: "desk", description: "other"},
	}
	tests := []struct {
		name, selector string
		index          int
		resolved       bool
	}{
		{"default", "", -1, true},
		{"connector", "DP-2", 1, true},
		{"connector before description", "desk", 3, true},
		{"unique description", "unique-desc", 0, true},
		{"missing", "absent", -1, false},
		{"duplicate description", "shared", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, resolved := selectMonitor(tt.selector, labels)
			if index != tt.index || resolved != tt.resolved {
				t.Fatalf("selectMonitor(%q) = %d, %t; want %d, %t", tt.selector, index, resolved, tt.index, tt.resolved)
			}
		})
	}
	t.Run("connector beats duplicate descriptions", func(t *testing.T) {
		index, resolved := selectMonitor("DP-2", []monitorLabel{
			{connector: "DP-2", description: "other"},
			{connector: "HDMI-1", description: "DP-2"},
			{connector: "DP-3", description: "DP-2"},
		})
		if index != 0 || !resolved {
			t.Fatalf("connector precedence = %d, %t", index, resolved)
		}
	})
	t.Run("duplicate connectors", func(t *testing.T) {
		_, resolved := selectMonitor("DP-2", []monitorLabel{{connector: "DP-2"}, {connector: "DP-2"}})
		if resolved {
			t.Fatal("ambiguous connector selected")
		}
	})
}
