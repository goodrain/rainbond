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

package job

import (
	"encoding/json"
	"errors"
	"strings"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
)

const (
	//args
	REPO_VERSION  = "repo_version"
	SYSTEM        = "system"
	MIP           = "master_ip"
	INSTALL_TYPE  = "net_status" //local default
	NETWORK_MODE  = "network_mode"
	ZKHOSTS       = "zk_hosts"
	CASSANDRA_IP  = "cassandra_ip"
	DNS           = "dns"
	ZMQ_SUB       = "zmq_sub"
	ZMQ_TO        = "zmq_to"
	STORAGE_MODE  = "storage_mode"
	NFS_HOST      = "nfs_host"
	NFS_ENDPOINT  = "nfs_endpoint"
	NFS_DEST      = "nfs_dest"
	ETCD_ENDPOINT = "etcd_endpoint"
	MYSQL_IP      = "mysql_ip"
	MYSQL_USER    = "mysql_user"
	MYSQL_PWD     = "mysql_pwd"
	K8S_API       = "k8s_api"

	//default values
	DEFAULT_REPO_VERSION = "3.4"
	DEFAULT_ACP_VERSION  = "3.4"
	DEFAULT_NETWORK_MODE = "calico-node"
	DEFAULT_INSTALL_TYPE = "default"
	DEFAULT_STORAGE_MODE = "nfs"

	//build in jobs
	JOB_NFS         = "nfs"
	JOB_NETWORK     = "network"
	JOB_TENGINE     = "tengine"
	JOB_DOCKER      = "docker"
	JOB_SYNC_IMAGES = "sync_images"
	JOB_KUBELET     = "kubelet"
	JOB_PRISM       = "prism"
)

func DealInitConfig(c *model.FirstConfig) error {

	cfgs := []*model.Config{}

	storageMode := new(model.Config)
	storageMode.Value = c.StorageMode
	storageMode.Name = STORAGE_MODE
	cfgs = append(cfgs, storageMode)
	if storageMode.Value == "nas" {
		//nfs,自己指定

		cfg := new(model.Config)
		cfg.Value = c.StorageHost
		cfg.Name = NFS_HOST
		cfgs = append(cfgs, cfg)

		cfgEndPoint := new(model.Config)
		cfg.Value = c.StorageEndPoint
		cfg.Name = NFS_ENDPOINT
		cfgs = append(cfgs, cfgEndPoint)
	} else {
		//nas 用户已经指定

		if c.StorageHost != "" {
			cfg := new(model.Config)
			cfg.Value = c.StorageHost
			cfg.Name = NFS_HOST
			cfgs = append(cfgs, cfg)
		}
		if c.StorageEndPoint != "" {
			cfg := new(model.Config)
			cfg.Value = c.StorageEndPoint
			cfg.Name = NFS_ENDPOINT
			cfgs = append(cfgs, cfg)
		}
	}
	networkMode := new(model.Config)
	networkMode.Value = c.NetworkMode
	networkMode.Name = NETWORK_MODE
	cfgs = append(cfgs, networkMode)
	//			args = []string{system, node, mode, zkHosts, cassandraIp, etcdIp}
	if c.ZKHosts != "" {
		cfg := new(model.Config)
		cfg.Value = c.StorageMode
		cfg.Name = ZKHOSTS
		cfgs = append(cfgs, cfg)
	}
	if c.CassandraIP != "" {
		cfg := new(model.Config)
		cfg.Value = c.StorageMode
		cfg.Name = CASSANDRA_IP
		cfgs = append(cfgs, cfg)
	}
	if c.EtcdIP != "" {
		cfg := new(model.Config)
		cfg.Value = c.EtcdIP
		cfg.Name = ETCD_ENDPOINT
		cfgs = append(cfgs, cfg)
	}
	if c.K8SAPIAddr != "" {
		cfg := new(model.Config)
		cfg.Value = c.K8SAPIAddr
		cfg.Name = K8S_API
		cfgs = append(cfgs, cfg)
	}
	if c.MasterIP != "" {
		cfg := new(model.Config)
		cfg.Value = c.MasterIP
		cfg.Name = MIP
		cfgs = append(cfgs, cfg)
	}
	if c.DNS != "" {
		cfg := new(model.Config)
		cfg.Value = c.DNS
		cfg.Name = DNS
		cfgs = append(cfgs, cfg)
	}
	if c.ZMQSub != "" {
		cfg := new(model.Config)
		cfg.Value = c.ZMQSub
		cfg.Name = ZMQ_SUB
		cfgs = append(cfgs, cfg)
	}
	if c.ZMQTo != "" {
		cfg := new(model.Config)
		cfg.Value = c.ZMQTo
		cfg.Name = ZMQ_TO
		cfgs = append(cfgs, cfg)
	}
	return saveConfig(cfgs)
}
func saveConfig(configs []*model.Config) error {
	for _, c := range configs {
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		_, err = store.DefalutClient.Put(conf.Config.ConfigStorage+c.Name, string(b))
		return err
	}
	return nil

}
func GetStartConfig() map[string]interface{} {
	r := make(map[string]interface{})
	r["StorageMode"] = []string{"nfs", "nas"}
	r["NetworkMode"] = []string{"midonet", "calico"}
	return r
}
func GetAllCfgs() ([]*model.Config, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath, clientv3.WithPrefix())
	result := []*model.Config{}

	for _, v := range resp.Kvs {
		c := &model.Config{}
		path := string(v.Key)
		c.Value = string(v.Value)
		ps := strings.Split(path, "/")
		c.Name = ps[len(ps)-1]
		result = append(result, c)
	}
	return result, err
}
func PutConfig(config *model.Config) error {
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = store.DefalutClient.Put(conf.Config.ConfigStorage+config.Name, string(b))
	if err != nil {
		return err
	}
	return nil
}
func GetConfigByName(name string) (*model.Config, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigStorage+name, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	if resp.Count > 0 {
		v := resp.Kvs[0].Value

		c := &model.Config{}
		err = json.Unmarshal(v, c)
		return c, err

	} else {
		return nil, errors.New("no resource")
	}
}
func DelConfigByName(name string) error {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigStorage+name, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	if resp.Count > 0 {
		_, err = store.DefalutClient.Delete(conf.Config.ConfigStorage+name, clientv3.WithPrefix())
		return err
	}
	return nil
}
func GetInstallTypeOrDefault() string {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + INSTALL_TYPE)
	installType := ""
	if err != nil {
		logrus.Warnf("can't get install type from etcd by %s using default %s", conf.Config.ConfigPath+INSTALL_TYPE, DEFAULT_INSTALL_TYPE)
		installType = DEFAULT_INSTALL_TYPE
	} else {
		if resp.Count > 0 {
			installType = string(resp.Kvs[0].Value)
		} else {
			installType = DEFAULT_INSTALL_TYPE
		}
	}
	return installType
}
func getACPVersionOrDefault() string {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + REPO_VERSION)
	installType := ""
	if err != nil {
		logrus.Warnf("can't get install type from etcd by %s using default %s", conf.Config.ConfigPath+REPO_VERSION, DEFAULT_ACP_VERSION)
		installType = DEFAULT_REPO_VERSION
	} else {
		if resp.Count > 0 {
			installType = string(resp.Kvs[0].Value)
		} else {
			installType = DEFAULT_REPO_VERSION
		}

	}
	return installType
}
func GetStorageModeOrDefault() string {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + STORAGE_MODE)
	installType := ""
	if err != nil {
		logrus.Warnf("can't get install type from etcd by %s using default %s", conf.Config.ConfigPath+STORAGE_MODE, DEFAULT_STORAGE_MODE)
		installType = DEFAULT_STORAGE_MODE
	} else {
		if resp.Count > 0 {
			installType = string(resp.Kvs[0].Value)
		} else {
			installType = DEFAULT_STORAGE_MODE
		}
	}
	return installType
}
func GetRepoVersionOrDefault() string {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + REPO_VERSION)
	repoVersion := ""
	if err != nil {
		logrus.Warnf("can't get repo version from etcd by %s using default %s", conf.Config.ConfigPath+INSTALL_TYPE, DEFAULT_INSTALL_TYPE)
		repoVersion = DEFAULT_REPO_VERSION
	} else {
		if resp.Count > 0 {
			repoVersion = string(resp.Kvs[0].Value)
		} else {
			repoVersion = DEFAULT_REPO_VERSION
		}

	}
	return repoVersion
}
func GetK8SIp() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+K8S_API, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get master ip from etcd by %s ", conf.Config.ConfigPath+K8S_API)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetMIp() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+MIP, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get master ip from etcd by %s ", conf.Config.ConfigPath+MIP)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetETCDIp() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+ETCD_ENDPOINT, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get etcd ip from etcd by %s ", conf.Config.ConfigPath+ETCD_ENDPOINT)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetMysqlIp() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+MYSQL_IP, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get mysql ip from etcd by %s ", conf.Config.ConfigPath+MYSQL_IP)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetMysqlUser() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+MYSQL_USER, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get mysql user from etcd by %s ", conf.Config.ConfigPath+MYSQL_USER)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetMySqlPwd() (string, error) {

	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath+MYSQL_PWD, clientv3.WithPrefix())
	mip := ""
	if err != nil {
		logrus.Warnf("can't get mysql pwd from etcd by %s ", conf.Config.ConfigPath+MYSQL_PWD)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetNFSHost() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + NFS_HOST)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs host from etcd by %s ", conf.Config.ConfigPath+NFS_HOST)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetNFSDest() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + NFS_DEST)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs host from etcd by %s ", conf.Config.ConfigPath+NFS_DEST)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetZMQSub() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + ZMQ_SUB)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get zmq_sub from etcd by %s ", conf.Config.ConfigPath+ZMQ_SUB)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}
	}
	return mip, nil
}
func GetZMQTo() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + ZMQ_TO)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get zmq_to from etcd by %s ", conf.Config.ConfigPath+ZMQ_TO)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}
	}
	return mip, nil
}
func GetDNS() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + DNS)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get dns from etcd by %s ", conf.Config.ConfigPath+DNS)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}
	}
	return mip, nil
}
func GetNFSEndPoint() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + NFS_ENDPOINT)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs endpoint from etcd by %s ", conf.Config.ConfigPath+NFS_ENDPOINT)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}
	}
	return mip, nil
}
func GetNetWorkMode() string {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + NETWORK_MODE)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs endpoint from etcd by %s ,using default %s", conf.Config.ConfigPath+NETWORK_MODE, DEFAULT_NETWORK_MODE)

	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return DEFAULT_NETWORK_MODE
		}

	}
	return mip
}
func GetZKHosts() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + ZKHOSTS)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs endpoint from etcd by %s ", conf.Config.ConfigPath+ZKHOSTS)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func GetCASSANDRAIP() (string, error) {
	resp, err := store.DefalutClient.Get(conf.Config.ConfigPath + CASSANDRA_IP)
	mip := ""
	if err != nil {
		logrus.Warnf("can't get nfs endpoint from etcd by %s ", conf.Config.ConfigPath+CASSANDRA_IP)
		return "", err
	} else {
		if resp.Count > 0 {
			mip = string(resp.Kvs[0].Value)
		} else {
			return mip, nil
		}

	}
	return mip, nil
}
func UpdateConfig(name, value string) (string, error) {
	_, err := store.DefalutClient.Put(conf.Config.ConfigPath+name, value)
	if err != nil {
		logrus.Infof("update config %s to %s failed,details %s", name, value, err.Error())
		return "", err
	}
	return value, nil
}
func UpdateMultiConfig(name, subKey, value string) error {
	_, err := store.DefalutClient.Put(conf.Config.ConfigPath+name+"/"+subKey, value)
	if err != nil {
		logrus.Infof("update config %s to %s failed,details %s", name, value, err.Error())
		return err
	}
	return nil
}
