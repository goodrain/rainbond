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

package region

// //DefineCloudAuth DefineCloudAuth
// func (t *tenant) DefineCloudAuth(gt *api_model.GetUserToken) DefineCloudAuthInterface {
// 	return &DefineCloudAuth{
// 		GT: gt,
// 	}
// }

// //DefineCloudAuth DefineCloudAuth
// type DefineCloudAuth struct {
// 	GT *api_model.GetUserToken
// }

// //DefineCloudAuthInterface DefineCloudAuthInterface
// type DefineCloudAuthInterface interface {
// 	GetToken() ([]byte, error)
// 	PostToken() ([]byte, error)
// 	PutToken() error
// }

// //GetToken GetToken
// func (d *DefineCloudAuth) GetToken() ([]byte, error) {
// 	resp, code, err := request(
// 		fmt.Sprintf("/cloud/auth/%s", d.GT.Body.EID),
// 		"GET",
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, util.CreateAPIHandleError(code, err)
// 	}
// 	if code > 400 {
// 		if code == 404 {
// 			return nil, util.CreateAPIHandleError(code, fmt.Errorf("eid %s is not exist", d.GT.Body.EID))
// 		}
// 		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get eid infos %s failed", d.GT.Body.EID))
// 	}
// 	//valJ, err := simplejson.NewJson(resp)
// 	return resp, nil
// }

// //PostToken PostToken
// func (d *DefineCloudAuth) PostToken() ([]byte, error) {
// 	data, err := ffjson.Marshal(d.GT.Body)
// 	if err != nil {
// 		return nil, err
// 	}
// 	resp, code, err := request(
// 		"/cloud/auth",
// 		"POST",
// 		data,
// 	)
// 	if err != nil {
// 		logrus.Errorf("create auth token error, %v", err)
// 		return nil, util.CreateAPIHandleError(code, err)
// 	}
// 	if code > 400 {
// 		logrus.Errorf("create auth token error")
// 		return nil, util.CreateAPIHandleError(code, fmt.Errorf("cretae auth token failed"))
// 	}
// 	return resp, nil
// }

// //PutToken PutToken
// func (d *DefineCloudAuth) PutToken() error {
// 	data, err := ffjson.Marshal(d.GT.Body)
// 	if err != nil {
// 		return err
// 	}
// 	_, code, err := request(
// 		fmt.Sprintf("/cloud/auth/%s", d.GT.Body.EID),
// 		"PUT",
// 		data,
// 	)
// 	if err != nil {
// 		logrus.Errorf("create auth token error, %v", err)
// 		return util.CreateAPIHandleError(code, err)
// 	}
// 	if code > 400 {
// 		logrus.Errorf("create auth token error")
// 		return util.CreateAPIHandleError(code, fmt.Errorf("cretae auth token failed"))
// 	}
// 	return nil
// }
