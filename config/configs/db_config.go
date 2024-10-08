package configs

import "github.com/spf13/pflag"

type DBConfig struct {
	DBType           string `json:"db_type"`
	DBConnectionInfo string `json:"db_connection_info"`
	ShowSQL          bool   `json:"show_sql"`
}

func AddDBFlags(fs *pflag.FlagSet, dc *DBConfig) {
	fs.StringVar(&dc.DBType, "db-type", "mysql", "db type mysql or etcd")
	fs.StringVar(&dc.DBConnectionInfo, "mysql", "admin:admin@tcp(127.0.0.1:3306)/region", "mysql db connection info")
	fs.BoolVar(&dc.ShowSQL, "show-sql", false, "The trigger for showing sql.")
}
