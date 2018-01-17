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

package sources

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/pkg/event"
)

//ImagePull 拉取镜像
//timeout 分钟为单位
func ImagePull(dockerCli *client.Client, image string, opts types.ImagePullOptions, logger event.Logger, timeout int) (*types.ImageInspect, error) {
	if logger != nil {
		//进度信息
		logger.Info(fmt.Sprintf("开始获取镜像：%s", image), map[string]string{"step": "pullimage"})
	}
	_, err := reference.ParseAnyReference(image)
	if err != nil {
		return nil, err
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	readcloser, err := dockerCli.ImagePull(ctx, image, opts)
	if err != nil {
		if strings.HasSuffix(err.Error(), "does not exist or no pull access") {
			return nil, fmt.Errorf("Image(%s) does not exist or no pull access", image)
		}
		return nil, err
	}
	defer readcloser.Close()
	r := bufio.NewReader(readcloser)
	for {
		if line, _, err := r.ReadLine(); err == nil {
			if logger != nil {
				//进度信息
				logger.Debug(string(line), map[string]string{"step": "progress"})
			}
			fmt.Println(string(line))
		} else {
			break
		}
	}
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

//ImagePush 推送镜像
//timeout 分钟为单位
func ImagePush(dockerCli *client.Client, image string, opts types.ImagePushOptions, logger event.Logger, timeout int) error {
	return nil
}
