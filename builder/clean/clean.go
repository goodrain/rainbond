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
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"

	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/sources"
)

//Manager CleanManager
type Manager struct {
	dclient *client.Client
	ctx     context.Context
	cancel  context.CancelFunc
}

//CreateCleanManager create clean manager
func CreateCleanManager() (*Manager, error) {
	dclient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Manager{
		dclient: dclient,
		ctx:     ctx,
		cancel:  cancel,
	}
	return c, nil
}

//Start start clean
func (t *Manager) Start(errchan chan error) error {
	run := func() {
		err := util.Exec(t.ctx, func() error {
			now := time.Now()
			datetime := now.AddDate(0, -1, 0)
			// Find more than five versions
			results, err := db.GetManager().VersionInfoDao().SearchVersionInfo()
			if err != nil {
				logrus.Error(err)
			}
			var serviceIDList []string
			for _, v := range results {
				serviceIDList = append(serviceIDList, v.ServiceID)
			}
			versions, err := db.GetManager().VersionInfoDao().GetVersionInfo(datetime, serviceIDList)
			if err != nil {
				logrus.Error(err)
			}

			for _, v := range versions {
				if v.DeliveredType == "image" {
					imagePath := v.DeliveredPath
					err := sources.ImageRemove(t.dclient, imagePath) //remove image
					if err != nil {
						if strings.Contains(err.Error(), "No such image") {
							logrus.Error(err)
						} else {
							logrus.Error(err)
							continue
						}
					}
					if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
						logrus.Error(err)
						continue
					}
					logrus.Info("Image deletion successful:", imagePath)

				}
				if v.DeliveredType == "slug" {
					filePath := v.DeliveredPath
					if err := os.Remove(filePath); err != nil {
						if strings.Contains(err.Error(), "no such file or directory") {
							logrus.Error(err)
						} else {
							logrus.Error(err)
							continue

						}
					}
					if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
						logrus.Error(err)
						continue
					}
					logrus.Info("file deletion successful:", filePath)

				}

			}
			return nil
		}, 24*time.Hour)
		if err != nil {
			errchan <- err
		}
	}
	go run()
	return nil
}

//Stop stop
func (t *Manager) Stop() error {
	logrus.Info("CleanManager is stoping.")
	t.cancel()
	return nil
}
