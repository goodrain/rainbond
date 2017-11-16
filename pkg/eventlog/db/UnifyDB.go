package db

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/config"
	"time"
	"github.com/goodrain/rainbond/pkg/eventlog/conf"
)

func CreateDBManager(conf conf.DBConf) error {
	var tryTime time.Duration
	tryTime = 0
	var err error
	for tryTime < 4 {
		tryTime++
		if err = db.CreateManager(config.Config{
			MysqlConnectionInfo: conf.URL,
			DBType:              conf.Type,
		}); err != nil {
			logrus.Errorf("get db manager failed, try time is %v,%s", tryTime, err.Error())
			time.Sleep((5 + tryTime*10) * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return err
	}
	logrus.Debugf("init db manager success")
	return nil
}
