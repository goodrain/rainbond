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
	"fmt"
	"github.com/goodrain/rainbond/builder"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/db"

	"github.com/goodrain/rainbond/builder/cloudos"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	// OldMetadata identify older versions of metadata
	OldMetadata = "OldMetadata"
	// NewMetadata identify new version of metadata
	NewMetadata = "NewMetadata"
)

//maxBackupVersionSize Maximum number of backup versions per service
var maxBackupVersionSize = 3

//BackupAPPNew backup group app new version
type BackupAPPNew struct {
	GroupID    string   `json:"group_id" `
	ServiceIDs []string `json:"service_ids" `
	Version    string   `json:"version"`
	EventID    string
	SourceDir  string `json:"source_dir"`
	SourceType string `json:"source_type"`
	BackupID   string `json:"backup_id"`
	BackupSize int64
	Logger     event.Logger
	//DockerClient *client.Client
	ImageClient sources.ImageClient
	//ContainerdClient *containerd.Client
	//full-online,full-offline
	Mode     string `json:"mode"`
	S3Config struct {
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		AccessKey  string `json:"access_key"`
		SecretKey  string `json:"secret_key"`
		BucketName string `json:"bucket_name"`
	} `json:"s3_config"`
}

func init() {
	RegisterWorker("backup_apps_new", BackupAPPNewCreater)
}

//BackupAPPNewCreater create
func BackupAPPNewCreater(in []byte, m *exectorManager) (TaskWorker, error) {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	backupNew := &BackupAPPNew{
		Logger:  logger,
		EventID: eventID,
		//DockerClient: m.DockerClient,
		ImageClient: m.imageClient,
	}
	if err := ffjson.Unmarshal(in, &backupNew); err != nil {
		return nil, err
	}
	return backupNew, nil
}

// AppSnapshot holds a snapshot of your app
type AppSnapshot struct {
	Services            []*RegionServiceSnapshot
	Plugins             []*dbmodel.TenantPlugin
	PluginBuildVersions []*dbmodel.TenantPluginBuildVersion
}

//RegionServiceSnapshot RegionServiceSnapshot
type RegionServiceSnapshot struct {
	ServiceID          string
	Service            *dbmodel.TenantServices
	ServiceProbe       []*dbmodel.TenantServiceProbe
	LBMappingPort      []*dbmodel.TenantServiceLBMappingPort
	ServiceEnv         []*dbmodel.TenantServiceEnvVar
	ServiceLabel       []*dbmodel.TenantServiceLable
	ServiceMntRelation []*dbmodel.TenantServiceMountRelation
	ServiceRelation    []*dbmodel.TenantServiceRelation
	ServiceStatus      string
	ServiceVolume      []*dbmodel.TenantServiceVolume
	ServiceConfigFile  []*dbmodel.TenantServiceConfigFile
	ServicePort        []*dbmodel.TenantServicesPort
	Versions           []*dbmodel.VersionInfo

	PluginRelation    []*dbmodel.TenantServicePluginRelation
	PluginConfigs     []*dbmodel.TenantPluginVersionDiscoverConfig
	PluginEnvs        []*dbmodel.TenantPluginVersionEnv
	PluginStreamPorts []*dbmodel.TenantServicesStreamPluginPort
}

//Run Run
func (b *BackupAPPNew) Run(timeout time.Duration) error {
	//read region group app metadata
	metadata, err := ioutil.ReadFile(fmt.Sprintf("%s/region_apps_metadata.json", b.SourceDir))
	if err != nil {
		return err
	}

	metaVersion, err := judgeMetadataVersion(metadata)
	if err != nil {
		b.Logger.Info(fmt.Sprintf("Failed to judge the version of metadata"), map[string]string{"step": "backup_builder", "status": "failure"})
		return err
	}
	if metaVersion == OldMetadata {
		var svcSnapshot []*RegionServiceSnapshot
		if err := ffjson.Unmarshal(metadata, &svcSnapshot); err != nil {
			b.Logger.Info(fmt.Sprintf("Failed to unmarshal metadata into RegionServiceSnapshot"), map[string]string{"step": "backup_builder", "status": "failure"})
			return err
		}
		if err := b.backupServiceInfo(svcSnapshot); err != nil {
			b.Logger.Info(fmt.Sprintf("Failed to backup metadata service info"), map[string]string{"step": "backup_builder", "status": "failure"})
			return err
		}
	} else {
		var appSnapshot AppSnapshot
		if err := ffjson.Unmarshal(metadata, &appSnapshot); err != nil {
			b.Logger.Info(fmt.Sprintf("Failed to unmarshal metadata into AppSnapshot"), map[string]string{"step": "backup_builder", "status": "failure"})
			return err
		}
		if err := b.backupServiceInfo(appSnapshot.Services); err != nil {
			b.Logger.Info(fmt.Sprintf("Failed to backup metadata service info"), map[string]string{"step": "backup_builder", "status": "failure"})
			return err
		}
		if err := b.backupPluginInfo(&appSnapshot); err != nil {
			b.Logger.Info(fmt.Sprintf("Failed to backup metadata plugin info"), map[string]string{"step": "backup_builder", "status": "failure"})
			return err
		}
	}

	if strings.HasSuffix(b.SourceDir, "/") {
		b.SourceDir = b.SourceDir[:len(b.SourceDir)-2]
	}
	if err := util.Zip(b.SourceDir, fmt.Sprintf("%s.zip", b.SourceDir)); err != nil {
		b.Logger.Info(fmt.Sprintf("Compressed backup metadata failed"), map[string]string{"step": "backup_builder", "status": "starting"})
		return err
	}
	b.BackupSize += util.GetFileSize(fmt.Sprintf("%s.zip", b.SourceDir))
	if err := os.RemoveAll(b.SourceDir); err != nil {
		logrus.Warningf("error removing temporary direcotry: %v", err)
	}
	b.SourceDir = fmt.Sprintf("%s.zip", b.SourceDir)

	if err := b.uploadPkg(); err != nil {
		return fmt.Errorf("error upload backup package: %v", err)
	}

	if err := b.updateBackupStatu("success"); err != nil {
		return err
	}
	return nil
}

func (b *BackupAPPNew) uploadPkg() error {
	if b.Mode != "full-online" {
		return nil
	}

	defer func() {
		if err := os.Remove(b.SourceDir); err != nil {
			logrus.Warningf("error removing temporary file: %v", err)
		}
	}()

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
	_, filename := filepath.Split(b.SourceDir)
	if err := cloudoser.PutObject(filename, b.SourceDir); err != nil {
		return fmt.Errorf("object key: %s; filepath: %s; error putting object: %v", filename, b.SourceDir, err)
	}
	return nil
}

// judging whether the metadata structure is old or new, the new version is v5.1.8 and later
func judgeMetadataVersion(metadata []byte) (string, error) {
	var appSnapshot AppSnapshot
	if err := ffjson.Unmarshal(metadata, &appSnapshot); err == nil {
		return NewMetadata, nil
	}

	var svcSnapshot []*RegionServiceSnapshot
	if err := ffjson.Unmarshal(metadata, &svcSnapshot); err == nil {
		return "", err
	}

	return OldMetadata, nil
}

func (b *BackupAPPNew) backupServiceInfo(serviceInfos []*RegionServiceSnapshot) error {
	for _, app := range serviceInfos {
		//backup app image or code slug file
		b.Logger.Info(fmt.Sprintf("Start backup Application(%s) runtime", app.Service.ServiceAlias), map[string]string{"step": "backup_builder", "status": "starting"})
		if len(app.Versions) > 0 {
			var backupVersionSize int
			for _, version := range app.Versions {
				if backupVersionSize >= maxBackupVersionSize {
					break
				}
				if version.DeliveredType == "slug" && version.FinalStatus == "success" {
					if ok, _ := b.checkVersionExist(version); !ok {
						version.FinalStatus = "lost"
						continue
					}
					if err := b.saveSlugPkg(app, version); err != nil {
						logrus.Errorf("upload app %s version %s slug file error.%s", app.Service.ServiceName, version.BuildVersion, err.Error())
					} else {
						backupVersionSize++
					}
				}
				if version.DeliveredType == "image" && version.FinalStatus == "success" {
					if ok, _ := b.checkVersionExist(version); !ok {
						version.FinalStatus = "lost"
						continue
					}
					if err := b.saveImagePkg(app, version); err != nil {
						logrus.Errorf("upload app %s version %s image error.%s", app.Service.ServiceName, version.BuildVersion, err.Error())
					} else {
						backupVersionSize++
					}
				}
			}
			if backupVersionSize == 0 {
				b.Logger.Error(fmt.Sprintf("Application(%s) Backup build version failure.", app.Service.ServiceAlias), map[string]string{"step": "backup_builder", "status": "success"})
				return fmt.Errorf("Application(%s) Backup build version failure", app.Service.ServiceAlias)
			}
			logrus.Infof("backup app %s %d version", app.Service.ServiceName, backupVersionSize)
			b.Logger.Info(fmt.Sprintf("Complete backup application (%s) runtime %d version", app.Service.ServiceAlias, backupVersionSize), map[string]string{"step": "backup_builder", "status": "success"})
		}

		b.Logger.Info(fmt.Sprintf("Start backup application(%s) persistent data", app.Service.ServiceAlias), map[string]string{"step": "backup_builder", "status": "starting"})
		//backup app data,The overall data of the direct backup service
		if len(app.ServiceVolume) > 0 {
			dstDir := fmt.Sprintf("%s/data_%s/%s.zip", b.SourceDir, app.Service.ServiceID, "__all_data")
			_, sharepath := GetVolumeDir()
			serviceVolumeData := path.Join(sharepath, "tenant", app.Service.TenantID, "service", app.Service.ServiceID)
			if !util.DirIsEmpty(serviceVolumeData) {
				if err := util.Zip(serviceVolumeData, dstDir); err != nil {
					logrus.Errorf("backup service(%s) volume data error.%s", app.ServiceID, err.Error())
					return err
				}
			}
		}
		for _, volume := range app.ServiceVolume {
			dstDir := fmt.Sprintf("%s/data_%s/%s.zip", b.SourceDir, app.ServiceID, strings.Replace(volume.VolumeName, "/", "", -1))
			hostPath := volume.HostPath
			if hostPath != "" && !util.DirIsEmpty(hostPath) {
				if err := util.Zip(hostPath, dstDir); err != nil {
					logrus.Errorf("backup service(%s) volume(%s) data error.%s", app.ServiceID, volume.VolumeName, err.Error())
					return err
				}
			}
		}
		b.Logger.Info(fmt.Sprintf("Complete backup application(%s) persistent data", app.Service.ServiceAlias), map[string]string{"step": "backup_builder", "status": "success"})
	}
	return nil
}

func (b *BackupAPPNew) backupPluginInfo(appSnapshot *AppSnapshot) error {
	b.Logger.Info(fmt.Sprintf("Start backup plugin"), map[string]string{"step": "backup_builder", "status": "starting"})
	for _, pv := range appSnapshot.PluginBuildVersions {
		dstDir := fmt.Sprintf("%s/plugin_%s/image_%s.tar", b.SourceDir, pv.PluginID, pv.DeployVersion)
		util.CheckAndCreateDir(filepath.Dir(dstDir))
		if _, err := b.ImageClient.ImagePull(pv.BuildLocalImage, builder.REGISTRYUSER, builder.REGISTRYPASS, b.Logger, 20); err != nil {
			b.Logger.Error(fmt.Sprintf("plugin image: %s; failed to pull image", pv.BuildLocalImage), map[string]string{"step": "backup_builder", "status": "failure"})
			logrus.Errorf("plugin image: %s; failed to pull image: %v", pv.BuildLocalImage, err)
			return err
		}
		if err := b.ImageClient.ImageSave(pv.BuildLocalImage, dstDir); err != nil {
			b.Logger.Error(util.Translation("save image to local dir error"), map[string]string{"step": "backup_builder", "status": "failure"})
			logrus.Errorf("plugin image: %s; failed to save image: %v", pv.BuildLocalImage, err)
			return err
		}
	}
	return nil
}

func (b *BackupAPPNew) checkVersionExist(version *dbmodel.VersionInfo) (bool, error) {
	if version.DeliveredType == "image" {
		imageInfo := sources.ImageNameHandle(version.DeliveredPath)
		reg, err := registry.NewInsecure(imageInfo.Host, builder.REGISTRYUSER, builder.REGISTRYPASS)
		if err != nil {
			logrus.Errorf("new registry client error %s", err.Error())
			return false, err
		}
		_, err = reg.Manifest(imageInfo.Name, imageInfo.Tag)
		if err != nil {
			logrus.Errorf("get image [%s] manifest info failure [%v], it could be not exist", version.DeliveredPath, err)
			// Compatible with MediaTypeManifest
			_, err := reg.ManifestV2(imageInfo.Name, imageInfo.Tag)
			if err != nil {
				logrus.Errorf("get image [%s] manifestV2 info failure [%v], it could be not exist", version.DeliveredPath, err)
				return false, err
			}
			return true, nil
		}
		return true, nil
	}
	if version.DeliveredType == "slug" {
		islugfile, err := os.Stat(version.DeliveredPath)
		if os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		if islugfile.IsDir() {
			return false, nil
		}
		return true, nil
	}
	return false, fmt.Errorf("delivered type is invalid")
}

// saveSlugPkg saves slug package on disk.
func (b *BackupAPPNew) saveSlugPkg(app *RegionServiceSnapshot, version *dbmodel.VersionInfo) error {
	dstDir := fmt.Sprintf("%s/app_%s/slug_%s.tgz", b.SourceDir, app.ServiceID, version.BuildVersion)
	util.CheckAndCreateDir(filepath.Dir(dstDir))
	if err := sources.CopyFileWithProgress(version.DeliveredPath, dstDir, b.Logger); err != nil {
		b.Logger.Error(util.Translation("push slug file to local dir error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("copy slug file error when backup app, %s", err.Error())
		return err
	}
	return nil
}

// saveSlugPkg saves image package on disk.
func (b *BackupAPPNew) saveImagePkg(app *RegionServiceSnapshot, version *dbmodel.VersionInfo) error {
	dstDir := fmt.Sprintf("%s/app_%s/image_%s.tar", b.SourceDir, app.ServiceID, version.BuildVersion)
	util.CheckAndCreateDir(filepath.Dir(dstDir))
	if _, err := b.ImageClient.ImagePull(version.DeliveredPath, builder.REGISTRYUSER, builder.REGISTRYPASS, b.Logger, 20); err != nil {
		b.Logger.Error(util.Translation("error pulling image"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf(fmt.Sprintf("image: %s; error pulling image: %v", version.DeliveredPath, err), version.DeliveredPath, err.Error())
	}
	if err := b.ImageClient.ImageSave(version.DeliveredPath, dstDir); err != nil {
		b.Logger.Error(util.Translation("save image to local dir error"), map[string]string{"step": "backup_builder", "status": "failure"})
		logrus.Errorf("save image(%s) to local dir error when backup app, %s", version.DeliveredPath, err.Error())
		return err
	}
	return nil
}

//Stop stop
func (b *BackupAPPNew) Stop() error {
	return nil
}

//Name return worker name
func (b *BackupAPPNew) Name() string {
	return "backup_apps_new"
}

//GetLogger GetLogger
func (b *BackupAPPNew) GetLogger() event.Logger {
	return b.Logger
}

//ErrorCallBack if run error will callback
func (b *BackupAPPNew) ErrorCallBack(err error) {
	if err != nil {
		logrus.Errorf("backup group app failure %s", err)
		b.Logger.Error(util.Translation("backup group app failure"), map[string]string{"step": "callback", "status": "failure"})
		b.updateBackupStatu("failed")
	}
}

func (b *BackupAPPNew) updateBackupStatu(status string) error {
	backupstatus, err := db.GetManager().AppBackupDao().GetAppBackup(b.BackupID)
	if err != nil {
		logrus.Errorf("update backup group app history failure %s", err)
		return err
	}
	backupstatus.Status = status
	backupstatus.SourceDir = b.SourceDir
	backupstatus.SourceType = b.SourceType
	backupstatus.BuckupSize = b.BackupSize
	return db.GetManager().AppBackupDao().UpdateModel(backupstatus)
}

//GetVolumeDir get volume path prifix
func GetVolumeDir() (string, string) {
	localPath := os.Getenv("LOCAL_DATA_PATH")
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if localPath == "" {
		localPath = "/grlocaldata"
	}
	if sharePath == "" {
		sharePath = "/grdata"
	}
	return localPath, sharePath
}
