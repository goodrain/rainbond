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

package daemon

import (
	"github.com/goodrain/rainbond/api/util"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
)

//StartRegionAPI 启动
func StartRegionAPI(ch chan os.Signal) {
	logrus.Info("old region api begin start..")
	arg := []string{"region_api.wsgi", "-b=127.0.0.1:8887", "--max-requests=5000", "--reload", "--debug", "--workers=4", "--log-file", "-", "--access-logfile", "-", "--error-logfile", "-"}
	cmd := exec.Command("gunicorn", arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		if err := cmd.Start(); err != nil {
			logrus.Error("start region old api error.", err.Error())
		}
		tick := time.NewTicker(time.Second * 5)
		select {
		case si := <-ch:
			cmd.Process.Signal(si)
			return
		case <-tick.C:
			monitor()
		}
	}()
	return
}
func monitor() {
	response, err := http.Get("http://127.0.0.1:8887/monitor")
	if err != nil {
		logrus.Error("monitor region old api error.", err.Error())
		return
	}
	defer util.CloseResponse(response)
	if response != nil && response.StatusCode/100 > 2 {
		logrus.Errorf("monitor region old api error. response code is %s", response.Status)
	}

}
