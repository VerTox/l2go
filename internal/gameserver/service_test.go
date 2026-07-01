package gameserver

import (
	"testing"
)

func TestBuildServerAddresses(t *testing.T) {
	tests := []struct {
		name         string
		externalHost string
		wantSubnets  []string
		wantHosts    []string
	}{
		{
			name:         "ip literal is advertised for the wildcard subnet",
			externalHost: "203.0.113.50",
			wantSubnets:  []string{"127.0.0.0/8", "0.0.0.0/0"},
			wantHosts:    []string{"127.0.0.1", "203.0.113.50"},
		},
		{
			name:         "localhost ip default",
			externalHost: "127.0.0.1",
			wantSubnets:  []string{"127.0.0.0/8", "0.0.0.0/0"},
			wantHosts:    []string{"127.0.0.1", "127.0.0.1"},
		},
		{
			name:         "hostname passed through unchanged for LoginServer resolution",
			externalHost: "l2.example.com",
			wantSubnets:  []string{"127.0.0.0/8", "0.0.0.0/0"},
			wantHosts:    []string{"127.0.0.1", "l2.example.com"},
		},
		{
			name:         "empty external host falls back to loopback (no <nil>)",
			externalHost: "",
			wantSubnets:  []string{"127.0.0.0/8", "0.0.0.0/0"},
			wantHosts:    []string{"127.0.0.1", "127.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnets, hosts := buildServerAddresses(tt.externalHost)

			if !equalStrings(subnets, tt.wantSubnets) {
				t.Errorf("subnets = %v, want %v", subnets, tt.wantSubnets)
			}
			if !equalStrings(hosts, tt.wantHosts) {
				t.Errorf("hosts = %v, want %v", hosts, tt.wantHosts)
			}
			if len(subnets) != len(hosts) {
				t.Errorf("subnets/hosts length mismatch: %d vs %d (LoginServer pairs them by index)", len(subnets), len(hosts))
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
