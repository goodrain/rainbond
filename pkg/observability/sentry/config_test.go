package sentry

import "testing"

func TestConfigFromEnvStaysDisabledWithoutDSN(t *testing.T) {
	cfg := configFromEnv(func(key string) string {
		return ""
	})

	if cfg.Enabled {
		t.Fatalf("expected sentry to stay disabled without dsn")
	}
}

func TestConfigFromEnvReadsValuesAndClampsSampleRate(t *testing.T) {
	values := map[string]string{
		"RAINBOND_ERROR_REPORTING_DSN":         "https://example.invalid/1",
		"RAINBOND_ERROR_REPORTING_ENVIRONMENT": "production",
		"RAINBOND_ERROR_REPORTING_RELEASE":     "v6.9.1-dev",
		"SENTRY_TRACES_SAMPLE_RATE":            "2",
	}
	cfg := configFromEnv(func(key string) string {
		return values[key]
	})

	if !cfg.Enabled {
		t.Fatalf("expected sentry to be enabled")
	}
	if cfg.DSN != "https://example.invalid/1" {
		t.Fatalf("unexpected dsn %q", cfg.DSN)
	}
	if cfg.Environment != "production" {
		t.Fatalf("unexpected environment %q", cfg.Environment)
	}
	if cfg.Release != "v6.9.1-dev" {
		t.Fatalf("unexpected release %q", cfg.Release)
	}
	if cfg.TracesSampleRate != 1 {
		t.Fatalf("expected clamped trace rate 1, got %v", cfg.TracesSampleRate)
	}
}

func TestConfigFromEnvPrefersRegionDSN(t *testing.T) {
	values := map[string]string{
		"RAINBOND_ERROR_REPORTING_DSN":         "https://shared.example.invalid/1",
		"RAINBOND_ERROR_REPORTING_BACKEND_DSN": "https://backend.example.invalid/2",
		"RAINBOND_ERROR_REPORTING_REGION_DSN":  "https://region.example.invalid/3",
	}
	cfg := configFromEnv(func(key string) string {
		return values[key]
	})

	if !cfg.Enabled {
		t.Fatalf("expected sentry to be enabled")
	}
	if cfg.DSN != "https://region.example.invalid/3" {
		t.Fatalf("unexpected dsn %q", cfg.DSN)
	}
}

func TestConfigFromEnvAllowsScopedRegionDisable(t *testing.T) {
	values := map[string]string{
		"RAINBOND_ERROR_REPORTING_REGION_ENABLED": "false",
		"RAINBOND_ERROR_REPORTING_REGION_DSN":     "https://region.example.invalid/3",
	}
	cfg := configFromEnv(func(key string) string {
		return values[key]
	})

	if cfg.Enabled {
		t.Fatalf("expected region scoped disable to win")
	}
}

func TestConfigFromEnvAllowsGlobalTelemetryDisable(t *testing.T) {
	values := map[string]string{
		"RAINBOND_TELEMETRY_DISABLED":      "true",
		"RAINBOND_ERROR_REPORTING_DSN":     "https://example.invalid/1",
		"RAINBOND_ERROR_REPORTING_RELEASE": "v6.9.1-dev",
	}
	cfg := configFromEnv(func(key string) string {
		return values[key]
	})

	if cfg.Enabled {
		t.Fatalf("expected telemetry disabled switch to win")
	}
}

func TestParseSampleRateDefaultsInvalidValues(t *testing.T) {
	if got := parseSampleRate("bad"); got != 0 {
		t.Fatalf("expected invalid sample rate to default to 0, got %v", got)
	}
	if got := parseSampleRate("-1"); got != 0 {
		t.Fatalf("expected negative sample rate to default to 0, got %v", got)
	}
}
