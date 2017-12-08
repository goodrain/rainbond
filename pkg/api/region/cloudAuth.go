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

package region

import (
	"fmt"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
)

//DefineCloudAuth DefineCloudAuth
func (t *Tenant) DefineCloudAuth(gt *api_model.GetUserToken) DefineCloudAuthInterface {
	return &DefineCloudAuth{
		GT: gt,
	}
}

//DefineCloudAuth DefineCloudAuth
type DefineCloudAuth struct {
	GT *api_model.GetUserToken
}

//DefineCloudAuthInterface DefineCloudAuthInterface
type DefineCloudAuthInterface interface {
	GetToken() ([]byte, error)
	PostToken() error
	PutToken() error
}

//GetToken GetToken
func (d *DefineCloudAuth) GetToken() ([]byte, error) {
	resp, status, err := DoRequest(
		fmt.Sprintf("/cloud/auth/%s", d.GT.Body.EID),
		"GET",
		nil,
	)
	if err != nil {
		logrus.Errorf("get cloud auth %s error, %v", d.GT.Body.EID, err)
		return nil, err
	}
	if status > 400 {
		if status == 404 {
			return nil, fmt.Errorf("eid %s is not exist", d.GT.Body.EID)
		}
		return nil, fmt.Errorf("get eid infos %s failed", d.GT.Body.EID)
	}
	//valJ, err := simplejson.NewJson(resp)
	return resp, nil
}

//PostToken PostToken
func (d *DefineCloudAuth) PostToken() error {
	data, err := ffjson.Marshal(d.GT.Body)
	if err != nil {
		return err
	}
	_, status, err := DoRequest(
		"/cloud/auth",
		"POST",
		data,
	)
	if err != nil {
		logrus.Errorf("create auth token error, %v", err)
		return err
	}
	if status > 400 {
		logrus.Errorf("create auth token error")
		return fmt.Errorf("cretae auth token failed")
	}
	return nil
}

//PutToken PutToken
func (d *DefineCloudAuth) PutToken() error {
	data, err := ffjson.Marshal(d.GT.Body)
	if err != nil {
		return err
	}
	_, status, err := DoRequest(
		fmt.Sprintf("/cloud/auth/%s", d.GT.Body.EID),
		"PUT",
		data,
	)
	if err != nil {
		logrus.Errorf("update token ttl error, %v", err)
		return err
	}
	if status > 400 {
		logrus.Errorf("update token ttl error")
		return fmt.Errorf("update token ttl failed")
	}
	return nil
}
