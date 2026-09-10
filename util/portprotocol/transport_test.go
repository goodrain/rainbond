package portprotocol

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestTransports(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []corev1.Protocol
	}{
		{"http", []corev1.Protocol{corev1.ProtocolTCP}},
		{"udp", []corev1.Protocol{corev1.ProtocolUDP}},
		{"TCP+UDP", []corev1.Protocol{corev1.ProtocolTCP, corev1.ProtocolUDP}},
		{"bogus", nil},
	} {
		if got := Transports(tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.input, got, tc.want)
		}
	}
}
func TestAllows(t *testing.T) {
	for _, tc := range []struct {
		component, route string
		want             bool
	}{
		{"http", "tcp", true}, {"http", "udp", false}, {"tcp", "tcp+udp", false},
		{"tcp+udp", "tcp", true}, {"tcp+udp", "udp", true}, {"tcp+udp", "tcp+udp", true},
		{"udp", "tcp", false}, {"tcp+udp", "bogus", false},
	} {
		if got := Allows(tc.component, tc.route); got != tc.want {
			t.Errorf("%s -> %s: %v", tc.component, tc.route, got)
		}
	}
}
