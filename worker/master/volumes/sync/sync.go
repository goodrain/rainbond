// 本文件定义了一个用于同步存储卷类型事件的工具。主要用于接收和处理存储卷类型的更新或创建事件，并将其同步到数据库中。

// 1. `VolumeTypeEvent` 结构体：
//    - 该结构体包含两个通道：`vtEventCh` 和 `stopCh`。
//    - `vtEventCh` 用于接收存储卷类型的事件（类型为 `model.TenantServiceVolumeType`），并存储在一个有缓冲区的通道中。
//    - `stopCh` 是一个信号通道，用于接收停止处理事件的信号。

// 2. `New` 函数：
//    - 该函数用于创建并初始化一个 `VolumeTypeEvent` 实例。
//    - 函数接受一个停止信号通道（`stopCh`）作为参数，并返回一个包含该通道和一个新的事件通道（`vtEventCh`）的 `VolumeTypeEvent` 实例。

// 3. `GetChan` 方法：
//    - 该方法用于获取 `VolumeTypeEvent` 实例中的事件通道（`vtEventCh`），供外部向该通道发送存储卷类型事件。

// 4. `Handle` 方法：
//    - 该方法是事件处理的核心逻辑，用于持续监听事件通道（`vtEventCh`）并处理接收到的存储卷类型事件。
//    - 在一个无限循环中，方法通过 `select` 语句同时监听两个通道：
//      - 如果收到停止信号（`e.stopCh`），方法将终止处理并返回。
//      - 如果接收到存储卷类型事件（`vtEventCh`），方法会调用数据库管理器的 `VolumeTypeDao` 进行事件的创建或更新操作。
//    - 如果数据库操作发生错误，方法会记录错误日志，并忽略该错误，继续处理后续的事件。

// 总体而言，本文件实现了一个用于同步存储卷类型事件的工具，通过监听和处理事件通道中的事件，将存储卷类型信息同步到数据库中，以确保系统中存储卷类型的最新状态。

package sync

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
)

// VolumeTypeEvent -
type VolumeTypeEvent struct {
	vtEventCh chan *model.TenantServiceVolumeType
	stopCh    chan struct{}
}

// New -
func New(stopCh chan struct{}) *VolumeTypeEvent {
	return &VolumeTypeEvent{
		stopCh:    stopCh,
		vtEventCh: make(chan *model.TenantServiceVolumeType, 100),
	}
}

// GetChan -
func (e *VolumeTypeEvent) GetChan() chan<- *model.TenantServiceVolumeType {
	return e.vtEventCh
}

// Handle -
func (e *VolumeTypeEvent) Handle() {
	for {
		select {
		case <-e.stopCh:
			return
		case vt := <-e.vtEventCh:
			if _, err := db.GetManager().VolumeTypeDao().CreateOrUpdateVolumeType(vt); err != nil {
				logrus.Errorf("sync storageClass error : %s, ignore it", err.Error())
			}
		}
	}
}
