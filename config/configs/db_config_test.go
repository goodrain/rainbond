package configs

import (
	"testing"

	"github.com/spf13/pflag"
)

// capability_id: rainbond.database.db-type-environment-default
func TestAddDBFlagsUsesDBTypeEnvironmentDefault(t *testing.T) {
	t.Setenv("DB_TYPE", "dm")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config := &DBConfig{}
	AddDBFlags(flags, config)

	if config.DBType != "dm" {
		t.Fatalf("expected DB_TYPE to provide the db-type default, got %q", config.DBType)
	}
}

// capability_id: rainbond.database.db-type-environment-default
func TestAddDBFlagsExplicitDBTypeOverridesEnvironmentDefault(t *testing.T) {
	t.Setenv("DB_TYPE", "dm")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config := &DBConfig{}
	AddDBFlags(flags, config)
	if err := flags.Parse([]string{"--db-type=mysql"}); err != nil {
		t.Fatalf("parse db-type flag: %v", err)
	}

	if config.DBType != "mysql" {
		t.Fatalf("expected explicit db-type flag to win, got %q", config.DBType)
	}
}

// capability_id: rainbond.database.db-type-environment-default
func TestAddDBFlagsDefaultsToMySQLWithoutEnvironmentValue(t *testing.T) {
	t.Setenv("DB_TYPE", "")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config := &DBConfig{}
	AddDBFlags(flags, config)

	if config.DBType != "mysql" {
		t.Fatalf("expected mysql default without DB_TYPE, got %q", config.DBType)
	}
}

// capability_id: region.database.backend-boundary
func TestAddDBFlagsDefersSchemaDefaultToDatabaseBackend(t *testing.T) {
	for _, dbType := range []string{"mysql", "dm", "cockroachdb", "sqlite"} {
		t.Run(dbType, func(t *testing.T) {
			t.Setenv("DB_TYPE", dbType)
			t.Setenv("RBD_DB_SCHEMA_MODE", "")

			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			config := &DBConfig{}
			AddDBFlags(flags, config)

			if config.SchemaMode != "" {
				t.Fatalf("expected the backend to choose the schema default, got %q", config.SchemaMode)
			}
		})
	}
}
