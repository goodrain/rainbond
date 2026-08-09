// Copyright (C) 2014-2018 Goodrain Co., Ltd.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package mysql

import (
	"testing"
	"time"
)

func TestDatabasePoolConfigDefaults(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFE_TIME", "")

	got := databasePoolConfigFromEnv()

	if got.maxOpenConns != 50 {
		t.Fatalf("maxOpenConns = %d, want 50", got.maxOpenConns)
	}
	if got.maxIdleConns != 10 {
		t.Fatalf("maxIdleConns = %d, want 10", got.maxIdleConns)
	}
	if got.connMaxLifetime != 5*time.Minute {
		t.Fatalf("connMaxLifetime = %s, want 5m", got.connMaxLifetime)
	}
}

func TestDatabasePoolConfigUsesPositiveEnvironmentOverrides(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "80")
	t.Setenv("DB_MAX_IDLE_CONNS", "20")
	t.Setenv("DB_CONN_MAX_LIFE_TIME", "3")

	got := databasePoolConfigFromEnv()

	if got.maxOpenConns != 80 {
		t.Fatalf("maxOpenConns = %d, want 80", got.maxOpenConns)
	}
	if got.maxIdleConns != 20 {
		t.Fatalf("maxIdleConns = %d, want 20", got.maxIdleConns)
	}
	if got.connMaxLifetime != 3*time.Minute {
		t.Fatalf("connMaxLifetime = %s, want 3m", got.connMaxLifetime)
	}
}

func TestDatabasePoolConfigIgnoresInvalidEnvironmentOverrides(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "invalid")
	t.Setenv("DB_MAX_IDLE_CONNS", "0")
	t.Setenv("DB_CONN_MAX_LIFE_TIME", "-1")

	got := databasePoolConfigFromEnv()

	if got.maxOpenConns != 50 {
		t.Fatalf("maxOpenConns = %d, want 50", got.maxOpenConns)
	}
	if got.maxIdleConns != 10 {
		t.Fatalf("maxIdleConns = %d, want 10", got.maxIdleConns)
	}
	if got.connMaxLifetime != 5*time.Minute {
		t.Fatalf("connMaxLifetime = %s, want 5m", got.connMaxLifetime)
	}
}

func TestDatabasePoolConfigCapsIdleConnectionsAtOpenConnections(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "5")
	t.Setenv("DB_MAX_IDLE_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFE_TIME", "5")

	got := databasePoolConfigFromEnv()

	if got.maxIdleConns != 5 {
		t.Fatalf("maxIdleConns = %d, want 5", got.maxIdleConns)
	}
}
