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

package clean

import (
	"context"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"

	"github.com/goodrain/rainbond/builder/sources"
)

// Manager CleanManager
type Manager struct {
	imageClient sources.ImageClient
	ctx         context.Context
	cancel      context.CancelFunc
	config      *rest.Config
	keepCount   uint
	clientset   *kubernetes.Clientset
}

// CreateCleanManager create clean manager
func CreateCleanManager(imageClient sources.ImageClient, config *rest.Config, clientset *kubernetes.Clientset, keepCount uint) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Manager{
		imageClient: imageClient,
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
		keepCount:   keepCount,
		clientset:   clientset,
	}
	return c, nil
}

// Start start clean
func (t *Manager) Start(errchan chan error) error {
	logrus.Info("CleanManager is starting.")
	run := func() {
		err := util.Exec(t.ctx, func() error {
			//保留份数 默认5份
			keepCount := t.keepCount
			// 获取构建成功的 并且大于5个版本的serviceId和具体的版本数
			services, err := db.GetManager().VersionInfoDao().GetServicesAndCount("success", keepCount)
			if err != nil {
				logrus.Error(err)
				return err
			}
			for _, service := range services {
				// service.Count-5： 超过指定数量的版本数,一定是正整数
				versions, err := db.GetManager().VersionInfoDao().SearchExpireVersionInfo(service.ServiceID, service.Count-keepCount)
				if err != nil {
					logrus.Error("SearchExpireVersionInfo error: ", err.Error())
					continue
				}
				for _, v := range versions {
					if v.DeliveredType == "image" {
						//clean rbd-hub images
						imageInfo := sources.ImageNameHandle(v.DeliveredPath)
						if strings.Contains(imageInfo.Host, "goodrain.me") {
							reg, err := registry.NewInsecure(imageInfo.Host, builder.REGISTRYUSER, builder.REGISTRYPASS)
							if err != nil {
								logrus.Error(err)
								continue
							} else {
								err = reg.CleanRepoByTag(imageInfo.Name, imageInfo.Tag)
								if err != nil {
									continue
								}
							}
							// registry garbage-collect
							cmd := []string{"registry", "garbage-collect", "/etc/docker/registry/config.yml"}
							out, b, err := reg.PodExecCmd(t.config, t.clientset, "rbd-hub", cmd)
							if err != nil {
								logrus.Error("rbd-hub exec cmd fail: ", out.String(), b.String(), err.Error())
								continue
							} else {
								logrus.Info("rbd-hub exec cmd success.")
							}
						}
						err := t.imageClient.ImageRemove(v.DeliveredPath)
						if err != nil {
							logrus.Error(err)
							continue
						}
						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Error(err)
							continue
						}
						logrus.Info("Image deletion successful:", v.DeliveredPath)
						continue
					}
					if v.DeliveredType == "slug" {
						filePath := v.DeliveredPath
						if err := os.Remove(filePath); err != nil {
							logrus.Error(err)
							continue
						}
						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Error(err)
							continue
						}
						logrus.Info("file deletion successful:", filePath)
					}
				}
			}

			return nil
		}, 1*time.Hour)
		if err != nil {
			errchan <- err
		}
	}
	go run()
	return nil
}

// Stop stop
func (t *Manager) Stop() error {
	logrus.Info("CleanManager is stoping.")
	t.cancel()
	return nil
}
