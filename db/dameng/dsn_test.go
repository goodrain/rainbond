package dameng

import (
	"strings"
	"testing"
)

// capability_id: rainbond.database.dameng-dsn-normalization
func TestNormalizeDSN(t *testing.T) {
	t.Run("passes through native dm URL", func(t *testing.T) {
		const input = "dm://app-user:app-password@db.example.internal:5236/DMDB?logLevel=error"

		got, err := NormalizeDSN(input)
		if err != nil {
			t.Fatalf("normalize native dm URL: %v", err)
		}
		if got != input {
			t.Fatalf("expected native dm URL to pass through, got %q", got)
		}
	})

	t.Run("converts legacy mysql DSN", func(t *testing.T) {
		got, err := NormalizeDSN("app-user:app-password@tcp(db.example.internal:5236)/DMDB")
		if err != nil {
			t.Fatalf("normalize legacy DSN: %v", err)
		}
		const want = "dm://app-user:app-password@db.example.internal:5236/DMDB"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("normalizes legacy schema case for Dameng", func(t *testing.T) {
		got, err := NormalizeDSN("app-user:app-password@tcp(db.example.internal:5236)/region")
		if err != nil {
			t.Fatalf("normalize legacy schema: %v", err)
		}
		const want = "dm://app-user:app-password@db.example.internal:5236/REGION"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("escapes legacy credential characters", func(t *testing.T) {
		got, err := NormalizeDSN("app-user:p@ss:word@tcp(db.example.internal:5236)/DMDB")
		if err != nil {
			t.Fatalf("normalize legacy DSN with escaped credentials: %v", err)
		}
		const want = "dm://app-user:p%40ss%3Aword@db.example.internal:5236/DMDB"
		if got != want {
			t.Fatalf("expected escaped credentials in %q, got %q", want, got)
		}
	})

	t.Run("rejects invalid input without returning it in the error", func(t *testing.T) {
		const input = "invalid-user:do-not-expose@tcp(db.example.internal:not-a-port)/DMDB"

		_, err := NormalizeDSN(input)
		if err == nil {
			t.Fatal("expected invalid DSN to be rejected")
		}
		if strings.Contains(err.Error(), "invalid-user") || strings.Contains(err.Error(), "do-not-expose") {
			t.Fatal("DSN error must not include connection credentials")
		}
	})

	t.Run("rejects invalid native URL without returning it in the error", func(t *testing.T) {
		const input = "dm://native-user:placeholder@db.example.internal:not-a-port/DMDB"

		_, err := NormalizeDSN(input)
		if err == nil {
			t.Fatal("expected invalid native DM URL to be rejected")
		}
		if err.Error() != "invalid Dameng DSN" {
			t.Fatal("expected a generic DM DSN error")
		}
		for _, fragment := range []string{"native-user", "placeholder", "dm://", "db.example.internal"} {
			if strings.Contains(err.Error(), fragment) {
				t.Fatal("native URL error must not include connection details")
			}
		}
	})
}
