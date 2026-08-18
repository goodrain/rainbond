package mysql

import (
	"database/sql"
	"os"
	"strconv"
	"time"
)

func configureConnectionPool(sqlDB *sql.DB) {
	maxOpenConns := 2500
	maxIdleConns := 500
	maxLifeTime := 5
	if os.Getenv("DB_MAX_OPEN_CONNS") != "" {
		openCon, err := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
		if err == nil {
			maxOpenConns = openCon
		}
	}
	if os.Getenv("DB_MAX_IDLE_CONNS") != "" {
		idleCon, err := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
		if err == nil {
			maxIdleConns = idleCon
		}
	}
	if os.Getenv("DB_CONN_MAX_LIFE_TIME") != "" {
		lifeTime, err := strconv.Atoi(os.Getenv("DB_CONN_MAX_LIFE_TIME"))
		if err == nil {
			maxLifeTime = lifeTime
		}
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(maxLifeTime) * time.Minute)
}
