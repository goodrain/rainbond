// Package nodecert defines public certificate expiration metadata.
package nodecert

// RequiredFiles names the four active RKE2 Agent certificates.
var RequiredFiles = []string{"agent/client-kubelet.crt", "agent/serving-kubelet.crt", "agent/client-kube-proxy.crt", "agent/client-rke2-controller.crt"}

// Certificate contains expiration metadata without certificate material.
type Certificate struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}
