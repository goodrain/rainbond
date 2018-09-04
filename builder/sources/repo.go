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

package sources

import (
	"fmt"
	"path"
	"strings"

	"github.com/goodrain/rainbond/util"

	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

//RepostoryBuildInfo 源码编译信息
type RepostoryBuildInfo struct {
	RepostoryURL     string
	RepostoryURLType string
	BuildBranch      string
	BuildPath        string
	CodeHome         string
	ep               *transport.Endpoint
}

//GetCodeHome 获取代码目录
func (r *RepostoryBuildInfo) GetCodeHome() string {
	if r.RepostoryURLType == "svn" {
		if ok, _ := util.FileExists(path.Join(r.CodeHome, "trunk")); ok && r.BuildBranch == "trunk" {
			return path.Join(r.CodeHome, "trunk")
		}
		if r.BuildBranch != "" && r.BuildBranch != "trunk" {
			if strings.HasPrefix(r.BuildBranch, "tag:") {
				codepath := path.Join(r.CodeHome, "tags", r.BuildBranch[4:])
				if ok, _ := util.FileExists(codepath); ok {
					return codepath
				}
				codepath = path.Join(r.CodeHome, "Tags", r.BuildBranch[4:])
				if ok, _ := util.FileExists(codepath); ok {
					return codepath
				}
			}
			codepath := path.Join(r.CodeHome, "branches", r.BuildBranch)
			if ok, _ := util.FileExists(codepath); ok {
				return codepath
			}
			codepath = path.Join(r.CodeHome, "Branches", r.BuildBranch)
			if ok, _ := util.FileExists(codepath); ok {
				return codepath
			}
		}
	}
	return r.CodeHome
}

//GetCodeBuildAbsPath 获取代码编译绝对目录
func (r *RepostoryBuildInfo) GetCodeBuildAbsPath() string {
	return path.Join(r.GetCodeHome(), r.BuildPath)
}

//GetCodeBuildPath 获取代码编译相对目录
func (r *RepostoryBuildInfo) GetCodeBuildPath() string {
	return r.BuildPath
}

//GetProtocol 获取协议
func (r *RepostoryBuildInfo) GetProtocol() string {
	if r.ep != nil {
		if r.ep.Protocol == "" {
			return "ssh"
		}
		return r.ep.Protocol
	}
	return ""
}

//CreateRepostoryBuildInfo 创建源码编译信息
//repoType git or svn
func CreateRepostoryBuildInfo(repoURL, repoType, branch, tenantID string, ServiceID string) (*RepostoryBuildInfo, error) {
	// repoURL= github.com/goodrain/xxx.git?dir=home
	ep, err := transport.NewEndpoint(repoURL)
	if err != nil {
		return nil, err
	}
	rbi := &RepostoryBuildInfo{
		ep:               ep,
		RepostoryURL:     repoURL,
		RepostoryURLType: repoType,
		BuildBranch:      branch,
	}
	index := strings.Index(repoURL, "?dir=")
	if index > -1 && len(repoURL) > index+5 {
		fmt.Println(repoURL[index+5:], repoURL[:index])
		rbi.BuildPath = repoURL[index+5:]
		rbi.CodeHome = GetCodeSourceDir(repoURL[:index], branch, tenantID, ServiceID)
		rbi.RepostoryURL = repoURL[:index]
	}
	rbi.CodeHome = GetCodeSourceDir(repoURL, branch, tenantID, ServiceID)
	return rbi, nil
}
