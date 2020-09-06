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

package etcdlock

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestMasterLock(t *testing.T) {
	master1, err := CreateMasterLock(nil, "/rainbond/appruntimesyncmaster", "127.0.0.1:1", 10)
	if err != nil {
		t.Fatal(err)
	}
	master1.Start()
	defer master1.Stop()
	master2, err := CreateMasterLock(nil, "/rainbond/appruntimesyncmaster", "127.0.0.1:2", 10)
	if err != nil {
		t.Fatal(err)
	}
	master2.Start()
	defer master2.Stop()
	logrus.Info("start receive event")
	for {
		select {
		case event := <-master1.EventsChan():
			logrus.Info("master1", event, time.Now())
			//master1.Stop()
			if event.Type == MasterDeleted {
				logrus.Info("delete")
				return
			}
		case event := <-master2.EventsChan():
			logrus.Info("master2", event, time.Now())
			//master2.Stop()
			if event.Type == MasterDeleted {
				logrus.Info("delete")
				return
			}
		}
	}
}
