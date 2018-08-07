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
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/docker/distribution/reference"
	"golang.org/x/net/context"
	//"github.com/docker/docker/api/types"
	"github.com/docker/engine-api/types"
	//"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder/model"
	"github.com/goodrain/rainbond/event"
)

//ImagePull 拉取镜像
//timeout 分钟为单位
func ImagePull(dockerCli *client.Client, image string, username, password string, logger event.Logger, timeout int) (*types.ImageInspect, error) {
	if logger != nil {
		//进度信息
		logger.Info(fmt.Sprintf("开始获取镜像：%s", image), map[string]string{"step": "pullimage"})
	}
	var pullipo types.ImagePullOptions
	if username != "" && password != "" {
		auth, err := EncodeAuthToBase64(types.AuthConfig{Username: username, Password: password})
		if err != nil {
			logrus.Errorf("make auth base63 push image error: %s", err.Error())
			logger.Error(fmt.Sprintf("生成获取镜像的Token失败"), map[string]string{"step": "builder-exector", "status": "failure"})
			return nil, err
		}
		pullipo = types.ImagePullOptions{
			RegistryAuth: auth,
		}
	} else {
		pullipo = types.ImagePullOptions{}
	}
	rf, err := reference.ParseAnyReference(image)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return nil, err
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	//TODO: 使用1.12版本api的bug “repository name must be canonical”，使用rf.String()完整的镜像地址
	readcloser, err := dockerCli.ImagePull(ctx, rf.String(), pullipo)
	if err != nil {
		logrus.Debugf("image name: %s readcloser error: %v", image, err.Error())
		if strings.HasSuffix(err.Error(), "does not exist or no pull access") {
			if logger != nil {
				logger.Error(fmt.Sprintf("镜像：%s不存在或无权获取", image), map[string]string{"step": "pullimage"})
			}
			return nil, fmt.Errorf("Image(%s) does not exist or no pull access", image)
		}
		return nil, err
	}
	defer readcloser.Close()
	dec := json.NewDecoder(readcloser)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var jm JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if jm.Error != nil {
			return nil, jm.Error
		}
		if logger != nil {
			logger.Debug(jm.JSONString(), map[string]string{"step": "progress"})
		} else {
			fmt.Println(jm.JSONString())
		}
	}
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image, false)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

//ImageTag 修改镜像tag
func ImageTag(dockerCli *client.Client, source, target string, logger event.Logger, timeout int) error {
	if logger != nil {
		//进度信息
		logger.Info(fmt.Sprintf("开始修改镜像tag：%s -> %s", source, target), map[string]string{"step": "changetag"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	err := dockerCli.ImageTag(ctx, source, target)
	if err != nil {
		logrus.Debugf("image tag err: %s", err.Error())
		return err
	}
	logger.Info("镜像tag修改完成", map[string]string{"step": "changetag"})
	return nil
}

//ImageNameHandle 解析imagename
func ImageNameHandle(imageName string) *model.ImageName {
	var i model.ImageName
	if strings.Contains(imageName, "/") {
		mm := strings.Split(imageName, "/")
		i.Host = mm[0]
		names := strings.Join(mm[1:], "/")
		if strings.Contains(names, ":") {
			nn := strings.Split(names, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = names
			i.Tag = "latest"
		}
	} else {
		if strings.Contains(imageName, ":") {
			nn := strings.Split(imageName, ":")
			i.Name = nn[0]
			i.Tag = nn[1]
		} else {
			i.Name = imageName
			i.Tag = "latest"
		}
	}
	return &i
}

//ImagePush 推送镜像
//timeout 分钟为单位
func ImagePush(dockerCli *client.Client, image, user, pass string, logger event.Logger, timeout int) error {
	if logger != nil {
		//进度信息
		logger.Info(fmt.Sprintf("开始推送镜像：%s", image), map[string]string{"step": "pushimage"})
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	if user == "" {
		user = os.Getenv("LOCAL_HUB_USER")
	}
	if pass == "" {
		pass = os.Getenv("LOCAL_HUB_PASS")
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	var opts types.ImagePushOptions
	pushauth, err := EncodeAuthToBase64(types.AuthConfig{
		Username:      user,
		Password:      pass,
		ServerAddress: reference.Domain(ref),
	})
	if err != nil {
		logrus.Errorf("make auth base63 push image error: %s", err.Error())
		if logger != nil {
			logger.Error(fmt.Sprintf("生成获取镜像的Token失败"), map[string]string{"step": "builder-exector", "status": "failure"})
		}
		return err
	}
	opts.RegistryAuth = pushauth
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	readcloser, err := dockerCli.ImagePush(ctx, image, opts)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			if logger != nil {
				logger.Error(fmt.Sprintf("镜像：%s不存在，不能推送", image), map[string]string{"step": "pushimage"})
			}
			return fmt.Errorf("Image(%s) does not exist", image)
		}
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		dec := json.NewDecoder(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
			logger.Debug(jm.JSONString(), map[string]string{"step": "progress"})
		}

	}
	return nil
}

//TrustedImagePush push image to trusted registry
func TrustedImagePush(dockerCli *client.Client, image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return ImagePush(dockerCli, image, user, pass, logger, timeout)
}

//CheckTrustedRepositories check Repositories is exist ,if not create it.
func CheckTrustedRepositories(image, user, pass string) error {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	var server string
	if reference.IsNameOnly(ref) {
		server = "docker.io"
	} else {
		server = reference.Domain(ref)
	}
	cli, err := createTrustedRegistryClient(server, user, pass)
	if err != nil {
		return err
	}
	var namespace, repName string
	infos := strings.Split(reference.TrimNamed(ref).String(), "/")
	if len(infos) == 3 && infos[0] == server {
		namespace = infos[1]
		repName = infos[2]
	}
	if len(infos) == 2 {
		namespace = infos[0]
		repName = infos[1]
	}
	_, err = cli.GetRepository(namespace, repName)
	if err != nil {
		if err.Error() == "resource does not exist" {
			rep := Repostory{
				Name:             repName,
				ShortDescription: fmt.Sprintf("push image for %s", image),
				LongDescription:  fmt.Sprintf("push image for %s", image),
				Visibility:       "private",
			}
			err := cli.CreateRepository(namespace, &rep)
			if err != nil {
				return fmt.Errorf("create repostory error,%s", err.Error())
			}
			return nil
		}
		return fmt.Errorf("get repostory error,%s", err.Error())
	}
	return err
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

//ImageBuild ImageBuild
func ImageBuild(dockerCli *client.Client, contextDir string, options types.ImageBuildOptions, logger event.Logger, timeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: []string{""},
		IncludeFiles:    []string{"."},
	})
	if err != nil {
		return err
	}
	rc, err := dockerCli.ImageBuild(ctx, buildCtx, options)
	if err != nil {
		return err
	}
	if rc.Body != nil {
		defer rc.Body.Close()
		dec := json.NewDecoder(rc.Body)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
			logger.Info(jm.JSONString(), map[string]string{"step": "build-progress"})
		}
	}
	return nil
}

//ImageInspectWithRaw get image inspect
func ImageInspectWithRaw(dockerCli *client.Client, image string) (*types.ImageInspect, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ins, _, err := dockerCli.ImageInspectWithRaw(ctx, image, false)
	if err != nil {
		return nil, err
	}
	return &ins, nil
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageSave(dockerCli *client.Client, image, destination string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc, err := dockerCli.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func ImageLoad(dockerCli *client.Client, tarFile string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = dockerCli.ImageLoad(ctx, reader, false)
	if err != nil {
		return err
	}

	return nil
}

//ImageImport save image to tar file
// source source file name eg. /tmp/xxx.tar
func ImageImport(dockerCli *client.Client, image, source string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	isource := types.ImageImportSource{
		Source:     file,
		SourceName: "-",
	}

	options := types.ImageImportOptions{}

	readcloser, err := dockerCli.ImageImport(ctx, isource, image, options)
	if err != nil {
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		r := bufio.NewReader(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if line, _, err := r.ReadLine(); err == nil {
				if logger != nil {
					logger.Debug(string(line), map[string]string{"step": "progress"})
				} else {
					fmt.Println(string(line))
				}
			} else {
				if err.Error() == "EOF" {
					return nil
				}
				return err
			}
		}
	}
	return nil
}

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	// We use sequential file access here to avoid depleting the standby list
	// on Windows. On Linux, this is a call directly to ioutil.TempFile
	tmpFile, err := os.OpenFile(path.Join(filepath.Dir(outfile), ".docker_temp_"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	_, err = io.Copy(tmpFile, r)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

//ImageRemove remove image
func ImageRemove(dockerCli *client.Client, image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := dockerCli.ImageRemove(ctx, image, types.ImageRemoveOptions{})
	return err
}
