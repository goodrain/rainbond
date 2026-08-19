package mysql

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.database.dameng-schema-lifecycle
func TestSchemaActionKeepsMySQLMigrationAndMakesDamengServicesVerify(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
		want   schemaAction
	}{
		{name: "mysql", config: config.Config{DBType: "mysql"}, want: schemaMigrate},
		{name: "dameng default", config: config.Config{DBType: "dm"}, want: schemaVerify},
		{name: "dameng migrate", config: config.Config{DBType: "dm", SchemaMode: "migrate"}, want: schemaMigrate},
		{name: "dameng verify", config: config.Config{DBType: "dm", SchemaMode: "verify"}, want: schemaVerify},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{config: tt.config}
			if got := manager.schemaAction(); got != tt.want {
				t.Fatalf("schema action = %q, want %q", got, tt.want)
			}
		})
	}
}

// capability_id: rainbond.database.dameng-schema-lifecycle
func TestVerifyDamengSchemaReportsTheMissingModelTable(t *testing.T) {
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "schema-verify.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	db.LogMode(false)

	manager := &Manager{
		db:     db,
		config: config.Config{DBType: "dm", SchemaMode: "verify"},
		models: []model.Interface{&model.KeyValue{}},
	}
	if err := manager.VerifySchema(); err == nil {
		t.Fatal("expected a missing Dameng schema table to fail verification")
	}

	if err := db.CreateTable(&model.KeyValue{}).Error; err != nil {
		t.Fatalf("create expected table: %v", err)
	}
	if err := manager.VerifySchema(); err != nil {
		t.Fatalf("verify complete schema: %v", err)
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
