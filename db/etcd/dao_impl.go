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

package etcd

import (
	"github.com/goodrain/rainbond/db/dao"
)

// TenantDao  tenantDao
func (m *Manager) TenantDao() dao.TenantDao {
	return nil
}

// TenantServiceDao TenantServiceDao
func (m *Manager) TenantServiceDao() dao.TenantServiceDao {
	return nil
}

// TenantServicesPortDao TenantServicesPortDao
func (m *Manager) TenantServicesPortDao() dao.TenantServicesPortDao {
	return nil
}

// TenantServiceRelationDao TenantServiceRelationDao
func (m *Manager) TenantServiceRelationDao() dao.TenantServiceRelationDao {
	return nil
}

// TenantServiceEnvVarDao TenantServiceEnvVarDao
func (m *Manager) TenantServiceEnvVarDao() dao.TenantServiceEnvVarDao {
	return nil
}

// TenantServiceMountRelationDao TenantServiceMountRelationDao
func (m *Manager) TenantServiceMountRelationDao() dao.TenantServiceMountRelationDao {
	return nil
}

// TenantServiceVolumeDao TenantServiceVolumeDao
func (m *Manager) TenantServiceVolumeDao() dao.TenantServiceVolumeDao {
	return nil
}

// func (m *Manager) K8sServiceDao() dao.K8sServiceDao {
// 	return nil
// }
// func (m *Manager) K8sDeployReplicationDao() dao.K8sDeployReplicationDao {
// 	return nil
// }
// func (m *Manager) K8sPodDao() dao.K8sPodDao {
// 	return nil
// }

// ServiceProbeDao ServiceProbeDao
func (m *Manager) ServiceProbeDao() dao.ServiceProbeDao {
	return nil
}
