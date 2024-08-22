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

// 本文件实现了一个磁盘使用情况的异步统计工具，用于定期统计和缓存租户服务的磁盘使用量。

// 1. `DiskCache` 结构体：
//    - 该结构体用于管理磁盘使用情况的缓存，包含一个用于存储缓存数据的切片（`cache`），以及数据库管理器（`dbmanager`）和上下文管理器（`ctx` 和 `cancel`）。
//    - `cache` 存储了键值对，每个键值对包含服务的唯一标识符（`Key`）和其对应的磁盘使用量（`Value`）。

// 2. `CreatDiskCache` 函数：
//    - 该函数用于创建并初始化一个 `DiskCache` 实例。
//    - 函数接受一个上下文（`ctx`）作为参数，并创建一个新的上下文和取消函数（`cancel`），然后返回一个初始化了数据库管理器和上下文的 `DiskCache` 实例。

// 3. `Start` 方法：
//    - 该方法用于启动磁盘使用情况的定期统计和缓存更新。
//    - 每隔五分钟（通过定时器 `timer` 控制），方法会调用 `setcache` 函数更新缓存数据。
//    - 方法通过监听上下文的取消信号（`d.ctx.Done()`）来判断何时停止统计。

// 4. `Stop` 方法：
//    - 该方法用于停止磁盘使用情况的统计。
//    - 调用取消函数（`d.cancel`）以终止统计任务，并记录停止信息。

// 5. `setcache` 方法：
//    - 该方法负责更新磁盘使用情况的缓存数据。
//    - 方法会从数据库中获取所有服务的 ID（目前相关代码被注释），并计算每个服务对应的磁盘使用量，最终将结果存储在 `cache` 切片中。
//    - 该功能暂时仅记录了日志，未实际执行数据库查询和磁盘计算逻辑。

// 6. `Get` 方法：
//    - 该方法用于获取当前缓存的磁盘统计数据。
//    - 方法将缓存中的每个服务的磁盘使用量累加并返回一个键值对（`map[string]float64`），其中键为服务的唯一标识符，值为对应的磁盘使用量。

// 总体而言，本文件实现了一个用于异步统计和缓存租户服务磁盘使用情况的工具，主要通过定期更新缓存数据的方式实现对磁盘使用的监控和分析。

package statistical

import (
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// DiskCache 磁盘异步统计
type DiskCache struct {
	cache []struct {
		Key   string
		Value float64
	}
	dbmanager db.Manager
	ctx       context.Context
	cancel    context.CancelFunc
}

// CreatDiskCache 创建
func CreatDiskCache(ctx context.Context) *DiskCache {
	cctx, cancel := context.WithCancel(ctx)
	return &DiskCache{
		dbmanager: db.GetManager(),
		ctx:       cctx,
		cancel:    cancel,
	}
}

// Start 开始启动统计
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

// Stop stop
func (d *DiskCache) Stop() {
	logrus.Info("stop disk cache statistics")
	d.cancel()
}
func (d *DiskCache) setcache() {
	logrus.Info("start get all service disk size")
	//start := time.Now()
	var diskcache []struct {
		Key   string
		Value float64
	}
	//services, err := d.dbmanager.TenantServiceDao().GetAllServicesID()
	//if err != nil {
	//	logrus.Errorln("Error get tenant service when select db :", err)
	//	return
	//}
	//_, err := d.dbmanager.TenantServiceVolumeDao().GetAllVolumes()
	//if err != nil {
	//	logrus.Errorln("Error get tenant service volume when select db :", err)
	//	return
	//}
	//sharePath := os.Getenv("SHARE_DATA_PATH")
	//if sharePath == "" {
	//	sharePath = "/grdata"
	//}
	//var cache = make(map[string]*model.TenantServices)
	//for _, service := range services {
	//	//service nfs volume
	//	size := util.GetDirSize(fmt.Sprintf("%s/tenant/%s/service/%s", sharePath, service.TenantID, service.ServiceID))
	//	if size != 0 {
	//		diskcache = append(diskcache, struct {
	//			Key   string
	//			Value float64
	//		}{
	//			Key:   service.ServiceID + "_" + service.AppID + "_" + service.TenantID,
	//			Value: size,
	//		})
	//	}
	//	cache[service.ServiceID] = service
	//}
	d.cache = diskcache
	//logrus.Infof("end get all service disk size,time consum %2.f s", time.Since(start).Seconds())
}

// Get 获取磁盘统计结果
func (d *DiskCache) Get() map[string]float64 {
	newcache := make(map[string]float64)
	for _, v := range d.cache {
		newcache[v.Key] += v.Value
	}
	return newcache
}
