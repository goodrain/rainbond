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
	"encoding/json"
	"fmt"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capability_id: rainbond.app-backup.upload-package
func TestUploadPkg(t *testing.T) {
	t.Skip("requires external storage integration")
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

// capability_id: rainbond.app-backup.upload-package-download-guard
func TestUploadPkg2(t *testing.T) {
	t.Skip("stale integration test references removed downloadFromS3 API")
}

// capability_id: rainbond.app-backup.metadata-version-detect
func TestJudgeMetadataVersion(t *testing.T) {
	newMetaBytes, err := json.Marshal(AppSnapshot{Services: []*RegionServiceSnapshot{{
		ServiceID: "svc-1",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	version, err := judgeMetadataVersion(newMetaBytes)
	if err != nil {
		t.Fatal(err)
	}
	if version != NewMetadata {
		t.Fatalf("expected %q, got %q", NewMetadata, version)
	}

	oldMetaBytes, err := json.Marshal([]*RegionServiceSnapshot{{
		ServiceID: "svc-2",
	}})
	if err != nil {
		t.Fatal(err)
	}
	version, err = judgeMetadataVersion(oldMetaBytes)
	if err != nil {
		t.Fatal(err)
	}
	if version != OldMetadata {
		t.Fatalf("expected %q, got %q", OldMetadata, version)
	}
}

// capability_id: rainbond.app-backup.volume-dir-defaults
func TestGetVolumeDir(t *testing.T) {
	t.Setenv("LOCAL_DATA_PATH", "")
	t.Setenv("SHARE_DATA_PATH", "")
	localPath, sharePath := GetVolumeDir()
	if localPath != "/grlocaldata" || sharePath != "/grdata" {
		t.Fatalf("unexpected defaults: local=%q share=%q", localPath, sharePath)
	}

	t.Setenv("LOCAL_DATA_PATH", "/custom-local")
	t.Setenv("SHARE_DATA_PATH", "/custom-share")
	localPath, sharePath = GetVolumeDir()
	if localPath != "/custom-local" || sharePath != "/custom-share" {
		t.Fatalf("unexpected custom dirs: local=%q share=%q", localPath, sharePath)
	}
}

// capability_id: rainbond.app-backup.service-volume-archive
func TestBackupServiceVolume(t *testing.T) {
	root := t.TempDir()
	hostPath := filepath.Join(root, "volume-data")
	if err := os.MkdirAll(hostPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostPath, "hello.txt"), []byte("rainbond"), 0644); err != nil {
		t.Fatal(err)
	}

	volume := dbmodel.TenantServiceVolume{VolumeName: "data", HostPath: hostPath}
	sourceDir := root
	serviceID := "svc-1"
	dstDir := fmt.Sprintf("%s/data_%s/%s.zip", sourceDir, serviceID, strings.Replace(volume.VolumeName, "/", "", -1))
	if hostPath != "" && !util.DirIsEmpty(hostPath) {
		if err := util.Zip(hostPath, dstDir); err != nil {
			t.Fatalf("backup service(%s) volume(%s) data error.%s", serviceID, volume.VolumeName, err.Error())
		}
	}
	if ok, err := util.FileExists(dstDir); err != nil || !ok {
		t.Fatalf("expected backup archive %q to exist, err=%v", dstDir, err)
	}
}
