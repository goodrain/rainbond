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

/*
本文件定义了与应用服务转换相关的错误处理功能。

主要内容：
1. `ErrServiceNotFound` 错误：定义了一个表示服务未找到的错误。该错误在尝试查找某个服务但未能找到时使用。

文件说明：
- 该文件提供了一个错误类型 `ErrServiceNotFound`，用于标识在服务管理过程中发生的“服务未找到”情况。
- 该错误通常用于服务相关的操作中，例如查找服务时未能找到指定服务的场景。

版权信息：
- 本程序为 Rainbond 应用管理平台，版权所有 (C) 2014-2017 Goodrain Co., Ltd.
- 本程序是自由软件：您可以按照 GNU 通用公共许可证的条款重新分发和/或修改该程序，许可证版本为 3，或（根据您的选择）任何更高版本。
- 如果需要非 GPL 许可使用 Rainbond，必须首先获得 Goodrain Co., Ltd. 授权的商业许可证。
- 本程序以希望它对您有用的方式发布，但不提供任何担保，包括但不限于适销性或特定用途适用性的默示担保。
- 您应该已经收到 GNU 通用公共许可证的副本。如果没有，请访问 <http://www.gnu.org/licenses/>。
*/

package conversion

import "errors"

// ErrServiceNotFound error not found
var ErrServiceNotFound = errors.New("service not found")
