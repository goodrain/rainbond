package sentry

import (
	"os"
	"strconv"
	"strings"
)

const defaultTracesSampleRate = 0

// Official release images inject the public Sentry ingest DSN at build time.
// Source builds stay disabled until a DSN is provided.

// Config keeps Sentry startup options derived from environment variables.
type Config struct {
	Enabled          bool
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64
}

func configFromEnv(getenv func(string) string) Config {
	dsn := firstEnv(
		getenv,
		"RAINBOND_ERROR_REPORTING_REGION_DSN",
		"RAINBOND_ERROR_REPORTING_BACKEND_DSN",
		"RAINBOND_ERROR_REPORTING_DSN",
		"SENTRY_REGION_DSN",
		"SENTRY_BACKEND_DSN",
		"SENTRY_DSN",
	)
	enabled := parseEnabled(
		getenv,
		"RAINBOND_ERROR_REPORTING_REGION_ENABLED",
		"RAINBOND_ERROR_REPORTING_BACKEND_ENABLED",
	) && dsn != ""
	return Config{
		Enabled:          enabled,
		DSN:              dsn,
		Environment:      valueOrDefault(firstEnv(getenv, "RAINBOND_ERROR_REPORTING_ENVIRONMENT", "SENTRY_ENVIRONMENT"), "production"),
		Release:          firstEnv(getenv, "RAINBOND_ERROR_REPORTING_RELEASE", "SENTRY_RELEASE", "RELEASE_DESC"),
		TracesSampleRate: parseSampleRate(getenv("SENTRY_TRACES_SAMPLE_RATE")),
	}
}

func currentConfig() Config {
	return configFromEnv(os.Getenv)
}

func parseBool(value string) bool {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func parseEnabled(getenv func(string) string, scopedKeys ...string) bool {
	if parseBool(getenv("RAINBOND_TELEMETRY_DISABLED")) || parseBool(getenv("RAINBOND_ERROR_REPORTING_DISABLED")) {
		return false
	}
	for _, scopedKey := range scopedKeys {
		if value := getenv(scopedKey); value != "" {
			return parseBool(value)
		}
	}
	if value := getenv("RAINBOND_ERROR_REPORTING_ENABLED"); value != "" {
		return parseBool(value)
	}
	if value := getenv("SENTRY_ENABLED"); value != "" {
		return parseBool(value)
	}
	return true
}

func firstEnv(getenv func(string) string, keys ...string) string {
	for _, key := range keys {
		if value := getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func parseSampleRate(value string) float64 {
	if value == "" {
		return defaultTracesSampleRate
	}
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil || rate < 0 {
		return defaultTracesSampleRate
	}
	if rate > 1 {
		return 1
	}
	return rate
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
