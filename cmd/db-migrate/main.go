// Rainbond database migration binary.
package main

import (
	"fmt"
	"os"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/db/mysql"
)

func main() {
	configs.Default().SetAppName("rbd-db-migrate").SetPublicFlags().Parse().SetLog()
	dbConfig := configs.Default().DBConfig
	manager, err := mysql.CreateManager(config.Config{
		MysqlConnectionInfo: dbConfig.DBConnectionInfo,
		DBType:              dbConfig.DBType,
		SchemaMode:          "migrate",
		ShowSQL:             dbConfig.ShowSQL,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "database schema initialization failed:", err)
		os.Exit(1)
	}
	if err := manager.CloseManager(); err != nil {
		fmt.Fprintln(os.Stderr, "database schema initialization cleanup failed:", err)
		os.Exit(1)
	}
}
