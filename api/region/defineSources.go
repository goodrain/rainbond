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

//DefineSources DefineSources
// func (t *tenant) DefineSources(ss *api_model.SourceSpec) DefineSourcesInterface {
// 	return &DefineSources{
// 		tenant: *tenant,
// 		Model: Body{
// 			SourceSpec: ss,
// 		},
// 	}
// }

// //DefineSources DefineSources
// type DefineSources struct {
// 	tenant
// 	Model Body
// }

// //Body Body
// type Body struct {
// 	SourceSpec *api_model.SourceSpec `json:"source_spec"`
// }

// //DefineSourcesInterface DefineSourcesInterface
// type DefineSourcesInterface interface {
// 	GetSource(sourceAlias string) ([]byte, error)
// 	PostSource(sourceAlias string) error
// 	PutSource(SourcesAlias string) error
// 	DeleteSource(sourceAlias string) error
// }

// //GetSource GetSource
// func (d *DefineSources) GetSource(sourceAlias string) ([]byte, error) {
// 	resp, status, err := request(
// 		fmt.Sprintf("/v2/tenants/%s/sources/%s/%s",
// 			d.tenant.tenantID, d.Model.SourceSpec.Alias, d.Model.SourceSpec.SourceBody.EnvName),
// 		"GET",
// 		nil,
// 	)
// 	if err != nil {
// 		logrus.Errorf("get define source %s error, %v", d.Model.SourceSpec.SourceBody.EnvName, err)
// 		return nil, err
// 	}
// 	if status > 400 {
// 		if status == 404 {
// 			return nil, fmt.Errorf("source %s is not exist", d.Model.SourceSpec.SourceBody.EnvName)
// 		}
// 		return nil, fmt.Errorf("get define source %s failed", d.Model.SourceSpec.SourceBody.EnvName)
// 	}
// 	//valJ, err := simplejson.NewJson(resp)
// 	return resp, nil
// }

// //PostSource PostSource
// func (d *DefineSources) PostSource(sourceAlias string) error {
// 	data, err := ffjson.Marshal(d.Model)
// 	if err != nil {
// 		return err
// 	}
// 	_, status, err := request(
// 		fmt.Sprintf("/v2/tenants/%s/sources/%s",
// 			d.tenant.tenantID, d.Model.SourceSpec.Alias),
// 		"POST",
// 		data,
// 	)
// 	if err != nil {
// 		logrus.Errorf("create define source error, %v", err)
// 		return err
// 	}
// 	if status > 400 {
// 		logrus.Errorf("create define source error")
// 		return fmt.Errorf("cretae define source failed")
// 	}
// 	return nil
// }

// //PutSource PutSource
// func (d *DefineSources) PutSource(sourceAlias string) error {
// 	data, err := ffjson.Marshal(d.Model)
// 	if err != nil {
// 		return err
// 	}
// 	_, status, err := request(
// 		fmt.Sprintf("/v2/tenants/%s/sources/%s/%s",
// 			d.tenant.tenantID, d.Model.SourceSpec.Alias, d.Model.SourceSpec.SourceBody.EnvName),
// 		"PUT",
// 		data,
// 	)
// 	if err != nil {
// 		logrus.Errorf("update define source error, %v", err)
// 		return err
// 	}
// 	if status > 400 {
// 		logrus.Errorf("update define source error")
// 		return fmt.Errorf("update define source failed")
// 	}
// 	return nil
// }

// //DeleteSource DeleteSource
// func (d *DefineSources) DeleteSource(sourceAlias string) error {
// 	_, status, err := request(
// 		fmt.Sprintf("/v2/tenants/%s/sources/%s/%s",
// 			d.tenant.tenantID, d.Model.SourceSpec.Alias, d.Model.SourceSpec.SourceBody.EnvName),
// 		"DELETE",
// 		nil,
// 	)
// 	if err != nil {
// 		logrus.Errorf("delete define source error, %v", err)
// 		return err
// 	}
// 	if status > 400 {
// 		logrus.Errorf("delete define source %s error", d.Model.SourceSpec.SourceBody.EnvName)
// 	}
// 	return nil
// }
