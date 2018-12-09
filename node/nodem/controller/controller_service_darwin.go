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

package controller

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/service"
)

//NewController At the stage you want to load the configurations of all rainbond components
func NewController(conf *option.Conf, manager *ManagerService) Controller {
	return &testController{}
}

type testController struct {
}

func (t *testController) InitStart(services []*service.Service) error {
	return nil
}
func (t *testController) StartService(name string) error {
	return nil
}
func (t *testController) StopService(name string) error {
	return nil
}
func (t *testController) StartList(list []*service.Service) error {
	return nil
}
func (t *testController) StopList(list []*service.Service) error {
	return nil
}
func (t *testController) RestartService(s *service.Service) error {
	return nil
}
func (t *testController) WriteConfig(s *service.Service) error {
	return nil
}
func (t *testController) RemoveConfig(name string) error {
	return nil
}
func (t *testController) EnableService(name string) error {
	return nil
}
func (t *testController) DisableService(name string) error {
	return nil
}
func (t *testController) CheckBeforeStart() bool {
	return false
}
