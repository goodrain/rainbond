package conversion

import (
	"net"
	"reflect"
	"testing"

	v1 "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-network.stable-interface-identity
func TestVMInterfaceMACIsStableAndScopedToService(t *testing.T) {
	input := []v1.Interface{{Name: "default", Model: "virtio", InterfaceBindingMethod: v1.InterfaceBindingMethod{Bridge: &v1.InterfaceBridge{}}}}
	before := input[0].DeepCopy()
	one := withStableVMInterfaceMACs(input, "service-one")
	again := withStableVMInterfaceMACs(input, "service-one")
	other := withStableVMInterfaceMACs(input, "service-two")
	if one[0].MacAddress == "" || one[0].MacAddress != again[0].MacAddress || one[0].MacAddress == other[0].MacAddress {
		t.Fatal("a VM must keep its NIC identity without sharing it with another service")
	}
	mac, err := net.ParseMAC(one[0].MacAddress)
	if err != nil || len(mac) != 6 || mac[0]&1 != 0 || mac[0]&2 == 0 {
		t.Fatal("the default NIC requires a locally administered unicast Ethernet address")
	}
	if !reflect.DeepEqual(input[0], *before) || one[0].Bridge == nil || one[0].Model != "virtio" {
		t.Fatal("assigning an identity changed the caller or network binding")
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestVMInterfaceMACKeepsExplicitAddressesAndSeparatesInterfaces(t *testing.T) {
	input := []v1.Interface{{Name: "default"}, {Name: "second"}, {Name: "custom", MacAddress: "02:11:22:33:44:55"}, {Name: ""}}
	got := withStableVMInterfaceMACs(input, "service-one")
	if got[0].MacAddress == got[1].MacAddress || got[2].MacAddress != input[2].MacAddress || got[3].MacAddress != "" {
		t.Fatal("the generated address overwrote explicit identity or mixed interfaces")
	}
	if !reflect.DeepEqual(withStableVMInterfaceMACs(input, ""), input) || withStableVMInterfaceMACs(nil, "service-one") != nil {
		t.Fatal("missing identity must not assign one shared fallback address")
	}
}
