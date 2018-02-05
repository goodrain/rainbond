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

package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/status"
	"github.com/goodrain/rainbond/pkg/util"
)

//DiskCache 磁盘异步统计
type DiskCache struct {
	cache         map[string]float64
	dbmanager     db.Manager
	statusManager status.ServiceStatusManager
	ctx           context.Context
}

//CreatDiskCache 创建
func CreatDiskCache(ctx context.Context, statusManager status.ServiceStatusManager) *DiskCache {
	return &DiskCache{
		dbmanager:     db.GetManager(),
		statusManager: statusManager,
		ctx:           ctx,
	}
}

//Start 开始启动统计
func (d *DiskCache) Start() {
	d.setcache()
	timer := time.NewTimer(time.Minute * 5)
	defer timer.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-timer.C:
			d.setcache()
			timer.Reset(time.Minute * 5)
		}
	}
}

func (d *DiskCache) setcache() {
	logrus.Info("start get all service disk size")
	start := time.Now()
	d.cache = nil
	d.cache = make(map[string]float64)
	services, err := d.dbmanager.TenantServiceDao().GetAllServices()
	if err != nil {
		logrus.Errorln("Error get tenant service when select db :", err)
	}
	volumes, err := d.dbmanager.TenantServiceVolumeDao().GetAllVolumes()
	if err != nil {
		logrus.Errorln("Error get tenant service volume when select db :", err)
	}
	localPath := os.Getenv("LOCAL_DATA_PATH")
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if localPath == "" {
		localPath = "/grlocaldata"
	}
	if sharePath == "" {
		sharePath = "/grdata"
	}
	var cache = make(map[string]*model.TenantServices)
	for _, service := range services {
		//默认目录
		size := util.GetDirSize(fmt.Sprintf("%s/tenant/%s/service/%s", sharePath, service.TenantID, service.ServiceID))
		if size != 0 {
			d.cache[service.ServiceID+"_"+service.TenantID] = size
		}
		cache[service.ServiceID] = service
	}
	gettenantID := func(serviceID string) string {
		if service, ok := cache[serviceID]; ok {
			return service.TenantID
		}
		return ""
	}
	for _, v := range volumes {
		if v.VolumeType == string(model.LocalVolumeType) {
			//默认目录
			size := util.GetDirSize(fmt.Sprintf("%s/tenant/%s/service/%s", localPath, gettenantID(v.ServiceID), v.ServiceID))
			if size != 0 {
				d.cache[v.ServiceID+"_"+gettenantID(v.ServiceID)] += size
			}
		}
	}
	logrus.Infof("end get all service disk size,time consum %d s", time.Now().Sub(start).Seconds())
}

//Get 获取磁盘统计结果
func (d *DiskCache) Get() map[string]float64 {
	return d.cache
}
