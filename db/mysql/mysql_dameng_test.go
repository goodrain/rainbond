package mysql

import "testing"

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
