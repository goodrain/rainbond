package controller

import "testing"

// capability_id: rainbond.api.kubeblocks.adapter-service-namespace
func TestKubeBlocksAdapterBaseURLUsesPluginNamespace(t *testing.T) {
	want := "http://kb-adapter-rbdplugin.rbd-plugins.svc:80"
	if blockMechanicaBaseURL != want {
		t.Fatalf("blockMechanicaBaseURL = %q, want %q", blockMechanicaBaseURL, want)
	}
}
