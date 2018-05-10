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

package utils

import "errors"

var (
	ErrNotFound        = errors.New("Record not found.")
	ErrValueMayChanged = errors.New("The value has been changed by others on this time.")

	ErrEmptyJobName        = errors.New("Name of job is empty.")
	ErrEmptyJobCommand     = errors.New("Command of job is empty.")
	ErrIllegalJobId        = errors.New("Invalid id that includes illegal characters such as '/'.")
	ErrIllegalJobGroupName = errors.New("Invalid job group name that includes illegal characters such as '/'.")

	ErrEmptyNodeGroupName = errors.New("Name of node group is empty.")
	ErrIllegalNodeGroupId = errors.New("Invalid node group id that includes illegal characters such as '/'.")

	ErrSecurityInvalidCmd  = errors.New("Security error: the suffix of script file is not on the whitelist.")
	ErrSecurityInvalidUser = errors.New("Security error: the user is not on the whitelist.")
	ErrNilRule             = errors.New("invalid job rule, empty timer.")
)
