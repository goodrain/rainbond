package configs

import (
	"os"

	"github.com/spf13/pflag"
)

type DBConfig struct {
	DBType           string `json:"db_type"`
	DBConnectionInfo string `json:"db_connection_info"`
	SchemaMode       string `json:"schema_mode"`
	ShowSQL          bool   `json:"show_sql"`
}

func AddDBFlags(fs *pflag.FlagSet, dc *DBConfig) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "mysql"
	}
	schemaMode := os.Getenv("RBD_DB_SCHEMA_MODE")
	fs.StringVar(&dc.DBType, "db-type", dbType, "db type mysql or etcd")
	fs.StringVar(&dc.DBConnectionInfo, "mysql", "admin:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.StringVar(&dc.SchemaMode, "db-schema-mode", schemaMode, "database schema action: migrate or verify")
	fs.BoolVar(&dc.ShowSQL, "show-sql", false, "The trigger for showing sql.")
}
