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

package sources

import (
	"testing"

	"github.com/goodrain/rainbond/event"
)

func TestPushFile(t *testing.T) {
	sftpClient, err := NewSFTPClient("admin", "9bc067dc", "47.92.168.60", "20012")
	if err != nil {
		t.Fatal(err)
	}
	if err := sftpClient.PushFile("/tmp/src.tgz", "/upload/team/servicekey/goodrain.tgz", event.GetTestLogger()); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadFile(t *testing.T) {
	sftpClient, err := NewSFTPClient("foo", "pass", "22.gr6ac909.0mi9zp2q.lfsdo.goodrain.org", "20004")
	if err != nil {
		t.Fatal(err)
	}
	if err := sftpClient.DownloadFile("/upload/team/servicekey/goodrain.tgz", "./team/servicekey/goodrain.tgz", event.GetTestLogger()); err != nil {
		t.Fatal(err)
	}
}
