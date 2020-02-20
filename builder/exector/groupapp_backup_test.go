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

package exector

import (
	"fmt"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"strings"
	"testing"
)

func TestUploadPkg(t *testing.T) {
	b := &BackupAPPNew{
		SourceDir: "/tmp/groupbackup/0d65c6608729438aad0a94f6317c80d0_20191024180024.zip",
		Mode:      "full-online",
	}
	b.S3Config.Provider = "AliyunOSS"
	b.S3Config.Endpoint = "dummy"
	b.S3Config.AccessKey = "dummy"
	b.S3Config.SecretKey = "dummy"
	b.S3Config.BucketName = "dummy"

	if err := b.uploadPkg(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadPkg2(t *testing.T) {
	b := &BackupAPPRestore{}
	b.S3Config.Provider = "alioss"
	b.S3Config.Endpoint = "dummy"
	b.S3Config.AccessKey = "dummy"
	b.S3Config.SecretKey = "dummy"
	b.S3Config.BucketName = "hrhtest"

	cacheDir := fmt.Sprintf("/tmp/cache/tmp/%s/%s", "c6b05a2a6d664fda83dab8d3bcf1a941", util.NewUUID())
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		t.Errorf("create cache dir error %s", err.Error())
	}
	b.cacheDir = cacheDir

	sourceDir := "/tmp/groupbackup/c6b05a2a6d664fda83dab8d3bcf1a941_20191024185643.zip"
	if err := b.downloadFromS3(sourceDir); err != nil {
		t.Error(err)
	}
}

func TestBackupServiceVolume(t *testing.T) {
	volume := dbmodel.TenantServiceVolume{}
	sourceDir := ""
	serviceID := ""
	dstDir := fmt.Sprintf("%s/data_%s/%s.zip", sourceDir, serviceID, strings.Replace(volume.VolumeName, "/", "", -1))
	hostPath := volume.HostPath
	if hostPath != "" && !util.DirIsEmpty(hostPath) {
		if err := util.Zip(hostPath, dstDir); err != nil {
			t.Fatalf("backup service(%s) volume(%s) data error.%s", serviceID, volume.VolumeName, err.Error())
		}
	}
}
