package ansible

import (
	"testing"
)

func TestPreCheckNodeInstall(t *testing.T) {
	tests := []struct {
		name    string
		opt     *NodeInstallOption
		wanterr bool
	}{
		{
			name: "empty node id",
			opt: &NodeInstallOption{
				HostRole:   "host role",
				InternalIP: "192.168.1.1",
				RootPass:   "root pass",
			},
			wanterr: true,
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			if err := preCheckNodeInstall(tc.opt); (err != nil) != tc.wanterr {
				t.Errorf("want error: %v, but got %v", tc.wanterr, err)
			}
		})
	}
}
