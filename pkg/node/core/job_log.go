
// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.
 
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

package core

import (
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"encoding/json"
	"time"

	"github.com/Sirupsen/logrus"
	v3 "github.com/coreos/etcd/clientv3"
	"gopkg.in/mgo.v2/bson"
)

const (
	Coll_JobLog       = "job_log"
	BuildIn_JobLog    = "buildIn_log"
	Coll_JobLatestLog = "job_latest_log"
	Coll_Stat         = "stat"
)

// 任务执行记录
type JobLog struct {
	Id        bson.ObjectId `bson:"_id,omitempty" json:"id"`
	JobId     string        `bson:"jobId" json:"jobId"`               // 任务 Id，索引
	JobGroup  string        `bson:"jobGroup" json:"jobGroup"`         // 任务分组，配合 Id 跳转用
	User      string        `bson:"user" json:"user"`                 // 执行此次任务的用户
	Name      string        `bson:"name" json:"name"`                 // 任务名称
	Node      string        `bson:"node" json:"node"`                 // 运行此次任务的节点 ip，索引
	Command   string        `bson:"command" json:"command,omitempty"` // 执行的命令，包括参数
	Output    string        `bson:"output" json:"output,omitempty"`   // 任务输出的所有内容
	Success   bool          `bson:"success" json:"success"`           // 是否执行成功
	BeginTime time.Time     `bson:"beginTime" json:"beginTime"`       // 任务开始执行时间，精确到毫秒，索引
	EndTime   time.Time     `bson:"endTime" json:"endTime"`           // 任务执行完毕时间，精确到毫秒
}

type JobLatestLog struct {
	JobLog   `bson:",inline"`
	RefLogId string `bson:"refLogId,omitempty" json:"refLogId"`
}

func GetJobLogById(id string) (l *JobLog, err error) {
	//err = mgoDB.FindId(Coll_JobLog, id, &l)
	resp, err := store.DefalutClient.Get(conf.Config.JobLog + "/" + Coll_JobLog + "/" + id)
	g := JobLog{}
	if err != nil || resp.Count == 0 {
		return nil, err
	}

	err = json.Unmarshal(resp.Kvs[0].Value, &g)
	return &g, err
}

var selectForJobLogList = bson.M{"command": 0, "output": 0}

func GetJobLogList() ([]*JobLog, int, error) {
	//func GetJobLogList(query bson.M, page, size int, sort string) ( []*JobLog, int,  error) {
	//err = mgoDB.WithC(Coll_JobLog, func(c *mgo.Collection) error {
	//	total, err = c.Find(query).Count()
	//	if err != nil {
	//		return err
	//	}
	//	return c.Find(query).Select(selectForJobLogList).Sort(sort).Skip((page - 1) * size).Limit(size).All(&list)
	//})

	gresp, err := store.DefalutClient.Get(conf.Config.JobLog+"/"+Coll_JobLog, v3.WithPrefix())
	if err != nil {
		return nil, 0, err
	}
	var list []*JobLog
	for i := range gresp.Kvs {
		joblog := JobLog{}
		err = json.Unmarshal(gresp.Kvs[i].Value, &joblog)
		//key := string(gresp.Kvs[i].Key)
		list = append(list, &joblog)
	}
	return list, int(gresp.Count), nil
}

func GetJobLatestLogList(query bson.M, page, size int, sort string) (list []*JobLatestLog, total int, err error) {
	//err = mgoDB.WithC(Coll_JobLatestLog, func(c *mgo.Collection) error {
	//	total, err = c.Find(query).Count()
	//	if err != nil {
	//		return err
	//	}
	//	return c.Find(query).Select(selectForJobLogList).Sort(sort).Skip((page - 1) * size).Limit(size).All(&list)
	//})
	return
}

func GetJobLatestLogListByJobIds(jobIds []string) (m map[string]*JobLatestLog, err error) {
	var list []*JobLatestLog

	//err = mgoDB.WithC(Coll_JobLatestLog, func(c *mgo.Collection) error {
	//	return c.Find(bson.M{"jobId": bson.M{"$in": jobIds}}).Select(selectForJobLogList).Sort("beginTime").All(&list)
	//})
	//if err != nil {
	//	return
	//}

	m = make(map[string]*JobLatestLog, len(list))
	for i := range list {
		m[list[i].JobId] = list[i]
	}
	return
}

func CreateJobLog(j *Job, t time.Time, rs string, success, isBuildIn bool) {
	et := time.Now()
	j.Avg(t, et)

	jl := JobLog{
		Id:    bson.NewObjectId(),
		JobId: j.ID,

		JobGroup: j.Group,
		Name:     j.Name,
		User:     j.User,

		Node: j.runOn,

		Command: j.Command,
		Output:  rs,
		Success: success,

		BeginTime: t,
		EndTime:   et,
	}
	jsoned, err := json.Marshal(jl)
	if err != nil {

	}
	if isBuildIn {
		logrus.Info("creating new build-in job log,key:%s", conf.Config.JobLog+BuildIn_JobLog+"/"+j.ID+"-"+j.runOn)
		store.DefalutClient.Put(conf.Config.JobLog+BuildIn_JobLog+"/"+j.ID+"-"+j.runOn, string(jsoned))
	} else {
		store.DefalutClient.Put(conf.Config.JobLog+Coll_JobLog+"/"+jl.Id.Hex(), string(jsoned))
	}

	latestLog := &JobLatestLog{
		RefLogId: jl.Id.Hex(),
		JobLog:   jl,
	}
	latestLog.Id = ""
	//todo job最新记录
	//latest:=bson.M{"node": jl.Node, "jobId": jl.JobId, "jobGroup": jl.JobGroup}
	//jsonedLatest,err:=json.Marshal(latest)
	//if err != nil {
	//
	//}
	//DefalutClient.Put(conf.Config.JobLog+"/"+Coll_JobLatestLog+"/"+jl.Id.Hex(),string(jsonedLatest))
	//if err := mgoDB.Upsert(Coll_JobLatestLog, bson.M{"node": jl.Node, "jobId": jl.JobId, "jobGroup": jl.JobGroup}, latestLog); err != nil {
	//	log.Errorf(err.Error())
	//}

	var inc = bson.M{"total": 1}
	if jl.Success {
		inc["successed"] = 1
	} else {
		inc["failed"] = 1
	}

	//err := mgoDB.Upsert(Coll_Stat, bson.M{"name": "job-day", "date": time.Now().Format("2006-01-02")}, bson.M{"$inc": inc})
	//if err != nil {
	//	log.Errorf("increase stat.job %s", err.Error())
	//}
	//err = mgoDB.Upsert(Coll_Stat, bson.M{"name": "job"}, bson.M{"$inc": inc})
	//if err != nil {
	//	log.Errorf("increase stat.job %s", err.Error())
	//}
}

type StatExecuted struct {
	Total     int64 `bson:"total" json:"total"`
	Successed int64 `bson:"successed" json:"successed"`
	Failed    int64 `bson:"failed" json:"failed"`
}

func JobLogStat() (s *StatExecuted, err error) {
	//err = mgoDB.FindOne(Coll_Stat, bson.M{"name": "job"}, &s)
	return
}

func JobLogDayStat(day time.Time) (s *StatExecuted, err error) {
	//err = mgoDB.FindOne(Coll_Stat, bson.M{"name": "job-day", "date": day.Format("2006-01-02")}, &s)
	return
}
