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

package store

import (
	"github.com/goodrain/rainbond/eventlog/conf"
	"testing"

	"github.com/sirupsen/logrus"
)

var urlData = `
2017-05-19 11:33:34 APPS SumTimeByUrl [{"tenant":"o2o","service":"zzcplus","url":"/active/js/wx_share.js","avgtime":"1.453","sumtime":"1.453","counts":"1"}]
`

func BenchmarkHandleMonitorMessage(b *testing.B) {
	manager, err := NewManager(conf.EventStoreConf{
		HandleDockerLogCoreNumber: 10,
		DB: conf.DBConf{
			Type: "mysql",
			URL:  "root:admin@tcp(127.0.0.1:3306)/event",
		},
	}, logrus.WithField("MODO", "test"))
	if err != nil {
		b.Fatal(err)
	}

	err = manager.Run()
	if err != nil {
		b.Fatal(err)
	}
	//defer manager.Stop()
	for i := 0; i < b.N; i++ {
		manager.MonitorMessageChan() <- [][]byte{[]byte("xxx"), []byte(`2017-05-19 11:33:32 APPS SumTimeBySql [{"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"update weixin_user set subscribe=? where openid = ?","avgtime":"0.10058000000000006","sumtime":"0.20116000000000012","counts":"2"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"insert into lottery_prize (lottery_type,lottery_no,prize_balls,create_time,prize_time) values (?,?,?,?,?)","avgtime":"0.298413","sumtime":"0.298413","counts":"1"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"select miss_type,miss_data from lottery_miss where lottery_type=? and lottery_no=?","avgtime":"0.326492","sumtime":"0.326492","counts":"1"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"insert into weixin_msg (openid,msg_type,content,msg_id,msg_time,create_time) values (?,?,?,?,?,?)","avgtime":"0.8989751","sumtime":"8.989751","counts":"10"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"select * from lottery_prize where lottery_type = ? and lottery_no = ? limit ?","avgtime":"0.20503315909090927","sumtime":"9.021459000000007","counts":"44"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"select id from news where sourceid = ? and del=? limit ?","avgtime":"0.26174076219512193","sumtime":"42.925484999999995","counts":"164"}, {"tenant_id":"d9621ccfc0b742829a517a2642ba04b7","service_id":"b6e19107cadb14b53a95442cb9120b8d","sql":"select a.id,a.title,a.public_time from news a inner join news_class b on a.id=b.news_id where b.class_id=? and a.del=? and a.public_time<? order by a.top_tag desc,a.update_time desc limit ?,?","avgtime":"10.365219238095236","sumtime":"217.66960399999996","counts":"21"}]`)}
		manager.MonitorMessageChan() <- [][]byte{[]byte("xxx"), []byte(urlData)}
	}
}

func TestHandleMonitorMessage(t *testing.T) {
	manager, err := NewManager(conf.EventStoreConf{
		HandleDockerLogCoreNumber: 10,
		DB: conf.DBConf{
			Type: "mysql",
			URL:  "root:admin@tcp(127.0.0.1:3306)/event",
		},
	}, logrus.WithField("MODO", "test"))
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	manager.MonitorMessageChan() <- [][]byte{[]byte("xxx"), []byte(urlData)}
}
