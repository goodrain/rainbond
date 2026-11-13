package conversion

import (
	"crypto/sha256"
	"net"
	"strings"

	v1 "kubevirt.io/api/core/v1"
)

func withStableVMInterfaceMACs(interfaces []v1.Interface, serviceID string) []v1.Interface {
	result := append([]v1.Interface(nil), interfaces...)
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return result
	}
	for i := range result {
		if result[i].MacAddress != "" || result[i].Name == "" {
			continue
		}
		// Bridge binding otherwise inherits a new Pod MAC after every VMI
		// recreation, while cloud-init's netplan still matches the first MAC.
		digest := sha256.Sum256([]byte("rainbond.vm.nic.v1\x00" + serviceID + "\x00" + result[i].Name))
		address := net.HardwareAddr(digest[:6])
		address[0] = (address[0] | 2) & 0xfe
		result[i].MacAddress = address.String()
	}
	return result
}
