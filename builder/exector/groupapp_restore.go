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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/cloudos"
	"github.com/goodrain/rainbond/builder/parser"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/errors"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

//BackupAPPRestore restrore the  group app backup
type BackupAPPRestore struct {
	//full-online,full-offline
	EventID  string
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
	volumeIDMap   map[uint]uint
	etcdcli       *clientv3.Client

	S3Config struct {
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		AccessKey  string `json:"access_key"`
		SecretKey  string `json:"secret_key"`
		BucketName string `json:"bucket_name"`
	} `json:"s3_config"`
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
		volumeIDMap:   make(map[uint]uint),
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
	if backup.Status != "success" || backup.SourceDir == "" || backup.BackupMode == "" {
		return fmt.Errorf("backup can not be restore")
	}

	cacheDir := fmt.Sprintf("/grdata/cache/tmp/%s/%s", b.BackupID, util.NewUUID())
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		return fmt.Errorf("create cache dir error %s", err.Error())
	}
	// delete the cache data
	defer b.deleteCache(cacheDir)

	b.cacheDir = cacheDir
	switch backup.BackupMode {
	case "full-online":
		if err := b.downloadFromS3(backup.SourceDir); err != nil {
			return fmt.Errorf("error downloading file from s3: %v", err)
		}
	default:
		b.downloadFromLocal(backup)
	}

	//read metadata file
	metadata, err := ioutil.ReadFile(path.Join(b.cacheDir, "region_apps_metadata.json"))
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	metaVersion, err := judgeMetadataVersion(metadata)
	if err != nil {
		b.Logger.Info(fmt.Sprintf("Failed to judge the version of metadata"), map[string]string{"step": "backup_builder", "status": "failure"})
		return err
	}

	var appSnapshot AppSnapshot
	var svcSnapshot []*RegionServiceSnapshot
	if metaVersion == OldMetadata {
		if err := ffjson.Unmarshal(metadata, &svcSnapshot); err != nil {
			return err
		}
		appSnapshot = AppSnapshot{
			Services: svcSnapshot,
		}
	} else {
		if err := ffjson.Unmarshal(metadata, &appSnapshot); err != nil {
			return err
		}
	}

	b.Logger.Info("读取备份元数据完成", map[string]string{"step": "restore_builder", "status": "running"})
	logrus.Infof("backup id: %s; successfully read metadata.", b.BackupID)
	//modify the metadata
	if err := b.modify(&appSnapshot); err != nil {
		return err
	}
	//restore metadata to db
	if err := b.restoreMetadata(&appSnapshot); err != nil {
		return err
	}
	b.Logger.Info("恢复备份元数据完成", map[string]string{"step": "restore_builder", "status": "success"})
	logrus.Infof("backup id: %s; successfully restore metadata.", b.BackupID)
	//If the following error occurs, delete the data from the database
	//restore all app all build version and data
	if err := b.restoreVersionAndData(backup, &appSnapshot); err != nil {
		return err
	}

	//save result
	b.saveResult("success", "")
	logrus.Infof("backup id: %s; successfully restore backup.", b.BackupID)
	b.Logger.Info("恢复成功", map[string]string{"step": "restore_builder", "status": "success"})
	return nil
}

func (b *BackupAPPRestore) deleteCache(dir string) error {
	logrus.Infof("delete cache %s", dir)
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// do not delete the metadata file
		if strings.HasSuffix(path, "console_apps_metadata.json") {
			return nil
		}
		return os.RemoveAll(path)
	})
}

func (b *BackupAPPRestore) restoreVersionAndData(backup *dbmodel.AppBackup, appSnapshot *AppSnapshot) error {
	for _, app := range appSnapshot.Services {
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
		b.Logger.Info(fmt.Sprintf("完成恢复应用(%s)运行环境", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "running"})

		b.Logger.Info(fmt.Sprintf("开始恢复应用(%s)持久化数据", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "starting"})
		//restore app data

		//if all data backup file exist, restore all data directly
		allDataFilePath := fmt.Sprintf("%s/data_%s/%s.zip", b.cacheDir, b.getOldServiceID(app.ServiceID), "__all_data")
		allDataRestore := false
		allTmpDir := fmt.Sprintf("/grdata/tmp/%s", app.ServiceID)
		if exist, _ := util.FileExists(allDataFilePath); exist {
			logrus.Infof("unzip all data from %s to %s", allDataFilePath, allTmpDir)
			if err := util.Unzip(allDataFilePath, allTmpDir); err != nil {
				logrus.Errorf("unzip all data file failure %s", err.Error())
			} else {
				allDataRestore = true
			}
		}
		for _, volume := range app.ServiceVolume {
			if volume.HostPath == "" {
				continue
			}
			var tmpDir string
			if !allDataRestore {
				dstDir := fmt.Sprintf("%s/data_%s/%s.zip", b.cacheDir, b.getOldServiceID(app.ServiceID), strings.Replace(volume.VolumeName, "/", "", -1))
				tmpDir = fmt.Sprintf("/grdata/tmp/%s_%d", volume.ServiceID, volume.ID)
				logrus.Infof("unzip %s to %s", dstDir, tmpDir)
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
			} else {
				tmpDir = path.Join(allTmpDir, b.getOldServiceID(app.ServiceID))
			}

			//if app type is statefulset, change pod hostpath
			if app.Service.IsState() {
				logrus.Infof("app type is statefulset, change pod hostpath %s. tmp dir: %s", volume.HostPath, tmpDir)
				//Next two level directory
				list, err := util.GetDirList(path.Join(tmpDir, volume.VolumePath), 1)
				if err != nil {
					logrus.Errorf("restore statefulset service(%s) volume(%s) data error.%s", app.ServiceID, volume.VolumeName, err.Error())
					return err
				}
				for _, path := range list {
					logrus.Infof("handle path %s", path)
					newNameTmp := strings.Split(filepath.Base(path), "-")
					// after version 5.0.4, path name is pod name. eg gr123456-0
					if len(newNameTmp) == 2 {
						newNameTmp[0] = b.serviceChange[b.getOldServiceID(app.ServiceID)].ServiceAlias
					}
					// before version 5.0.4, path name is pvc name, eg manual16-grcaa708-0
					if len(newNameTmp) == 3 {
						newNameTmp[1] = b.serviceChange[b.getOldServiceID(app.ServiceID)].ServiceAlias
						oldVolumeID, _ := strconv.Atoi(newNameTmp[0][6:])
						if oldVolumeID > 0 {
							newNameTmp[0] = fmt.Sprintf("manual%d", b.volumeIDMap[uint(oldVolumeID)])
						}
					}
					newName := strings.Join(newNameTmp, "-")
					newpath := filepath.Join(util.GetParentDirectory(path), newName)
					logrus.Infof("rename %s to %s", path, newpath)
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
			if !allDataRestore {
				logrus.Infof("rename parent directory from %s to %s", tmpDir, util.GetParentDirectory(volume.HostPath))
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

		if allDataRestore {
			dst := fmt.Sprintf("/grdata/tenant/%s/service/%s", app.Service.TenantID, app.Service.ServiceID)
			err := util.Rename(path.Join(allTmpDir, b.getOldServiceID(app.ServiceID)), dst)
			if err != nil {
				logrus.Errorf("rename %s to %s failure %s", path.Join(allTmpDir, b.getOldServiceID(app.ServiceID)), fmt.Sprintf("/grdata/tenant/%s/service/%s", app.Service.TenantID, app.Service.ServiceID), err.Error())
			}
		}
		b.Logger.Info(fmt.Sprintf("完成恢复应用(%s)持久化数据", app.Service.ServiceAlias), map[string]string{"step": "restore_builder", "status": "running"})
		//TODO:relation relation volume data?
	}

	if len(appSnapshot.PluginBuildVersions) == 0 {
		return nil
	}

	// restore plugin image
	for _, pb := range appSnapshot.PluginBuildVersions {
		dstDir := fmt.Sprintf("%s/plugin_%s/image_%s.tar", b.cacheDir, pb.PluginID, pb.DeployVersion)
		if err := sources.ImageLoad(b.DockerClient, dstDir, b.Logger); err != nil {
			b.Logger.Error(util.Translation("load image to local hub error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("dst: %s; failed to load plugin image: %v", dstDir, err)
			return err
		}
		imageName := getNewImageName(pb.BuildLocalImage)
		if imageName != "" {
			if err := sources.ImagePush(b.DockerClient, imageName, builder.REGISTRYUSER, builder.REGISTRYPASS, b.Logger, 30); err != nil {
				b.Logger.Error("push plugin image failure", map[string]string{"step": "restore_builder", "status": "failure"})
				logrus.Errorf("failure push image %s: %v", imageName, err)
				return err
			}
		}
	}
	b.Logger.Info("完成恢复插件镜像", map[string]string{"step": "restore_builder", "status": "running"})

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
	dstDir := fmt.Sprintf("%s/app_%s/slug_%s.tgz", b.cacheDir, b.getOldServiceID(app.ServiceID), version.BuildVersion)
	if err := sources.CopyFileWithProgress(dstDir, version.DeliveredPath, b.Logger); err != nil {
		b.Logger.Error(util.Translation("down slug file from local dir error"), map[string]string{"step": "restore_builder", "status": "failure"})
		logrus.Errorf("copy slug file error when backup app, %s", err.Error())
		return err
	}
	return nil
}

func (b *BackupAPPRestore) downloadImage(backup *dbmodel.AppBackup, app *RegionServiceSnapshot, version *dbmodel.VersionInfo) error {
	dstDir := fmt.Sprintf("%s/app_%s/image_%s.tar", b.cacheDir, b.getOldServiceID(app.ServiceID), version.BuildVersion)
	if err := sources.ImageLoad(b.DockerClient, dstDir, b.Logger); err != nil {
		b.Logger.Error(util.Translation("load image to local hub error"), map[string]string{"step": "restore_builder", "status": "failure"})
		logrus.Errorf("load image to local hub error when restore backup app, %s", err.Error())
		return err
	}
	imageName := version.ImageName
	if imageName == "" {
		imageName = version.DeliveredPath
	}
	newImageName := getNewImageName(imageName)
	if newImageName != imageName {
		if err := sources.ImageTag(b.DockerClient, imageName, newImageName, b.Logger, 3); err != nil {
			b.Logger.Error(util.Translation("change image tag error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("change image tag %s to %s failure, %s", imageName, newImageName, err.Error())
			return err
		}
		imageName = newImageName
	}
	if imageName != "" {
		if err := sources.ImagePush(b.DockerClient, imageName, builder.REGISTRYUSER, builder.REGISTRYPASS, b.Logger, 30); err != nil {
			b.Logger.Error(util.Translation("push image to local hub error"), map[string]string{"step": "restore_builder", "status": "failure"})
			logrus.Errorf("push image to local hub error when restore backup app, %s", err.Error())
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
func getNewImageName(imageName string) string {
	image := parser.ParseImageName(imageName)
	if image.GetDomain() != builder.REGISTRYDOMAIN {
		newImageName := strings.Replace(imageName, image.GetDomain(), builder.REGISTRYDOMAIN, 1)
		return newImageName
	}
	return imageName
}
func (b *BackupAPPRestore) modify(appSnapshot *AppSnapshot) error {
	for _, app := range appSnapshot.Services {
		oldServiceID := app.ServiceID
		//compatible component type
		switch app.Service.ExtendMethod {
		case "state":
			app.Service.ExtendMethod = dbmodel.ServiceTypeStateMultiple.String()
		case "stateless":
			app.Service.ExtendMethod = dbmodel.ServiceTypeStatelessMultiple.String()
		}
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

		//change service_id and service_alias
		newServiceID := util.NewUUID()
		newServiceAlias := "gr" + newServiceID[26:]
		app.ServiceID = newServiceID
		app.Service.ServiceID = newServiceID
		app.Service.ServiceAlias = newServiceAlias
		app.Service.ServiceName = newServiceAlias
		for _, a := range app.ServiceProbe {
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
		for _, a := range app.ServiceRelation {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceVolume {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServiceConfigFile {
			a.ServiceID = newServiceID
		}
		for _, a := range app.ServicePort {
			a.ServiceID = newServiceID
		}
		for _, a := range app.Versions {
			if a.DeliveredType == "image" {
				a.ImageName = getNewImageName(a.ImageName)
				a.DeliveredPath = getNewImageName(a.DeliveredPath)
			}
			a.ServiceID = newServiceID
		}

		// plugin
		for _, a := range app.PluginRelation {
			a.ServiceID = newServiceID
		}
		for _, a := range app.PluginConfigs {
			a.ServiceID = newServiceID
		}
		for _, a := range app.PluginEnvs {
			a.ServiceID = newServiceID
		}
		for _, a := range app.PluginStreamPorts {
			a.ServiceID = newServiceID
		}
		// TODO: change service info in plugin config

		b.serviceChange[oldServiceID] = &Info{
			ServiceID:    newServiceID,
			ServiceAlias: newServiceAlias,
			Status:       app.ServiceStatus,
		}
	}
	//modify relations
	for _, app := range appSnapshot.Services {
		for _, a := range app.ServiceMntRelation {
			info := b.serviceChange[a.DependServiceID]
			if info != nil {
				a.DependServiceID = info.ServiceID
			}
		}
		for _, a := range app.ServiceRelation {
			info := b.serviceChange[a.DependServiceID]
			if info != nil {
				a.DependServiceID = info.ServiceID
			}
		}
	}

	// plugin
	for _, p := range appSnapshot.Plugins {
		p.TenantID = b.TenantID
	}

	return nil
}
func (b *BackupAPPRestore) restoreMetadata(appSnapshot *AppSnapshot) error {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	for _, app := range appSnapshot.Services {
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
		for _, a := range app.ServiceRelation {
			a.ID = 0
			if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app relation when restore backup error. %s", err.Error())
			}
		}
		localPath, sharePath := GetVolumeDir()
		for _, a := range app.ServiceVolume {
			oldVolumeID := a.ID
			a.ID = 0
			switch a.VolumeType {
			//nfs
			case dbmodel.ShareFileVolumeType.String():
				a.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, b.TenantID, a.ServiceID, a.VolumePath)
			//local
			case dbmodel.LocalVolumeType.String():
				a.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, b.TenantID, a.ServiceID, a.VolumePath)
			case dbmodel.MemoryFSVolumeType.String(), dbmodel.ConfigFileVolumeType.String():
				logrus.Debugf("simple volume type: %s", a.VolumeType)
			default:
				logrus.Warnf("custom volumeType: %s", a.VolumeType)
				volumeType, err := db.GetManager().VolumeTypeDao().GetVolumeTypeByType(a.VolumeType)
				if err != nil {
					logrus.Warnf("get volumeType[%s] error : %s, use share-file instead", a.VolumeType, err.Error())
				}
				if volumeType == nil {
					logrus.Warnf("service[%s] volumeType[%s] do not exists, use default volumeType[%s]", a.ServiceID, a.VolumeType, dbmodel.ShareFileVolumeType.String())
					a.VolumeType = dbmodel.ShareFileVolumeType.String()
					a.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, b.TenantID, a.ServiceID, a.VolumePath)
				}
			}
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app volume when restore backup error. %s", err.Error())
			}
			b.volumeIDMap[oldVolumeID] = a.ID
		}
		for _, a := range app.ServiceConfigFile {
			a.ID = 0
			if err := db.GetManager().TenantServiceConfigFileDao().AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("create app config file when restore backup errro. %s", err.Error())
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
		// plugin info
		for _, a := range app.PluginRelation {
			a.ID = 0
			if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).AddModel(a); err != nil {
				tx.Rollback()
				return fmt.Errorf("error creating plugin relation: %v", err)
			}
		}
		for _, pc := range app.PluginConfigs {
			pc.ID = 0
			if err := db.GetManager().TenantPluginVersionConfigDaoTransactions(tx).AddModel(pc); err != nil {
				tx.Rollback()
				return fmt.Errorf("error creating plugin config: %v", err)
			}
		}
		for _, pe := range app.PluginEnvs {
			pe.ID = 0
			if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).AddModel(pe); err != nil {
				tx.Rollback()
				return fmt.Errorf("error creating plugin version env: %v", err)
			}
		}
		for _, psp := range app.PluginStreamPorts {
			psp.ID = 0
			if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).AddModel(psp); err != nil {
				tx.Rollback()
				return fmt.Errorf("error creating plugin stream port: %v", err)
			}
		}
	}

	for _, p := range appSnapshot.Plugins {
		p.ID = 0
		if err := db.GetManager().TenantPluginDaoTransactions(tx).AddModel(p); err != nil {
			if err == errors.ErrRecordAlreadyExist {
				continue
			}
			tx.Rollback()
			return fmt.Errorf("error creating plugin: %v", err)
		}
	}
	for _, p := range appSnapshot.PluginBuildVersions {
		p.ID = 0
		p.BuildLocalImage = getNewImageName(p.BuildLocalImage)
		if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).AddModel(p); err != nil {
			if err == errors.ErrRecordAlreadyExist {
				continue
			}
			tx.Rollback()
			return fmt.Errorf("error creating plugin build version: %v", err)
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

func (b *BackupAPPRestore) downloadFromS3(sourceDir string) error {
	s3Provider, err := cloudos.Str2S3Provider(b.S3Config.Provider)
	if err != nil {
		return err
	}
	cfg := &cloudos.Config{
		ProviderType: s3Provider,
		Endpoint:     b.S3Config.Endpoint,
		AccessKey:    b.S3Config.AccessKey,
		SecretKey:    b.S3Config.SecretKey,
		BucketName:   b.S3Config.BucketName,
	}
	cloudoser, err := cloudos.New(cfg)
	if err != nil {
		return fmt.Errorf("error creating cloudoser: %v", err)
	}

	_, objectKey := filepath.Split(sourceDir)
	disDir := path.Join(b.cacheDir, objectKey)
	logrus.Debugf("object key: %s; file path: %s; start downloading backup file.", objectKey, disDir)
	if err := cloudoser.GetObject(objectKey, disDir); err != nil {
		return fmt.Errorf("object key: %s; file path: %s; error downloading file for object storage: %v", objectKey, disDir, err)
	}
	logrus.Debugf("successfully downloading backup file: %s", disDir)

	err = util.Unzip(disDir, b.cacheDir)
	if err != nil {
		// b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("error unzipping backup file: %v", err)
		return err
	}

	dirs, err := util.GetDirNameList(b.cacheDir, 1)
	if err != nil || len(dirs) < 1 {
		// b.Logger.Error(util.Translation("unzip metadata file error"), map[string]string{"step": "backup_builder", "status": "failure"})
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
