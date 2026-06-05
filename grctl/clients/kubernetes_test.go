package clients

import (
	"testing"

	"k8s.io/client-go/rest"
)

func TestK8SClientInitClientBuildsRuntimeClient(t *testing.T) {
	originalClient := RainbondKubeClient
	t.Cleanup(func() {
		RainbondKubeClient = originalClient
	})

	config := &rest.Config{
		Host: "https://127.0.0.1",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	if err := K8SClientInitClient(nil, config); err != nil {
		t.Fatalf("expected runtime client initialization to succeed, got error: %v", err)
	}
	if RainbondKubeClient == nil {
		t.Fatal("expected runtime client to be initialized")
	}
}
