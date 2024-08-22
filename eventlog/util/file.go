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
// 文件: file.go
// 说明: 该文件实现了文件管理功能的核心组件。文件中定义了用于处理文件操作的相关方法，
// 以支持平台内的文件读写和管理需求。通过这些方法，Rainbond 平台能够高效地管理文件资源，
// 提供可靠的文件存储和访问服务。

package util

import "os"

// AppendToFile 文件名字(带全路径)
// content: 写入的内容
func AppendToFile(fileName string, content string) error {
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	defer f.Close()
	return err
}
