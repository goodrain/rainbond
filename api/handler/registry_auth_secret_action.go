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
	"fmt"

	"github.com/coreos/etcd/clientv3"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/mq/client"
)

// RegistryAuthSecretAction -
type RegistryAuthSecretAction struct {
	dbmanager db.Manager
	mqclient  client.MQClient
	etcdCli   *clientv3.Client
}

//CreateRegistryAuthSecretManager creates registry auth secret manager
func CreateRegistryAuthSecretManager(dbmanager db.Manager, mqclient client.MQClient, etcdCli *clientv3.Client) *RegistryAuthSecretAction {
	return &RegistryAuthSecretAction{
		dbmanager: dbmanager,
		mqclient:  mqclient,
		etcdCli:   etcdCli,
	}
}

//AddOrUpdateRegistryAuthSecret adds or updates registry auth secret
func (g *RegistryAuthSecretAction) AddOrUpdateRegistryAuthSecret(req *apimodel.AddOrUpdateRegistryAuthSecretStruct) error {
	body := make(map[string]interface{})
	body["action"] = "apply"
	body["tenant_id"] = req.TenantID
	body["secret_id"] = req.SecretID
	body["domain"] = req.Domain
	body["username"] = req.Username
	body["password"] = req.Password

	err := g.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "apply_registry_auth_secret",
		TaskBody: body,
	})
	if err != nil {
		return fmt.Errorf("unexpected error occurred while sending task: %v", err)
	}
	return nil
}

// DeleteRegistryAuthSecret deletes registry auth secret
func (g *RegistryAuthSecretAction) DeleteRegistryAuthSecret(req *apimodel.DeleteRegistryAuthSecretStruct) error {
	body := make(map[string]interface{})
	body["action"] = "delete"
	body["tenant_id"] = req.TenantID
	body["secret_id"] = req.SecretID

	err := g.mqclient.SendBuilderTopic(client.TaskStruct{
		Topic:    client.WorkerTopic,
		TaskType: "apply_registry_auth_secret",
		TaskBody: body,
	})
	if err != nil {
		return fmt.Errorf("unexpected error occurred while sending task: %v", err)
	}
	return nil
}
