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
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/builder/cloudos"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
)

// BackupAPPDelete -
type BackupAPPDelete struct {
	TenantID string `json:"tenant_id"`
	BackupID string `json:"backup_id"`

	S3Config struct {
		Provider   string `json:"provider"`
		Endpoint   string `json:"endpoint"`
		AccessKey  string `json:"access_key"`
		SecretKey  string `json:"secret_key"`
		BucketName string `json:"bucket_name"`
	} `json:"s3_config"`
}

func init() {
	RegisterWorker("delete_backup", BackupAPPDeleteCreater)
}

// BackupAPPDeleteCreater -
func BackupAPPDeleteCreater(in []byte, m *exectorManager) (TaskWorker, error) {
	var backupAPPDelete BackupAPPDelete
	if err := ffjson.Unmarshal(in, &backupAPPDelete); err != nil {
		return nil, err
	}
	return &backupAPPDelete, nil
}

//Run Run
func (b *BackupAPPDelete) Run(timeout time.Duration) error {
	logrus.Infof("backup id: %s; delete backup", b.BackupID)
	backup, err := db.GetManager().AppBackupDao().GetAppBackup(b.BackupID)
	if err != nil {
		return err
	}

	switch backup.BackupMode {
	case "full-online":
		if err := b.deleteFromS3(backup.SourceDir); err != nil {
			return fmt.Errorf("error delete file from s3: %v", err)
		}
	default:
		if err := b.deleteFromLocal(backup.SourceDir); err != nil {
			return fmt.Errorf("delete from local: %v", err)
		}
	}

	backup.Deleted = true
	if err := db.GetManager().AppBackupDao().UpdateModel(backup); err != nil {
		return fmt.Errorf("delete backup: %v", err)
	}

	return nil
}

//Stop stop
func (b *BackupAPPDelete) Stop() error {
	return nil
}

//Name return worker name
func (b *BackupAPPDelete) Name() string {
	return "backup_apps_delete"
}

//GetLogger GetLogger
func (b *BackupAPPDelete) GetLogger() event.Logger {
	return event.GetTestLogger()
}

//ErrorCallBack if run error will callback
func (b *BackupAPPDelete) ErrorCallBack(err error) {
}

func (b *BackupAPPDelete) deleteFromS3(sourceDir string) error {
	logrus.Infof("delete from s3: %s", sourceDir)
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
	logrus.Infof("object key: %s; delete backup", objectKey)
	if err := cloudoser.DeleteObject(objectKey); err != nil {
		return fmt.Errorf("object key: %s; error deleting file for object storage: %v", objectKey, err)
	}

	return nil
}

func (b *BackupAPPDelete) deleteFromLocal(sourceDir string) error {
	logrus.Infof("delete from local: %s", sourceDir)
	if err := os.RemoveAll(sourceDir); err != nil {
		return fmt.Errorf("remove '%s'", sourceDir)
	}

	return nil
}
