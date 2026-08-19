package mysql

import (
	"errors"
	"testing"
)

// capability_id: rainbond.database.dameng-bootstrap-skip
func TestDamengSkipsMySQLBootstrap(t *testing.T) {
	manager := &Manager{}
	manager.config.DBType = "dm"

	if !manager.skipsMySQLBootstrap() {
		t.Fatal("expected Dameng startup to skip MySQL-only bootstrap SQL")
	}
}

// capability_id: rainbond.database.dameng-bootstrap-skip
func TestMySQLKeepsBootstrap(t *testing.T) {
	manager := &Manager{}
	manager.config.DBType = "mysql"

	if manager.skipsMySQLBootstrap() {
		t.Fatal("expected MySQL startup to retain bootstrap SQL")
	}
}

// capability_id: rainbond.database.dameng-open-error-cause
func TestDamengOpenErrorPreservesDriverCause(t *testing.T) {
	cause := errors.New("database connection rejected")

	err := damengOpenError(cause)

	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped driver error, got %v", err)
	}
	if got, want := err.Error(), "unable to open Dameng database: database connection rejected"; got != want {
		t.Fatalf("unexpected error: got %q, want %q", got, want)
	}
}

// capability_id: rainbond.database.dameng-open-error-cause
func TestDamengOpenErrorRedactsConnectionCredentials(t *testing.T) {
	cause := errors.New("failed to connect using dm://SYSDBA:secret-password@db.internal:5237/REGION")

	err := damengOpenError(cause)

	if got, want := err.Error(), "unable to open Dameng database: failed to connect using dm://SYSDBA:<redacted>@db.internal:5237/REGION"; got != want {
		t.Fatalf("unexpected redacted error: got %q, want %q", got, want)
	}
}
