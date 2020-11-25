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

package openresty

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Check returns if the nginx healthz endpoint is returning ok (status code 200)
func (o *OrService) Check() error {
	url := fmt.Sprintf("http://127.0.0.1:%v/%v", o.ocfg.ListenPorts.Status, o.ocfg.HealthPath)
	timeout := o.ocfg.HealthCheckTimeout
	statusCode, err := simpleGet(url, timeout)
	if err != nil {
		logrus.Errorf("error checking %s healthz: %v", url, err)
		return err
	}
	if statusCode != 200 {
		return fmt.Errorf("ingress controller is not healthy")
	}

	return nil
}

func simpleGet(url string, timeout time.Duration) (int, error) {
	client := &http.Client{
		Timeout:   timeout * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return -1, err
	}

	res, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()

	return res.StatusCode, nil
}
