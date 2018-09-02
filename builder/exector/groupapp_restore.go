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

package exector

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"

	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	dbmodel "github.com/goodrain/rainbond/db/model"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/tidwall/gjson"
)

//BackupAPPRestore restrore the  group app backup
type BackupAPPRestore struct {
	//full-online,full-offline
	EventID  string
	SlugInfo struct {
		Namespace   string `json:"namespace"`
		FTPHost     string `json:"ftp_host"`
		FTPPort     string `json:"ftp_port"`
		FTPUser     string `json:"ftp_username"`
		FTPPassword string `json:"ftp_password"`
	} `json:"slug_info"`
	ImageInfo struct {
		HubURL      string `json:"hub_url"`
		HubUser     string `json:"hub_user"`
		HubPassword string `json:"hub_password"`
		Namespace   string `json:"namespace"`
		IsTrust     bool   `json:"is_trust,omitempty"`
	} `json:"image_info,omitempty"`
	BackupID string `json:"backup_id"`
	TenantID string `json:"tenant_id"`
	Logger   event.Logger
	//RestoreMode(cdct) current datacenter and current tenant
	//RestoreMode(cdot) current datacenter and other tenant
	//RestoreMode(od)     other datacenter
	RestoreMode  string `json:"restore_mode"`
	RestoreID    string `json:"restore_id"`
	DockerClient *client.Client
	cacheDir     string
	//serviceChange  key: oldServiceID
	serviceChange map[string]*Info
	etcdcli       *clientv3.Client
}

//Info service cache info
type Info struct {
	ServiceID    string
	ServiceAlias string
	Status       string
	LBPorts      map[int]int
}

func init() {
	RegisterWorker("backup_apps_restore", BackupAPPRestoreCreater)
}

//BackupAPPRestoreCreater create
func BackupAPPRestoreCreater(in []byte, m *exectorManager) (TaskWorker, error) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	backupRestore := &BackupAPPRestore{
		Logger:        logger,
		EventID:       eventID,
		DockerClient:  m.DockerClient,
		etcdcli:       m.EtcdCli,
		serviceChange: make(map[string]*Info, 0),
	}
	if err := ffjson.Unmarshal(in, &backupRestore); err != nil {
		return nil, err
	}
	return backupRestore, nil
}

//Run Run
func (b *BackupAPPRestore) Run(timeout time.Duration) error {
	//download or copy backup data
	backup, err := db.GetManager().AppBackupDao().GetAppBackup(b.BackupID)
	if err != nil {
		return err
	}
	if backup.Status != "success" || backup.SourceDir == "" || backup.SourceType == "" {
		return fmt.Errorf("backup can not be restore")
	}
	cacheDir := fmt.Sprintf("/grdata/cache/tmp/%s/%s", b.BackupID, b.EventID)
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		return fmt.Errorf("create cache dir error %s", err.Error())
	}
	b.cacheDir = cacheDir
	switch backup.SourceType {
	case "sftp":
		b.downloadFromFTP(backup)
	default:
		b.downloadFromLocal(backup)
	}
	//read metadata file
	metadata, err := ioutil.ReadFile(fmt.Sprintf("%s/region_apps_metadata.json", b.cacheDir))
	if err != nil {
		return err
	}
	var appSnapshots []*RegionServiceSnapshot
	if err := ffjson.Unmarshal(metadata, &appSnapshots); err != nil {
		return err
	}
	b.Logger.Info("读取备份元数据完成", map[string]string{"step": "restore_builder", "status": "success"})
	//modify the metadata
	if err := b.modify(appSnapshots); err != nil {
		return err
	}
	//restore metadata to db
	if err := b.restoreMetadata(appSnapshots); err != nil {
		return err
	}
	b.Logger.Info("恢复备份元数据完成", map[string]string{"step": "restore_builder", "status": "success"})
	//If the following error occurs, delete the data from the database
	//restore all app all builde version and data
	if err := b.restoreVersionAndData(backup, appSnapshots); err != nil {
		return err
	}
	//save result
	b.saveResult("success", "")
	return nil
}
func (b *BackupAPPRestore) getServiceType(labels []*dbmodel.TenantServiceLable) string {
	for _, l := range labels {
		if l.LabelKey == dbmodel.LabelKeyServiceType {
			return l.LabelValue
		}
	}
	return util.StatelessServiceType
}
func (b *BackupAPPRestore) restoreVersionAndData(backup *dbmodel.AppBackup, appSnapshots []*RegionServiceSnapshot) error {
	for _, app := range appSnapshots {
		//backup app image or code slug file
		b.Logger.Info(fmt.Sprintf("开始恢复应用(%s)运行环境", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "starting"})
		for _, version := range app.Versions {
			if version.DeliveredType == "slug" && version.FinalStatus == "success" {
				if err := b.downloadSlug(backup, app, version); err != nil {
					logrus.Errorf("download app slug file error.%s", err.Error())
					return err
				}
			}
			if version.DeliveredType == "image" && version.FinalStatus == "success" {
				if err := b.downloadImage(backup, app, version); err != nil {
					logrus.Errorf("download app image error.%s", err.Error())
					return err
				}
			}
		}
		b.Logger.Info(fmt.Sprintf("完成恢复应用(%s)运行环境", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "success"})
		b.Logger.Info(fmt.Sprintf("开始恢复应用(%s)持久化数据", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "starting"})
		//restore app data
		for _, volume := range app.ServiceVolume {
			if volume.HostPath != "" {
				dstDir := fmt.Sprintf("%s/data_%s/%s.zip", b.cacheDir, b.getOldServiceID(app.ServiceID), strings.Replace(volume.VolumeName, "/", "", -1))
				tmpDir := fmt.Sprintf("/grdata/tmp/%s_%d", volume.ServiceID, volume.ID)
				if err := util.Unzip(dstDir, tmpDir); err != nil {
					if !strings.Contains(err.Error(), "no such file") {
						logrus.Errorf("restore service(%s) volume(%s) data error.%s", app.ServiceID, volume.VolumeName, err.Error())
						return err
					}
					//backup data is not exist because dir is empty.
					//so create host path and continue
					os.MkdirAll(volume.HostPath, 0777)
					continue
				}
				//if app type is statefulset, change pod hostpath
				if b.getServiceType(app.ServiceLabel) == util.StatefulServiceType {
					//Next two level directory
					list, err := util.GetDirList(tmpDir, 2)
					if err != nil {
						logrus.Errorf("restore statefulset service(%s) volume(%s) data error.%s", app.ServiceID, volume.VolumeName, err.Error())
						return err
					}
					for _, path := range list {
						newNameTmp := strings.Split(filepath.Base(path), "-")
						newNameTmp[0] = b.serviceChange[b.getOldServiceID(app.ServiceID)].ServiceAlias
						newName := strings.Join(newNameTmp, "-")
						newpath := filepath.Join(util.GetParentDirectory(path), newName)
						err := util.Rename(path, newpath)
						if err != nil {
							if strings.Contains(err.Error(), "file exists") {
								if err := util.MergeDir(path, newpath); err != nil {
									return err
								}
							} else {
								return err
							}
						}
						if err := os.Chmod(newpath, 0777); err != nil {
							return err
						}
					}
				}
				err := util.Rename(tmpDir, util.GetParentDirectory(volume.HostPath))
				if err != nil {
					if strings.Contains(err.Error(), "file exists") {
						if err := util.MergeDir(tmpDir, util.GetParentDirectory(volume.HostPath)); err != nil {
							return err
						}
					} else {
						return err
					}
				}
				if err := os.Chmod(volume.HostPath, 0777); err != nil {
					return err
				}
			}
		}
		if app.Service.HostPath != "" {
			dstDir := fmt.Sprintf("%s/data_%s/%s_common.zip", b.cacheDir, b.getOldServiceID(app.ServiceID), b.getOldServiceID(app.ServiceID))
			tmpDir := fmt.Sprintf("/grdata/tmp/%s_%s", app.ServiceID, app.ServiceID)
			if err := util.Unzip(dstDir, tmpDir); err != nil {
				if !strings.Contains(err.Error(), "no such file") {
					logrus.Errorf("restore service(%s) default volume data error.%s", app.ServiceID, err.Error())
					return err
				}
				//backup data is not exist because dir is empty.
				//so create host path and continue
				os.MkdirAll(app.Service.HostPath, 0777)
			} else {
				err := util.Rename(tmpDir, util.GetParentDirectory(app.Service.HostPath))
				if err != nil {
					if strings.Contains(err.Error(), "file exists") {
						if err := util.MergeDir(tmpDir, util.GetParentDirectory(app.Service.HostPath)); err != nil {
							return err
						}
					} else {
						return err
					}
				}
				if err := os.Chmod(app.Service.HostPath, 0777); err != nil {
					return err
				}
			}
		}
		b.Logger.Info(fmt.Sprintf("完成恢复应用(%s)持久化数据", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "success"})
		//TODO:relation relation volume data?
	}
	return nil
}
func (b *BackupAPPRestore) getOldServiceID(new string) string {
	for k, v := range b.serviceChange {
		if v.ServiceID == new {
			return k
		}
	}
	return ""
}
func (b *BackupAPPRestore) downloadSlug(backup *dbmodel.AppBackup, app *RegionServiceSnapshot, version *dbmodel.VersionInfo) error {
	if backup.BackupMode == "full-online" && b.SlugInfo.FTPHost != "" && b.SlugInfo.FTPPort != "" {
		sFTPClient, err := sources.NewSFTPClient(b.SlugInfo.FTPUser, b.SlugInfo.FTPPassword, b.SlugInfo.FTPHost, b.SlugInfo.FTPPort)
		if err != nil {
			b.Logger.Error(util.Translation("create ftp client error"), map[string]string{"step": "restore_builder", "status": "failure"})
			return err
		}
		defer sFTPClient.Close()
		dstDir := fmt.Sprintf("%s/app_%s/%s.tgz", filepath.Dir(backup.SourceDir), b.getOldServiceID(app.ServiceID), version.BuildVersion)
		if err := sFTPClient.DownloadFile(dstDir, version.DeliveredPath, b.Logger); err != nil {
			b.Logger.Error(util.Translation("down slug file from sftp server error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("down %s slug file error when backup app , %s", dstDir, err.Error())
			return err
		}
	} else {
		dstDir := fmt.Sprintf("%s/app_%s/slug_%s.tgz", b.cacheDir, b.getOldServiceID(app.ServiceID), version.BuildVersion)
		if err := sources.CopyFileWithProgress(dstDir, version.DeliveredPath, b.Logger); err != nil {
			b.Logger.Error(util.Translation("down slug file from local dir error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("copy slug file error when backup app, %s", err.Error())
			return err
		}
	}
	return nil
}

func (b *BackupAPPRestore) downloadImage(backup *dbmodel.AppBackup, app *RegionServiceSnapshot, version *dbmodel.VersionInfo) error {
	if backup.BackupMode == "full-online" && b.ImageInfo.HubURL != "" {
		backupImage, err := app.Service.CreateShareImage(b.ImageInfo.HubURL, b.ImageInfo.Namespace, fmt.Sprintf("%s_backup", backup.Version))
		if err != nil {
			return fmt.Errorf("create backup image error %s", err)
		}
		if _, err := sources.ImagePull(b.DockerClient, backupImage, b.ImageInfo.HubUser, b.ImageInfo.HubPassword, b.Logger, 10); err != nil {
			b.Logger.Error(util.Translation("pull image from hub error"), map[string]string{"step": "restore_builder", "status": "failure"})
			return fmt.Errorf("restore backup image pull error %s", err)
		}
		if err := sources.ImageTag(b.DockerClient, backupImage, version.DeliveredPath, b.Logger, 1); err != nil {
			return fmt.Errorf("change image tag when restore backup error %s", err)
		}
		err = sources.ImagePush(b.DockerClient, version.DeliveredPath, builder.REGISTRYUSER, builder.REGISTRYPASS, b.Logger, 10)
		if err != nil {
			return fmt.Errorf("push image to local  when restore backup error %s", err)
		}
	} else {
		dstDir := fmt.Sprintf("%s/app_%s/image_%s.tar", b.cacheDir, b.getOldServiceID(app.ServiceID), version.BuildVersion)
		if err := sources.ImageLoad(b.DockerClient, dstDir, b.Logger); err != nil {
			b.Logger.Error(util.Translation("load image to local hub error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("load image to local hub error when restore backup app, %s", err.Error())
			return err
		}
	}
	return nil
}

//if restore error, will clear
func (b *BackupAPPRestore) clear() {
	//clear db
	manager := db.GetManager()
	for _, v := range b.serviceChange {
		serviceID := v.ServiceID
		manager.TenantServiceDao().DeleteServiceByServiceID(serviceID)
		manager.TenantServicesPortDao().DELPortsByServiceID(serviceID)
		manager.ServiceProbeDao().DELServiceProbesByServiceID(serviceID)
		manager.TenantServiceLBMappingPortDao().DELServiceLBMappingPortByServiceID(serviceID)
		manager.TenantServiceEnvVarDao().DELServiceEnvsByServiceID(serviceID)
		manager.TenantServiceLabelDao().DeleteLabelByServiceID(serviceID)
		manager.TenantServiceMountRelationDao().DELTenantServiceMountRelationByServiceID(serviceID)
		manager.TenantServicePluginRelationDao().DeleteALLRelationByServiceID(serviceID)
		manager.TenantServiceRelationDao().DELRelationsByServiceID(serviceID)
		manager.TenantServiceVolumeDao().DeleteTenantServiceVolumesByServiceID(serviceID)
		manager.VersionInfoDao().DeleteVersionByServiceID(serviceID)
	}
	//clear cache data
	os.RemoveAll(b.cacheDir)
}
func (b *BackupAPPRestore) modify(appSnapshots []*RegionServiceSnapshot) error {
	for _, app := range appSnapshots {
		oldServiceID := app.ServiceID
		// if b.RestoreMode == "cdot" || b.RestoreMode == "od" {
		//change tenant
		app.Service.TenantID = b.TenantID
		for _, port := range app.ServicePort {
			port.TenantID = b.TenantID
		}
		for _, relation := range app.ServiceRelation {
			relation.TenantID = b.TenantID
		}
		for _, env := range app.ServiceEnv {
			env.TenantID = b.TenantID
		}
		for _, smr := range app.ServiceMntRelation {
			smr.TenantID = b.TenantID
		}
		//}
		//change service_id and service_alias
		newServiceID := util.NewUUID()
		newServiceAlias := "gr" + newServiceID[26:]
		app.ServiceID = newServiceID
		app.Service.ServiceID = newServiceID
		app.Service.ServiceAlias = newServiceAlias
		for _, a := range app.ServiceProbe {
			a.ServiceID = newServiceID
		}
		for _, a := range app.LBMappingPort {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceEnv {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceLabel {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceMntRelation {
			a.ServiceID = newServiceID
		}
		for _, a := range app.PluginRelation {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceRelation {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceVolume {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServicePort {
			a.ServiceID = newServiceID
		}
		for _, a := range app.Versions {
			a.ServiceID = newServiceID
		}
		b.serviceChange[oldServiceID] = &Info{
			ServiceID:    newServiceID,
			ServiceAlias: newServiceAlias,
			Status:       app.ServiceStatus,
		}
	}
	//modify relations
	for _, app := range appSnapshots {
		for _, a := range app.ServiceMntRelation {
			a.DependServiceID = b.serviceChange[a.DependServiceID].ServiceID
		}
		for _, a := range app.ServiceRelation {
			a.DependServiceID = b.serviceChange[a.DependServiceID].ServiceID
		}
	}
	return nil
}
func (b *BackupAPPRestore) restoreMetadata(appSnapshots []*RegionServiceSnapshot) error {
	tx := db.GetManager().Begin()
	for _, app := range appSnapshots {
		app.Service.ID = 0
		if err := db.GetManager().TenantServiceDaoTransactions(tx).AddModel(app.Service); err != nil {
			tx.Rollback()
			return fmt.Errorf("create app when restore backup error. %s", err.Error())
		}
		for _, a := range app.ServiceProbe {
			a.ID = 0
			if err := db.GetManager().ServiceProbeDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app probe when restore backup error. %s", err.Error())
			}
		}
		for _, a := range app.LBMappingPort {
			a.ID = 0
			if err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).AddModel(a); err != nil {
				if strings.Contains(err.Error(), "is exist ") {
					//modify the lb port
					maport, err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).CreateTenantServiceLBMappingPort(a.ServiceID, a.ContainerPort)
					if err != nil {
						tx.Rollback()
						return fmt.Errorf("create new app lb port when restore backup error. %s", err.Error())
					}
					info := b.serviceChange[b.getOldServiceID(app.ServiceID)]
					if info == nil {
						continue
					}
					if info.LBPorts == nil {
						info.LBPorts = map[int]int{maport.ContainerPort: maport.Port}
					} else {
						info.LBPorts[maport.ContainerPort] = maport.Port
					}
				} else {
					tx.Rollback()
					return fmt.Errorf("create app probe when restore backup error. %s", err.Error())
				}
			}
		}
		for _, a := range app.ServiceEnv {
			a.ID = 0
			if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app envs when restore backup error. %s", err.Error())
			}
		}
		for _, a := range app.ServiceLabel {
			a.ID = 0
			if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app labels when restore backup error. %s", err.Error())
			}
		}
		for _, a := range app.ServiceMntRelation {
			a.ID = 0
			if err := db.GetManager().TenantServiceMountRelationDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app mount relation when restore backup error. %s", err.Error())
			}
		}
		//TODO: support service plugin backup and restore
		// for _, a := range app.PluginRelation {
		// 	a.ID = 0
		// 	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).AddModel(a); err != nil {
		// 		tx.Rollback()
		// 		return fmt.Errorf("create app plugin when restore backup error. %s", err.Error())
		// 	}
		// }
		for _, a := range app.ServiceRelation {
			a.ID = 0
			if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app relation when restore backup error. %s", err.Error())
			}
		}
		localPath := os.Getenv("LOCAL_DATA_PATH")
		sharePath := os.Getenv("SHARE_DATA_PATH")
		if localPath == "" {
			localPath = "/grlocaldata"
		}
		if sharePath == "" {
			sharePath = "/grdata"
		}
		for _, a := range app.ServiceVolume {
			a.ID = 0
			switch a.VolumeType {
			//nfs
			case dbmodel.ShareFileVolumeType.String():
				a.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, b.TenantID, a.ServiceID, a.VolumePath)
			//local
			case dbmodel.LocalVolumeType.String():
				a.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, b.TenantID, a.ServiceID, a.VolumePath)
			}
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app volume when restore backup error. %s", err.Error())
			}
		}
		for _, a := range app.ServicePort {
			a.ID = 0
			if err := db.GetManager().TenantServicesPortDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app ports when restore backup error. %s", err.Error())
			}
		}
		for _, a := range app.Versions {
			a.ID = 0
			if err := db.GetManager().VersionInfoDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app versions when restore backup error. %s", err.Error())
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (b *BackupAPPRestore) downloadFromLocal(backup *dbmodel.AppBackup) error {
	sourceDir := backup.SourceDir
	err := util.Unzip(sourceDir, b.cacheDir)
	if err != nil {
		b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("unzip file error when restore backup app , %s", err.Error())
		return err
	}
	dirs, err := util.GetDirNameList(b.cacheDir, 1)
	if err != nil || len(dirs) < 1 {
		b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
		return fmt.Errorf("find metadata cache dir error after unzip file")
	}
	b.cacheDir = filepath.Join(b.cacheDir, dirs[0])
	return nil
}

func (b *BackupAPPRestore) downloadFromFTP(backup *dbmodel.AppBackup) error {
	sourceDir := backup.SourceDir
	sFTPClient, err := sources.NewSFTPClient(b.SlugInfo.FTPUser, b.SlugInfo.FTPPassword, b.SlugInfo.FTPHost, b.SlugInfo.FTPPort)
	if err != nil {
		b.Logger.Error(util.Translation("create ftp client error"), map[string]string{"step": "backup_builder", "status": "failure"})
		return err
	}
	defer sFTPClient.Close()
	dstDir := fmt.Sprintf("%s/%s", b.cacheDir, filepath.Base(sourceDir))
	if err := sFTPClient.DownloadFile(sourceDir, dstDir, b.Logger); err != nil {
		b.Logger.Error(util.Translation("down slug file from sftp server error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("down  slug file error when restore backup app , %s", err.Error())
		return err
	}
	err = util.Unzip(dstDir, b.cacheDir)
	if err != nil {
		b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("unzip file error when restore backup app , %s", err.Error())
		return err
	}
	dirs, err := util.GetDirNameList(b.cacheDir, 1)
	if err != nil || len(dirs) < 1 {
		b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
		return fmt.Errorf("find metadata cache dir error after unzip file")
	}
	b.cacheDir = filepath.Join(b.cacheDir, dirs[0])
	return nil
}

//Stop stop
func (b *BackupAPPRestore) Stop() error {
	return nil
}

//Name return worker name
func (b *BackupAPPRestore) Name() string {
	return "backup_apps_restore"
}

//GetLogger GetLogger
func (b *BackupAPPRestore) GetLogger() event.Logger {
	return b.Logger
}

//ErrorCallBack if run error will callback
func (b *BackupAPPRestore) ErrorCallBack(err error) {
	if err != nil {
		logrus.Errorf("restore backup group app failure %s", err)
		b.Logger.Error(util.Translation("restore backup group app failure"), map[string]string{"step": "callback", "status": "failure"})
		b.clear()
		b.saveResult("failed", err.Error())
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
	CacheDir      string           `json:"cache_dir"`
}

func (b *BackupAPPRestore) saveResult(status, message string) {
	var rr = RestoreResult{
		Status:        status,
		Message:       message,
		CreateTime:    time.Now(),
		ServiceChange: b.serviceChange,
		BackupID:      b.BackupID,
		RestoreMode:   b.RestoreMode,
		EventID:       b.EventID,
		RestoreID:     b.RestoreID,
		CacheDir:      b.cacheDir,
	}
	body, _ := ffjson.Marshal(rr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.etcdcli.Put(ctx, "/rainbond/backup_restore/"+rr.RestoreID, string(body))
}
