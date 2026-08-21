package mysql

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/config"
)

// capability_id: region.database.backend-boundary
func TestManagerDoesNotBranchOnDatabaseNames(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "mysql.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	databaseNames := map[string]struct{}{
		"cockroachdb": {},
		"dm":          {},
		"mysql":       {},
		"postgres":    {},
		"sqlite":      {},
		"sqlite3":     {},
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		if _, found := databaseNames[strings.ToLower(value)]; found {
			position := fileSet.Position(literal.Pos())
			t.Errorf("mysql.go:%d branches on database %q outside the backend boundary", position.Line, value)
		}
		return true
	})
}

// capability_id: region.database.backend-boundary
func TestDatabaseBackendProvidesSchemaDefaults(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		expected schemaAction
	}{
		{name: "mysql", dbType: "mysql", expected: schemaMigrate},
		{name: "dameng", dbType: "dm", expected: schemaVerify},
		{name: "cockroachdb", dbType: "cockroachdb", expected: schemaMigrate},
		{name: "sqlite", dbType: "sqlite", expected: schemaMigrate},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend, err := lookupDatabaseBackend(test.dbType)
			if err != nil {
				t.Fatalf("lookup backend: %v", err)
			}
			if got := backend.defaultSchemaAction(); got != test.expected {
				t.Fatalf("default schema action = %q, want %q", got, test.expected)
			}
		})
	}
}

// capability_id: region.database.backend-boundary
func TestManagerSchemaActionUsesBackendDefaultAndExplicitOverride(t *testing.T) {
	backend, err := lookupDatabaseBackend("dm")
	if err != nil {
		t.Fatalf("lookup backend: %v", err)
	}

	manager := &Manager{config: config.Config{DBType: "dm"}, backend: backend}
	if got := manager.schemaAction(); got != schemaVerify {
		t.Fatalf("default schema action = %q, want %q", got, schemaVerify)
	}

	manager.config.SchemaMode = "migrate"
	if got := manager.schemaAction(); got != schemaMigrate {
		t.Fatalf("explicit schema action = %q, want %q", got, schemaMigrate)
	}
}

// capability_id: region.database.backend-boundary
func TestUnknownDatabaseBackendIsRejected(t *testing.T) {
	if _, err := lookupDatabaseBackend("unknown"); err == nil {
		t.Fatal("expected an unknown database backend to be rejected")
	}
}
