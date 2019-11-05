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
