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

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
)

var (
	extendTag = "@extend:"
	pwdTag    = "@pwd@"
	rootTag   = "@root@"
	root      = ""
)

// 设置扩展标识，如果不设置，默认为 '@extend:'
func SetExtendTag(tag string) {
	extendTag = tag
}

func SetRoot(r string) {
	root = r
}

// 设置当前路径标识，如果不设置，默认为 '@pwd@'
// @pwd@ 会被替换成当前文件的路径，
// 至于是绝对路径还是相对路径，取决于读取文件时，传入的是绝对路径还是相对路径
func SetPathTag(tag string) {
	pwdTag = tag
}

//加载json（可配置扩展字段）配置文件
func LoadExtendConf(filePath string, v interface{}) error {
	data, err := extendFile(filePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, v)
	return err
}

func extendFile(filePath string) (data []byte, err error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return
	}
	if fi.IsDir() {
		err = fmt.Errorf(filePath + " is not a file.")
		return
	}

	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}

	if len(root) != 0 {
		b = bytes.Replace(b, []byte(rootTag), []byte(root), -1)
	}

	dir := filepath.Dir(filePath)
	return extendFileContent(dir, bytes.Replace(b, []byte(pwdTag), []byte(dir), -1))
}

func extendFileContent(dir string, content []byte) (data []byte, err error) {
	//检查是不是规范的json
	test := new(interface{})
	err = json.Unmarshal(content, &test)
	if err != nil {
		return
	}

	// 替换子json文件
	reg := regexp.MustCompile(`"` + extendTag + `.*?"`)
	data = reg.ReplaceAllFunc(content, func(match []byte) []byte {
		match = match[len(extendTag)+1 : len(match)-1]
		sb, e := extendFile(filepath.Join(dir, string(match)))
		if e != nil {
			err = fmt.Errorf("替换json配置[%s]失败：%s", match, e.Error())
		}
		return sb
	})
	return
}
