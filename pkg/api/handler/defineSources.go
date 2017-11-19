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

package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/api/option"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	"github.com/pquerna/ffjson/ffjson"
)

//SourcesAction  sources action struct
type SourcesAction struct {
	etcdCli *clientv3.Client
}

//CreateSourcesManager get sources manager
func CreateSourcesManager(conf option.Config) (*SourcesAction, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.EtcdEndpoint,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logrus.Errorf("create etcd client v3 error, %v", err)
		return nil, err
	}
	defer cli.Close()
	return &SourcesAction{
		etcdCli: cli,
	}, nil
}

//CreateDefineSources CreateDefineSources
func (s *SourcesAction) CreateDefineSources(
	tenantID string, ss *api_model.SetDefineSourcesStruct) *util.APIHandleError {

	sourceAlias := ss.Body.SourceSpec.Alias
	k := fmt.Sprintf("/sources/define/%s/%s/%s",
		tenantID,
		sourceAlias,
		ss.Body.SourceSpec.SourceBody.EnvName)
	if CheckKeyIfExist(s.etcdCli, k) {
		return util.CreateAPIHandleError(405,
			fmt.Errorf("key %v is exist", ss.Body.SourceSpec.SourceBody.EnvName))
	}
	v, err := ffjson.Marshal(ss.Body.SourceSpec)
	if err != nil {
		logrus.Errorf("mashal etcd value error, %v", err)
		return util.CreateAPIHandleError(500, err)
	}
	_, err = s.etcdCli.Put(context.TODO(), k, string(v))
	if err != nil {
		logrus.Errorf("put k %s into etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, err)
	}
	//TODO: store mysql
	return nil
}

//CheckKeyIfExist CheckKeyIfExist
func CheckKeyIfExist(etcdCli *clientv3.Client, k string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := etcdCli.Get(ctx, k)
	cancel()
	if err != nil {
		logrus.Errorf("get etcd value error, %v", err)
		return false
	}
	if resp.Count != 0 {
		return true
	}
	return false
}
