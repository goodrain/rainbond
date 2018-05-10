// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package db

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/goodrain/rainbond/eventlog/conf"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"
)

var initTableSQL = `CREATE TABLE IF NOT EXISTS event_log_message (
	"ID"  SERIAL PRIMARY KEY,
	event_id character varying(40),
	start_time character varying(40),
	message bytea
)`

type cockroachPlugin struct {
	conf conf.DBConf
	log  *logrus.Entry
	url  string
	db   *sql.DB
}

func (m *cockroachPlugin) SaveMessage(mes []*EventLogMessage) error {

	if mes == nil || len(mes) < 1 {
		return nil
	}

	data, err := ffjson.Marshal(mes)
	if err != nil {
		return err
	}
	compressData, err := compress(data)
	if err != nil {
		return err
	}
	time := mes[0].Time
	eventID := mes[0].EventID
	row, err := m.db.Query(`INSERT INTO "event_log_message" ("event_id","start_time","message") VALUES ($1,$2,$3)`, eventID, time, compressData)
	if err != nil {
		return errors.New("cockroach plugin insert message error." + err.Error())
	}
	defer row.Close()
	return nil
}

func (m *cockroachPlugin) Close() error {
	return m.db.Close()
}

func (m *cockroachPlugin) open() error {
	readDB, err := sql.Open("postgres", m.url)
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	readDB.SetMaxIdleConns(m.conf.PoolMaxSize)
	readDB.SetMaxOpenConns(m.conf.PoolSize)
	for {
		err = readDB.Ping()
		if err != nil {
			m.log.Error("Ping test cockroach error.", err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}
	m.db = readDB
	tx, err := m.db.Begin()
	if err != nil {
		return errors.New("cockroach plugin do not support transaction." + err.Error())
	}
	row, err := tx.Query(initTableSQL)
	if err != nil {
		tx.Rollback()
		return errors.New("cockroach plugin init lable table error." + err.Error())
	}
	defer row.Close()
	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return errors.New("cockroach plugin init table error." + err.Error())
	}
	return nil
}
