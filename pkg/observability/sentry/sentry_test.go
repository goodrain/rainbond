package sentry

import "testing"

func TestNewClientBuildsEnvelopeURLFromDSN(t *testing.T) {
	c, err := newClient(Config{
		Enabled:     true,
		DSN:         "https://public@example.sentry.local/prefix/42",
		Environment: "production",
		Release:     "v6.9.1-dev",
	}, "rbd-api")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if c.envelopeURL != "https://example.sentry.local/prefix/api/42/envelope/" {
		t.Fatalf("unexpected envelope url %q", c.envelopeURL)
	}
	if c.authHeader != "Sentry sentry_version=7, sentry_client=rainbond-go/1.0, sentry_key=public" {
		t.Fatalf("unexpected auth header %q", c.authHeader)
	}
	if c.publicDSN != "https://public@example.sentry.local/prefix/42" {
		t.Fatalf("unexpected public dsn %q", c.publicDSN)
	}
	if c.component != "rbd-api" {
		t.Fatalf("unexpected component %q", c.component)
	}
}

func TestNewClientDropsLegacySecretFromPublicDSN(t *testing.T) {
	c, err := newClient(Config{
		Enabled: true,
		DSN:     "https://public:legacy-secret@example.sentry.local/42",
	}, "rbd-api")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if c.publicDSN != "https://public@example.sentry.local/42" {
		t.Fatalf("unexpected public dsn %q", c.publicDSN)
	}
}

func TestNewClientRejectsInvalidDSN(t *testing.T) {
	if _, err := newClient(Config{Enabled: true, DSN: "https://example.invalid/42"}, "rbd-api"); err == nil {
		t.Fatalf("expected missing public key to fail")
	}
}

func TestSanitizeStringFiltersCredentials(t *testing.T) {
	got := sanitizeString("token=abc Authorization: Bearer secret password=pw")
	want := "token=[Filtered] Authorization: [Filtered] password=[Filtered]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSanitizeExtraFiltersSensitiveValues(t *testing.T) {
	got := sanitizeExtra(map[string]interface{}{
		"token":  "abc",
		"detail": "secret=value app=keep",
	})

	if got["token"] != "[Filtered]" {
		t.Fatalf("expected token to be filtered")
	}
	if got["detail"] != "secret=[Filtered] app=keep" {
		t.Fatalf("unexpected detail %q", got["detail"])
	}
}
