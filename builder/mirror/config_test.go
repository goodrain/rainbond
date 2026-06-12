package mirror

import (
	"testing"
	"time"
)

// capability_id: rainbond.builder.dynamic-mirror-config
func TestLoadConfigDefaults(t *testing.T) {
	cfg := LoadConfig(func(string) string { return "" })

	if !cfg.Enabled {
		t.Fatal("dynamic mirrors should default to enabled")
	}
	assertStringSlice(t, cfg.SourceURLs, []string{defaultSourceURL})
	if cfg.RefreshInterval != 6*time.Hour {
		t.Fatalf("default refresh interval = %v, want 6h", cfg.RefreshInterval)
	}
	if cfg.MaxCount != 3 {
		t.Fatalf("default max count = %d, want 3", cfg.MaxCount)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	env := map[string]string{
		"DYNAMIC_REGISTRY_MIRRORS": "false",
		"MIRROR_SOURCE_URLS":       "https://a.example.com/m.json, https://b.example.com/m.json,",
		"MIRROR_REFRESH_INTERVAL":  "30m",
		"MIRROR_MAX_COUNT":         "5",
	}
	cfg := LoadConfig(func(k string) string { return env[k] })

	if cfg.Enabled {
		t.Fatal("DYNAMIC_REGISTRY_MIRRORS=false should disable")
	}
	assertStringSlice(t, cfg.SourceURLs, []string{"https://a.example.com/m.json", "https://b.example.com/m.json"})
	if cfg.RefreshInterval != 30*time.Minute {
		t.Fatalf("refresh interval = %v, want 30m", cfg.RefreshInterval)
	}
	if cfg.MaxCount != 5 {
		t.Fatalf("max count = %d, want 5", cfg.MaxCount)
	}
}

func TestLoadConfigInvalidValuesFallBack(t *testing.T) {
	env := map[string]string{
		"MIRROR_REFRESH_INTERVAL": "soon",
		"MIRROR_MAX_COUNT":        "-1",
	}
	cfg := LoadConfig(func(k string) string { return env[k] })

	if cfg.RefreshInterval != 6*time.Hour {
		t.Fatalf("invalid interval should fall back to 6h, got %v", cfg.RefreshInterval)
	}
	if cfg.MaxCount != 3 {
		t.Fatalf("invalid max count should fall back to 3, got %d", cfg.MaxCount)
	}
}
