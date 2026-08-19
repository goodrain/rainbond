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

// capability_id: rainbond.database.dameng-schema-lifecycle
func TestAddDBFlagsDefaultsDamengServicesToSchemaVerification(t *testing.T) {
	t.Setenv("DB_TYPE", "dm")
	t.Setenv("RBD_DB_SCHEMA_MODE", "")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config := &DBConfig{}
	AddDBFlags(flags, config)

	if config.SchemaMode != "verify" {
		t.Fatalf("expected Dameng service schema mode to default to verify, got %q", config.SchemaMode)
	}
}
