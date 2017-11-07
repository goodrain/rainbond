
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

package controller

import (
	"net/http"
	"strings"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/twinj/uuid"

	corenode "github.com/goodrain/rainbond/pkg/node/core/node"

	"github.com/Sirupsen/logrus"
	v3 "github.com/coreos/etcd/clientv3"

	"encoding/json"
	"fmt"

	"github.com/go-chi/chi"

	"github.com/goodrain/rainbond/pkg/node/api/model"
)

func DeleteGroup(w http.ResponseWriter, r *http.Request) {

	groupId := chi.URLParam(r, "id")

	if len(groupId) == 0 {
		outJSONWithCode(w, http.StatusBadRequest, "empty node ground id.")
		return
	}

	_, err := core.DeleteGroupById(groupId)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	gresp, err := store.DefalutClient.Get(conf.Config.Cmd, v3.WithPrefix())
	if err != nil {
		errstr := fmt.Sprintf("failed to fetch jobs from etcd after deleted node group[%s]: %s", groupId, err.Error())
		logrus.Errorf(errstr)
		outJSONWithCode(w, http.StatusInternalServerError, errstr)
		return
	}

	// update rule's node group
	for i := range gresp.Kvs {
		job := core.Job{}
		err = json.Unmarshal(gresp.Kvs[i].Value, &job)
		key := string(gresp.Kvs[i].Key)
		if err != nil {
			logrus.Errorf("failed to unmarshal job[%s]: %s", key, err.Error())
			continue
		}

		update := false
		for j := range job.Rules {
			var ngs []string
			for _, gid := range job.Rules[j].GroupIDs {
				if gid != groupId {
					ngs = append(ngs, gid)
				}
			}
			if len(ngs) != len(job.Rules[j].GroupIDs) {
				job.Rules[j].GroupIDs = ngs
				update = true
			}
		}

		if update {
			v, err := json.Marshal(&job)
			if err != nil {
				logrus.Errorf("failed to marshal job[%s]: %s", key, err.Error())
				continue
			}
			_, err = store.DefalutClient.PutWithModRev(key, string(v), gresp.Kvs[i].ModRevision)
			if err != nil {
				logrus.Errorf("failed to update job[%s]: %s", key, err.Error())
				continue
			}
		}
	}

	outJSONWithCode(w, http.StatusNoContent, nil)
}
func UpdateGroup(w http.ResponseWriter, r *http.Request) {
	g := core.Group{}
	de := json.NewDecoder(r.Body)
	var err error
	if err = de.Decode(&g); err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	var successCode = http.StatusOK
	g.ID = strings.TrimSpace(g.ID)
	if len(g.ID) == 0 {
		successCode = http.StatusCreated
		g.ID = uuid.NewV4().String()
	}

	if err = g.Check(); err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}

	// @TODO modRev
	var modRev int64 = 0
	if _, err = g.Put(modRev); err != nil {
		outJSONWithCode(w, http.StatusBadRequest, err.Error())
		return
	}

	outJSONWithCode(w, successCode, nil)
}
func GetGroupByGroupId(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	g, err := core.GetGroupById(groupID)
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}

	if g == nil {
		outJSONWithCode(w, http.StatusNotFound, nil)
		return
	}
	outJSON(w, g)
}
func GetGroups(w http.ResponseWriter, r *http.Request) {
	list, err := corenode.GetNodeGroups()
	if err != nil {
		outJSONWithCode(w, http.StatusInternalServerError, err.Error())
		return
	}
	outJSON(w, list)
}
func GetNodes(w http.ResponseWriter, r *http.Request) {
	//nodes, err := core.GetNodes()
	//fmt.Println(nodes)
	//if err != nil {
	//	outJSONWithCode(w, http.StatusInternalServerError, err.Error())
	//	return
	//}

	gresp, err := store.DefalutClient.Get(conf.Config.Node, v3.WithPrefix(), v3.WithKeysOnly())
	if err == nil {
		connecedMap := make(map[string]bool, gresp.Count)
		for i := range gresp.Kvs {
			k := string(gresp.Kvs[i].Key)
			index := strings.LastIndexByte(k, '/')
			connecedMap[k[index+1:]] = true //k[index+1:]表示etcd中 node最后一段名字，connecedMap为所有node名字
		}
		outJSON(w, connecedMap)
	} else {
		logrus.Errorf("failed to fetch key[%s] from etcd: %s", conf.Config.Node, err.Error())
	}
}

func GetMasterNodes(w http.ResponseWriter, r *http.Request) {
	gresp, err := store.DefalutClient.Get(conf.Config.Master, v3.WithPrefix(), v3.WithKeysOnly())
	if err == nil {
		var keys []string
		for i := range gresp.Kvs {
			k := string(gresp.Kvs[i].Key)
			keys = append(keys, k)
		}
		outJSON(w, keys)
	} else {
		logrus.Errorf("failed to fetch key[%s] from etcd: %s", conf.Config.Node, err.Error())
	}
}
func difference(slice1 []string, slice2 []string) []string {
	var diff []string

	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				if s1 == s2 {
					found = true
					break
				}
			}
			// String not found. We add it to return slice
			if !found {
				diff = append(diff, s1)
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}

	return diff
}
func GetWorkerNodes(w http.ResponseWriter, r *http.Request) {
	gresp, err := store.DefalutClient.Get(conf.Config.Node, v3.WithPrefix(), v3.WithKeysOnly())
	var keys []string
	if err == nil {
		for i := range gresp.Kvs {
			k := string(gresp.Kvs[i].Key)
			keys = append(keys, k)
		}
	}

	respM, err := store.DefalutClient.Get(conf.Config.Master, v3.WithPrefix(), v3.WithKeysOnly())
	var keysM []string
	if err == nil {
		for i := range respM.Kvs {
			k := string(respM.Kvs[i].Key)
			keysM = append(keysM, k)
		}
	}

	outJSON(w, difference(keys, keysM))
}
func outJSON(w http.ResponseWriter, data interface{}) {
	outJSONWithCode(w, http.StatusOK, data)
}
func outRespSuccess(w http.ResponseWriter, bean interface{}, data []interface{}) {
	outRespDetails(w, 200, "success", "成功", bean, data)
	//m:=model.ResponseBody{}
	//m.Code=200
	//m.Msg="success"
	//m.MsgCN="成功"
	//m.Body.List=data
}
func outRespDetails(w http.ResponseWriter, code int, msg, msgcn string, bean interface{}, data []interface{}) {
	w.Header().Set("Content-Type", "application/json")
	m := model.ResponseBody{}
	m.Code = code
	m.Msg = msg
	m.MsgCN = msgcn
	m.Body.List = data
	m.Body.Bean = bean

	s := ""
	b, err := json.Marshal(m)

	if err != nil {
		s = `{"error":"json.Marshal error"}`
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		s = string(b)
		w.WriteHeader(code)
	}
	fmt.Fprint(w, s)
}
func outJSONWithCode(w http.ResponseWriter, httpCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	s := ""
	b, err := json.Marshal(data)
	fmt.Println(string(b))
	if err != nil {
		s = `{"error":"json.Marshal error"}`
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		s = string(b)
		w.WriteHeader(httpCode)
	}
	fmt.Fprint(w, s)
}
