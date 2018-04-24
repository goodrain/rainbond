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

package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/pquerna/ffjson/ffjson"
)

//SourcesAction  sources action struct
type SourcesAction struct {
	etcdCli *clientv3.Client
}

//CreateSourcesManager get sources manager
func CreateSourcesManager(etcdCli *clientv3.Client) *SourcesAction {
	return &SourcesAction{
		etcdCli: etcdCli,
	}
}

//CreateDefineSources CreateDefineSources
func (s *SourcesAction) CreateDefineSources(
	tenantID string, ss *api_model.SetDefineSourcesStruct) *util.APIHandleError {
	sourceAlias := ss.Body.SourceSpec.Alias
	k := fmt.Sprintf("/resources/define/%s/%s/%s",
		tenantID,
		sourceAlias,
		ss.Body.SourceSpec.SourceBody.EnvName)
	v, err := ffjson.Marshal(ss.Body.SourceSpec)
	if err != nil {
		logrus.Errorf("mashal etcd value error, %v", err)
		return util.CreateAPIHandleError(500, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = s.etcdCli.Put(ctx, k, string(v))
	if err != nil {
		logrus.Errorf("put k %s into etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, err)
	}
	//TODO: store mysql
	return nil
}

//UpdateDefineSources UpdateDefineSources
func (s *SourcesAction) UpdateDefineSources(
	tenantID string, ss *api_model.SetDefineSourcesStruct) *util.APIHandleError {
	sourceAlias := ss.Body.SourceSpec.Alias
	k := fmt.Sprintf("/resources/define/%s/%s/%s",
		tenantID,
		sourceAlias,
		ss.Body.SourceSpec.SourceBody.EnvName)
	v, err := ffjson.Marshal(ss.Body.SourceSpec)
	if err != nil {
		logrus.Errorf("mashal etcd value error, %v", err)
		return util.CreateAPIHandleError(500, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = s.etcdCli.Put(ctx, k, string(v))
	if err != nil {
		logrus.Errorf("put k %s into etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, err)
	}
	//TODO: store mysql
	return nil
}

//DeleteDefineSources DeleteDefineSources
func (s *SourcesAction) DeleteDefineSources(tenantID, sourceAlias, envName string) *util.APIHandleError {
	k := fmt.Sprintf(
		"/resources/define/%s/%s/%s",
		tenantID,
		sourceAlias,
		envName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := s.etcdCli.Delete(ctx, k)
	if err != nil {
		logrus.Errorf("delete k %s from etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, err)
	}
	return nil
}

//GetDefineSources GetDefineSources
func (s *SourcesAction) GetDefineSources(
	tenantID,
	sourceAlias,
	envName string) (*api_model.SourceSpec, *util.APIHandleError) {
	k := fmt.Sprintf(
		"/resources/define/%s/%s/%s",
		tenantID,
		sourceAlias,
		envName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := s.etcdCli.Get(ctx, k)
	if err != nil {
		logrus.Errorf("get etcd k %s error, %v", k, err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	if resp.Count == 0 {
		return nil, util.CreateAPIHandleError(404, fmt.Errorf("k %s is not exist", k))
	}
	v := resp.Kvs[0].Value
	var ss api_model.SourceSpec
	if err := ffjson.Unmarshal(v, &ss); err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	return &ss, nil
}
