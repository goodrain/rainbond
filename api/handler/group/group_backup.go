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

package group

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"

	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"
	apidb "github.com/goodrain/rainbond/api/db"

	"github.com/goodrain/rainbond/appruntimesync/client"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"

	"github.com/jinzhu/gorm"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	core_util "github.com/goodrain/rainbond/util"
)

//Backup GroupBackup
// swagger:parameters groupBackup
type Backup struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	Body       struct {
		EventID    string   `json:"event_id" validate:"event_id|required"`
		GroupID    string   `json:"group_id" validate:"group_name|required"`
		Metadata   string   `json:"metadata,omitempty" validate:"metadata|required"`
		ServiceIDs []string `json:"service_ids" validate:"service_ids|required"`
		Mode       string   `json:"mode" validate:"mode|required|in:full-online,full-offline"`
		Version    string   `json:"version" validate:"version|required"`
		SlugInfo   struct {
			Namespace   string `json:"namespace"`
			FTPHost     string `json:"ftp_host"`
			FTPPort     string `json:"ftp_port"`
			FTPUser     string `json:"ftp_username"`
			FTPPassword string `json:"ftp_password"`
		} `json:"slug_info,omitempty"`
		ImageInfo struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty"`
		} `json:"image_info,omitempty"`
		SourceDir string `json:"source_dir"`
		BackupID  string `json:"backup_id,omitempty"`
	}
}

//BackupHandle group app backup handle
type BackupHandle struct {
	MQClient  pb.TaskQueueClient
	statusCli *client.AppRuntimeSyncClient
	etcdCli   *clientv3.Client
}

//CreateBackupHandle CreateBackupHandle
func CreateBackupHandle(MQClient pb.TaskQueueClient, statusCli *client.AppRuntimeSyncClient, etcdCli *clientv3.Client) *BackupHandle {
	return &BackupHandle{MQClient: MQClient, statusCli: statusCli, etcdCli: etcdCli}
}

//NewBackup new backup task
func (h *BackupHandle) NewBackup(b Backup) (*dbmodel.AppBackup, *util.APIHandleError) {
	logger := event.GetManager().GetLogger(b.Body.EventID)
	var appBackup = dbmodel.AppBackup{
		EventID:    b.Body.EventID,
		BackupID:   core_util.NewUUID(),
		GroupID:    b.Body.GroupID,
		Status:     "starting",
		Version:    b.Body.Version,
		BackupMode: b.Body.Mode,
	}
	//check last backup task whether complete or version whether exist
	if db.GetManager().AppBackupDao().CheckHistory(b.Body.GroupID, b.Body.Version) {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("last backup task do not complete or have restore backup or version is exist"))
	}
	//check all service exist
	if alias, err := db.GetManager().TenantServiceDao().GetServiceAliasByIDs(b.Body.ServiceIDs); len(alias) != len(b.Body.ServiceIDs) || err != nil {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("some services do not exist in need backup services"))
	}
	//make source dir
	sourceDir := fmt.Sprintf("/grdata/groupbackup/%s_%s", b.Body.GroupID, b.Body.Version)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("create backup dir error,%s", err))
	}
	b.Body.SourceDir = sourceDir
	appBackup.SourceDir = sourceDir
	//snapshot the app metadata of region and write
	if err := h.snapshot(b.Body.ServiceIDs, sourceDir); err != nil {
		os.RemoveAll(sourceDir)
		if strings.HasPrefix(err.Error(), "Statefulset app must be closed") {
			return nil, util.CreateAPIHandleError(401, fmt.Errorf("snapshot group apps error,%s", err))
		}
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("snapshot group apps error,%s", err))
	}
	logger.Info(core_util.Translation("write region level metadata success"), map[string]string{"step": "back-api"})
	//write console level metadata.
	if err := ioutil.WriteFile(fmt.Sprintf("%s/console_apps_metadata.json", sourceDir), []byte(b.Body.Metadata), 0755); err != nil {
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("write metadata file error,%s", err))
	}
	logger.Info(core_util.Translation("write console level metadata success"), map[string]string{"step": "back-api"})
	//save backup history
	if err := db.GetManager().AppBackupDao().AddModel(&appBackup); err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("create backup history", err)
	}
	//clear metadata
	b.Body.Metadata = ""
	b.Body.BackupID = appBackup.BackupID
	data, err := ffjson.Marshal(b.Body)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("build task body data error,%s", err))
	}
	//Initiate a data backup task.
	task := &apidb.BuildTaskStruct{
		TaskType: "backup_apps_new",
		TaskBody: data,
	}
	eq, err := apidb.BuildTaskBuild(task)
	if err != nil {
		logrus.Error("Failed to BuildTaskBuild for BackupApp:", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("build task error,%s", err))
	}
	// 写入事件到MQ中
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = h.MQClient.Enqueue(ctx, eq)
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for BackupApp:", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("build enqueue task error,%s", err))
	}
	logger.Info(core_util.Translation("Asynchronous tasks are sent successfully"), map[string]string{"step": "back-api"})
	return &appBackup, nil
}

//GetBackup get one backup info
func (h *BackupHandle) GetBackup(backupID string) (*dbmodel.AppBackup, *util.APIHandleError) {
	backup, err := db.GetManager().AppBackupDao().GetAppBackup(backupID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get backup history", err)
	}
	return backup, nil
}

//DeleteBackup delete backup
func (h *BackupHandle) DeleteBackup(backupID string) *util.APIHandleError {
	backup, err := h.GetBackup(backupID)
	if err != nil {
		return err
	}
	//if status != success it could be deleted
	//if status == success, backup mode must be offline could be deleted
	if backup.Status != "success" || backup.BackupMode == "full-offline" {
		backup.Deleted = true
		if er := db.GetManager().AppBackupDao().UpdateModel(backup); er != nil {
			return util.CreateAPIHandleErrorFromDBError("delete backup error", er)
		}
		return nil
	}
	return util.CreateAPIHandleErrorf(400, "backup success do not support delete.")
}

//GetBackupByGroupID get some backup info by group id
func (h *BackupHandle) GetBackupByGroupID(groupID string) ([]*dbmodel.AppBackup, *util.APIHandleError) {
	backups, err := db.GetManager().AppBackupDao().GetAppBackups(groupID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get backup history", err)
	}
	return backups, nil
}

//RegionServiceSnapshot RegionServiceSnapshot
type RegionServiceSnapshot struct {
	ServiceID          string
	Service            *dbmodel.TenantServices
	ServiceProbe       []*dbmodel.ServiceProbe
	LBMappingPort      []*dbmodel.TenantServiceLBMappingPort
	ServiceEnv         []*dbmodel.TenantServiceEnvVar
	ServiceLabel       []*dbmodel.TenantServiceLable
	ServiceMntRelation []*dbmodel.TenantServiceMountRelation
	PluginRelation     []*dbmodel.TenantServicePluginRelation
	ServiceRelation    []*dbmodel.TenantServiceRelation
	ServiceStatus      string
	ServiceVolume      []*dbmodel.TenantServiceVolume
	ServicePort        []*dbmodel.TenantServicesPort
	Versions           []*dbmodel.VersionInfo
}

//snapshot
func (h *BackupHandle) snapshot(ids []string, sourceDir string) error {
	var datas []RegionServiceSnapshot
	for _, id := range ids {
		var data = RegionServiceSnapshot{
			ServiceID: id,
		}
		status := h.statusCli.GetStatus(id)
		serviceType, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceTypeLabel(id)
		if err != nil {
			return fmt.Errorf("Get service deploy type error,%s", err.Error())
		}
		if status != client.CLOSED && serviceType != nil && serviceType.LabelValue == core_util.StatefulServiceType {
			return fmt.Errorf("Statefulset app must be closed before backup,%s", err.Error())
		}
		data.ServiceStatus = status
		service, err := db.GetManager().TenantServiceDao().GetServiceByID(id)
		if err != nil {
			return fmt.Errorf("Get service(%s) error %s", id, err.Error())
		}
		data.Service = service
		serviceProbes, err := db.GetManager().ServiceProbeDao().GetServiceProbes(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) probe error %s", id, err)
		}
		data.ServiceProbe = serviceProbes
		lbmappingPorts, err := db.GetManager().TenantServiceLBMappingPortDao().GetTenantServiceLBMappingPortByService(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) lb mapping port error %s", id, err)
		}
		data.LBMappingPort = lbmappingPorts
		serviceEnv, err := db.GetManager().TenantServiceEnvVarDao().GetServiceEnvs(id, nil)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) envs error %s", id, err)
		}
		data.ServiceEnv = serviceEnv
		serviceLabels, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceLabel(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) labels error %s", id, err)
		}
		data.ServiceLabel = serviceLabels
		serviceMntRelations, err := db.GetManager().TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) mnt relations error %s", id, err)
		}
		data.ServiceMntRelation = serviceMntRelations
		servicePlugins, err := db.GetManager().TenantServicePluginRelationDao().GetALLRelationByServiceID(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) plugins error %s", id, err)
		}
		data.PluginRelation = servicePlugins
		serviceRelations, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) relations error %s", id, err)
		}
		data.ServiceRelation = serviceRelations
		serviceVolume, err := db.GetManager().TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) volume error %s", id, err)
		}
		data.ServiceVolume = serviceVolume
		servicePorts, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) ports error %s", id, err)
		}
		data.ServicePort = servicePorts
		versions, err := db.GetManager().VersionInfoDao().GetVersionByServiceID(id)
		if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
			return fmt.Errorf("Get service(%s) build versions error %s", id, err)
		}
		data.Versions = versions
		datas = append(datas, data)
	}
	body, err := ffjson.Marshal(datas)
	if err != nil {
		return err
	}
	//write region level metadata.
	if err := ioutil.WriteFile(fmt.Sprintf("%s/region_apps_metadata.json", sourceDir), body, 0755); err != nil {
		return util.CreateAPIHandleError(500, fmt.Errorf("write region_apps_metadata file error,%s", err))
	}
	return nil
}

//BackupRestore BackupRestore
type BackupRestore struct {
	BackupID string `json:"backup_id"`
	Body     struct {
		SlugInfo struct {
			Namespace   string `json:"namespace"`
			FTPHost     string `json:"ftp_host"`
			FTPPort     string `json:"ftp_port"`
			FTPUser     string `json:"ftp_username"`
			FTPPassword string `json:"ftp_password"`
		} `json:"slug_info,omitempty"`
		ImageInfo struct {
			HubURL      string `json:"hub_url"`
			HubUser     string `json:"hub_user"`
			HubPassword string `json:"hub_password"`
			Namespace   string `json:"namespace"`
			IsTrust     bool   `json:"is_trust,omitempty"`
		} `json:"image_info,omitempty"`
		EventID string `json:"event_id"`
		//need restore target tenant id
		TenantID string `json:"tenant_id"`
		//RestoreMode(cdct) current datacenter and current tenant
		//RestoreMode(cdot) current datacenter and other tenant
		//RestoreMode(od)     other datacenter
		RestoreMode string `json:"restore_mode"`
	}
}

//RestoreResult RestoreResult
type RestoreResult struct {
	Status        string           `json:"status"`
	Message       string           `json:"message"`
	CreateTime    time.Time        `json:"create_time"`
	ServiceChange map[string]*Info `json:"service_change"`
	BackupID      string           `json:"backup_id"`
	RestoreMode   string           `json:"restore_mode"`
	EventID       string           `json:"event_id"`
	RestoreID     string           `json:"restore_id"`
	Metadata      string           `json:"metadata"`
	CacheDir      string           `json:"cache_dir"`
}

//Info service cache info
type Info struct {
	ServiceID    string
	ServiceAlias string
	Status       string
	LBPorts      map[int]int
}

//RestoreBackup restore a backup version
//all app could be closed before restore
func (h *BackupHandle) RestoreBackup(br BackupRestore) (*RestoreResult, *util.APIHandleError) {
	logger := event.GetManager().GetLogger(br.Body.EventID)
	backup, Aerr := h.GetBackup(br.BackupID)
	if Aerr != nil {
		return nil, Aerr
	}
	if backup.Status != "success" || backup.SourceDir == "" || backup.SourceType == "" {
		return nil, util.CreateAPIHandleErrorf(500, "backup can not be restored")
	}
	var restoreID string
	if br.Body.EventID != "" {
		restoreID = br.Body.EventID
	}
	restoreID = core_util.NewUUID()
	var dataMap = map[string]interface{}{
		"slug_info":    br.Body.SlugInfo,
		"image_info":   br.Body.ImageInfo,
		"backup_id":    backup.BackupID,
		"tenant_id":    br.Body.TenantID,
		"restore_id":   restoreID,
		"restore_mode": br.Body.RestoreMode,
	}
	data, err := ffjson.Marshal(dataMap)
	if err != nil {
		return nil, util.CreateAPIHandleErrorf(500, "build task body data error,%s", err)
	}
	//Initiate a data backup task.
	task := &apidb.BuildTaskStruct{
		TaskType: "backup_apps_restore",
		TaskBody: data,
	}
	eq, err := apidb.BuildTaskBuild(task)
	if err != nil {
		logrus.Error("Failed to BuildTaskBuild for BackupApp:", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("build task error,%s", err))
	}
	// 写入事件到MQ中
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = h.MQClient.Enqueue(ctx, eq)
	if err != nil {
		logrus.Error("Failed to Enqueue MQ for BackupApp:", err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf("build enqueue task error,%s", err))
	}
	logger.Info(core_util.Translation("Asynchronous tasks are sent successfully"), map[string]string{"step": "back-api"})
	var rr = &RestoreResult{
		Status:      "starting",
		BackupID:    br.BackupID,
		EventID:     br.Body.EventID,
		RestoreMode: br.Body.RestoreMode,
		RestoreID:   restoreID,
	}
	body, _ := ffjson.Marshal(rr)
	_, err = h.etcdCli.Put(ctx, "/rainbond/backup_restore/"+restoreID, string(body))
	if err != nil {
		logrus.Errorf("save backup restore history error.")
		return nil, util.CreateAPIHandleError(500, err)
	}
	return rr, nil
}

//RestoreBackupResult RestoreBackupResult
func (h *BackupHandle) RestoreBackupResult(restoreID string) (*RestoreResult, *util.APIHandleError) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := h.etcdCli.Get(ctx, "/rainbond/backup_restore/"+restoreID)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if res.Count == 0 {
		return nil, util.CreateAPIHandleError(404, fmt.Errorf("restore result not exist "))
	}
	var rr RestoreResult
	if err := ffjson.Unmarshal(res.Kvs[0].Value, &rr); err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if rr.Status == "success" {
		//write console level metadata.
		body, err := ioutil.ReadFile(fmt.Sprintf("%s/console_apps_metadata.json", rr.CacheDir))
		if err != nil {
			return nil, util.CreateAPIHandleError(500, fmt.Errorf("read metadata file error,%s", err))
		}
		rr.Metadata = string(body)
		//set app status
		for _, v := range rr.ServiceChange {
			if v.Status == client.UNDEPLOY {
				h.statusCli.SetStatus(v.ServiceID, client.UNDEPLOY)
			} else {
				h.statusCli.SetStatus(v.ServiceID, client.CLOSED)
			}
		}
	}
	return &rr, nil
}

//BackupCopy BackupCopy
type BackupCopy struct {
	Body struct {
		EventID string `json:"event_id" validate:"event_id|required"`
		GroupID string `json:"group_id" validate:"group_id|required"`
		//Status in starting,failed,success,restore
		Status     string `json:"status" validate:"status|required"`
		Version    string `json:"version" validate:"version|required"`
		SourceDir  string `json:"source_dir" validate:"source_dir|required"`
		SourceType string ` json:"source_type" validate:"source_type|required"`
		BackupMode string `json:"backup_mode" validate:"backup_mode|required"`
		BuckupSize int    `json:"backup_size" validate:"backup_size|required"`
	}
}

//BackupCopy BackupCopy
func (h *BackupHandle) BackupCopy(b BackupCopy) (*dbmodel.AppBackup, *util.APIHandleError) {
	var ab dbmodel.AppBackup
	ab.BackupID = core_util.NewUUID()
	ab.EventID = b.Body.EventID
	ab.GroupID = b.Body.GroupID
	ab.Status = b.Body.Status
	ab.Version = b.Body.Version
	ab.SourceDir = b.Body.SourceDir
	ab.SourceType = b.Body.SourceType
	ab.BackupMode = b.Body.BackupMode
	ab.BuckupSize = b.Body.BuckupSize
	if err := db.GetManager().AppBackupDao().AddModel(&ab); err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("copy backup", err)
	}
	return &ab, nil
}
