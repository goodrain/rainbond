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

package sources

import (
	"testing"

	"github.com/goodrain/rainbond/pkg/event"
)

func TestFTPUp(t *testing.T) {
	logger := event.GetManager().GetLogger("system")
	ftp := NewFTPConnManager(logger, "goodrain-admin", "goodrain123465", "139.196.88.57", 10021)
	defer ftp.FTP.Close()
	upfile := "/Users/pujielan/Downloads/http.conf"
	if err := ftp.FTPLogin(logger); err != nil {
		t.Fatal(err)	
	}
	path := "app-publish/application/"
	curPath, err := ftp.FTPCWD(logger, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ftp.FTPUpload(logger, curPath, upfile); err != nil {
		t.Fatal(err)
	}
	//t.Logf("%+v", __)
}
