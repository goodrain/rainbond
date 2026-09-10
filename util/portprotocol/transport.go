// Package portprotocol maps Rainbond application protocols to Kubernetes transports.
package portprotocol

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Transports expands a component protocol without treating a combined mode as a wire protocol.
func Transports(protocol string) []corev1.Protocol {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "http", "https", "grpc", "mysql", "tcp":
		return []corev1.Protocol{corev1.ProtocolTCP}
	case "udp":
		return []corev1.Protocol{corev1.ProtocolUDP}
	case "tcp+udp":
		return []corev1.Protocol{corev1.ProtocolTCP, corev1.ProtocolUDP}
	case "sctp":
		return []corev1.Protocol{corev1.ProtocolSCTP}
	default:
		return nil
	}
}

// Allows reports whether every requested transport is supported by the component.
func Allows(component, route string) bool {
	available, requested := Transports(component), Transports(route)
	if len(requested) == 0 {
		return false
	}
	for _, want := range requested {
		found := false
		for _, have := range available {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Canonical returns the normalized transport mode represented by Kubernetes ports.
func Canonical(protocols []corev1.Protocol) string {
	tcp, udp := false, false
	for _, p := range protocols {
		if p == corev1.ProtocolUDP {
			udp = true
		} else if p == corev1.ProtocolTCP || p == "" {
			tcp = true
		}
	}
	if tcp && udp {
		return "tcp+udp"
	}
	if udp {
		return "udp"
	}
	return "tcp"
}
