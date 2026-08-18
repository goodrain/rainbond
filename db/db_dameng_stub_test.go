//go:build !dm

package db

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/dameng"
)

// capability_id: rainbond.database.dameng-driver-image-guard
func TestCreateManagerDamengWithoutImageDriver(t *testing.T) {
	const connectionInfo = "test-user:test-password@tcp(db.example.internal:5236)/REGION"

	err := CreateManager(config.Config{
		DBType:              "dm",
		MysqlConnectionInfo: connectionInfo,
	})
	if err == nil {
		t.Fatal("expected an image without the Dameng driver to reject DB_TYPE=dm")
	}
	if !errors.Is(err, dameng.ErrDriverNotBuilt) {
		t.Fatalf("expected ErrDriverNotBuilt, got %v", err)
	}
	if strings.Contains(err.Error(), connectionInfo) || strings.Contains(err.Error(), "test-password") {
		t.Fatalf("Dameng driver error must not expose connection information: %v", err)
	}
}
