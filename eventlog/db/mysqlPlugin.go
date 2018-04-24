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
	"github.com/goodrain/rainbond/eventlog/conf"
	"io"
	"time"

	"bytes"
	"compress/zlib"

	"github.com/Sirupsen/logrus"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pquerna/ffjson/ffjson"
)

var initLableTableSQL = "create table if not exists `event_log_message` (`ID` int(20) NOT NULL auto_increment,`event_id` VARCHAR(40) NOT NULL,`start_time` VARCHAR(40) NOT NULL,`message` blob ,PRIMARY KEY (`id`));"

type mysqlPlugin struct {
	conf conf.DBConf
	log  *logrus.Entry
	url  string
	db   *sql.DB
}

func (m *mysqlPlugin) SaveMessage(mes []*EventLogMessage) error {

	if mes == nil || len(mes) < 1 {
		return nil
	}
	tx, err := m.db.Begin()
	if err != nil {
		return errors.New("mysql plugin do not support transaction." + err.Error())
	}

	data, err := ffjson.Marshal(mes)
	if err != nil {
		tx.Rollback()
		return err
	}
	compressData, err := compress(data)
	if err != nil {
		tx.Rollback()
		return err
	}
	time := mes[0].Time
	eventID := mes[0].EventID
	row, err := tx.Query("INSERT INTO event_log_message (`event_id`,`start_time`,`message`) VALUES (?,?,?) ", eventID, time, compressData)
	if err != nil {
		tx.Rollback()
		return errors.New("mysql plugin insert message error." + err.Error())
	}
	defer row.Close()
	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return errors.New("mysql plugin insert message commit error." + err.Error())
	}
	return nil
}

func (m *mysqlPlugin) Close() error {
	return m.db.Close()
}

func (m *mysqlPlugin) open() error {
	readDB, err := sql.Open("mysql", m.url)
	if err != nil {
		m.log.Error("Open mysql error.", err.Error())
		return err
	}
	readDB.SetMaxIdleConns(m.conf.PoolMaxSize)
	readDB.SetMaxOpenConns(m.conf.PoolSize)
	for {
		err = readDB.Ping()
		if err != nil {
			m.log.Error("Ping test mysql error.", err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}
	m.db = readDB
	tx, err := m.db.Begin()
	if err != nil {
		return errors.New("mysql plugin do not support transaction." + err.Error())
	}
	row, err := tx.Query(initLableTableSQL)
	if err != nil {
		tx.Rollback()
		return errors.New("mysql plugin init lable table error." + err.Error())
	}
	defer row.Close()
	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return errors.New("mysql plugin init table error." + err.Error())
	}
	return nil
}

func compress(source []byte) ([]byte, error) {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(source)
	w.Close()
	return b.Bytes(), nil
}
func uncompress(source []byte) (re []byte, err error) {
	r, err := zlib.NewReader(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	io.Copy(&buffer, r)
	r.Close()
	return buffer.Bytes(), nil
}
