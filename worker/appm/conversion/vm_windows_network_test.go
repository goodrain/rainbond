package conversion

import "testing"

// capability_id: rainbond.vm-network.windows-private-dhcp
func TestWindowsPodNetworkDoesNotDelegateLauncherAddressToGuest(t *testing.T) {
	for _, system := range []string{"Windows 10 Education 22H2", "windows11", "Windows Server 2022"} {
		t.Run(system, func(t *testing.T) {
			cfg, err := buildVMRuntimeConfig(map[string]string{vmOSNameKey: system}, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Networks) != 1 || cfg.Networks[0].Pod == nil || len(cfg.Interfaces) != 1 {
				t.Fatal("Windows must keep the primary Kubernetes pod network")
			}
			iface := cfg.Interfaces[0]
			if iface.Masquerade == nil || iface.Bridge != nil || len(iface.Ports) != 0 {
				t.Fatal("Windows requires a private guest DHCP lease and forwarding for platform-managed service ports")
			}
			first := withStableVMInterfaceMACs(cfg.Interfaces, "migrated-windows")
			second := withStableVMInterfaceMACs(cfg.Interfaces, "migrated-windows")
			if first[0].MacAddress == "" || first[0].MacAddress != second[0].MacAddress {
				t.Fatal("recreating a launcher must not create a new Windows network adapter identity")
			}
		})
	}
}

// capability_id: rainbond.vm-network.windows-private-dhcp
func TestLinuxAndUnidentifiedGuestsKeepTheirNetworkBinding(t *testing.T) {
	for _, system := range []string{"", "Ubuntu 24.04", "Debian 12"} {
		cfg, err := buildVMRuntimeConfig(map[string]string{vmOSNameKey: system}, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].Bridge == nil || cfg.Interfaces[0].Masquerade != nil {
			t.Fatal("Windows networking must not change an unrelated guest's binding")
		}
	}
}
