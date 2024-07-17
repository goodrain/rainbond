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

package service

import (
	"archive/tar"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/node/core/store"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// AppService app service
type AppService struct {
	Prefix    string
	c         *option.Conf
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// CreateAppService create
func CreateAppService(c *option.Conf, clientset *kubernetes.Clientset, config *rest.Config) *AppService {
	return &AppService{
		c:         c,
		Prefix:    "/traefik",
		clientset: clientset,
		config:    config,
	}
}

func (a *AppService) AppFileUpload(containerName, podName, srcPath, destPath, namespace string) error {
	reader, writer := io.Pipe()
	if destPath != "/" && strings.HasSuffix(string(destPath[len(destPath)-1]), "/") {
		destPath = destPath[:len(destPath)-1]
	}
	destPath = destPath + "/" + path.Base(srcPath)
	go func() {
		defer writer.Close()
		cmdutil.CheckErr(cpMakeTar(srcPath, destPath, writer))
	}()
	var cmdArr []string
	cmdArr = []string{"tar", "-xmf", "-"}
	destDir := path.Dir(destPath)
	if len(destDir) > 0 {
		cmdArr = append(cmdArr, "-C", destDir)
	}
	req := a.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(a.config, "POST", req.URL())
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  reader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
}

func (a *AppService) AppFileDownload(containerName, podName, filePath, namespace string) error {
	reader, outStream := io.Pipe()
	req := a.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"tar", "cf", "-", filePath},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(a.config, "POST", req.URL())
	if err != nil {
		return err
	}
	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
		cmdutil.CheckErr(err)
	}()
	prefix := getPrefix(filePath)
	prefix = path.Clean(prefix)
	destPath := path.Join("./", path.Base(prefix))
	err = unTarAll(reader, destPath, prefix)
	if err != nil {
		return err
	}
	return nil
}

func cpMakeTar(srcPath string, destPath string, out io.Writer) error {
	tw := tar.NewWriter(out)

	defer func() {
		if err := tw.Close(); err != nil {
			logrus.Errorf("Error closing tar writer: %v\n", err)
		}
	}()

	// 获取源路径的信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get info for %s: %v", srcPath, err)
	}

	basePath := srcPath
	if srcInfo.IsDir() {
		basePath = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对于源路径的相对路径
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// 创建 tar 归档文件的文件头信息
		hdr, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return fmt.Errorf("failed to create header for %s: %v", path, err)
		}

		// 写入文件头信息到 tar 归档
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write header for %s: %v", path, err)
		}

		if info.Mode().IsRegular() {
			// 如果是普通文件，则将文件内容写入到 tar 归档
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %v", path, err)
			}
			defer file.Close()

			_, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("failed to write file %s to tar: %v", path, err)
			}
		}

		return nil
	})
}

func unTarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPrefix(file string) string {
	return strings.TrimLeft(file, "/")
}

// FindAppEndpoints 获取app endpoint
func (a *AppService) FindAppEndpoints(appName string) []*config.Endpoint {
	var ends = make(map[string]*config.Endpoint)
	res, err := store.DefalutClient.Get(fmt.Sprintf("%s/backends/%s/servers", a.Prefix, appName), clientv3.WithPrefix())
	if err != nil {
		logrus.Errorf("list all servers of %s error.%s", appName, err.Error())
		return nil
	}
	if res.Count == 0 {
		return nil
	}
	for _, kv := range res.Kvs {
		if strings.HasSuffix(string(kv.Key), "/url") { //获取服务地址
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverURL := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					en.URL = serverURL
				} else {
					ends[serverName] = &config.Endpoint{Name: serverName, URL: serverURL}
				}
			}
		}
		if strings.HasSuffix(string(kv.Key), "/weight") { //获取服务权重
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverWeight := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					var err error
					en.Weight, err = strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
				} else {
					weight, err := strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
					ends[serverName] = &config.Endpoint{Name: serverName, Weight: weight}
				}
			}
		}
	}
	result := []*config.Endpoint{}
	for _, v := range ends {
		result = append(result, v)
	}
	return result
}
