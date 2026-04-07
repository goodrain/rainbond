package filepersistence

import "testing"

// capability_id: rainbond.filepersistence.volcengine-client-init
func TestVolcengineProviderInitIsIdempotent(t *testing.T) {
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey: "ak",
			SecretKey: "sk",
			Region:    "cn-shanghai",
		},
	}

	if err := provider.init(); err != nil {
		t.Fatal(err)
	}
	if provider.client == nil {
		t.Fatal("expected client to be initialized")
	}
	first := provider.client

	if err := provider.init(); err != nil {
		t.Fatal(err)
	}
	if provider.client != first {
		t.Fatal("expected init to reuse existing client")
	}
}
