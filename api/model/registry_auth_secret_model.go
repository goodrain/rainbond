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

package model

//AddOrUpdateRegistryAuthSecretStruct is used to add or update registry auth secret
type AddOrUpdateRegistryAuthSecretStruct struct {
	TenantID string `json:"tenant_id" validate:"tenant_id|required"`
	SecretID string `json:"secret_id" validate:"secret_id|required"`
	Domain   string `json:"domain" validate:"domain|required"`
	Username string `json:"username" validate:"username|required"`
	Password string `json:"password" validate:"password|required"`
}

//DeleteRegistryAuthSecretStruct is used to delete registry auth secret
type DeleteRegistryAuthSecretStruct struct {
	TenantID string `json:"tenant_id" validate:"tenant_id|required"`
	SecretID string `json:"secret_id" validate:"secret_id|required"`
}
