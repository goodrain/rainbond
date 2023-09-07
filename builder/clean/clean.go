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

package clean

import (
	"bytes"
	"context"
	"errors"
	"github.com/containerd/containerd/errdefs"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"strings"
	"time"
)

// Manager CleanManager
type Manager struct {
	imageClient   sources.ImageClient
	ctx           context.Context
	cancel        context.CancelFunc
	config        *rest.Config
	keepCount     uint
	clientset     *kubernetes.Clientset
	cleanInterval int
}

// CreateCleanManager create clean manager
func CreateCleanManager(imageClient sources.ImageClient, config *rest.Config, clientset *kubernetes.Clientset, keepCount uint, cleanInterval int) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Manager{
		imageClient:   imageClient,
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
		keepCount:     keepCount,
		clientset:     clientset,
		cleanInterval: cleanInterval,
	}
	return c, nil
}

// Start start clean
func (t *Manager) Start(errchan chan error) error {
	logrus.Info("CleanManager is starting.")
	duration := time.Duration(t.cleanInterval) * time.Minute
	run := func() {
		err := util.Exec(t.ctx, func() error {
			//保留份数 默认5份
			keepCount := t.keepCount
			// 获取构建成功的 并且大于5个版本的serviceId和具体的版本数
			services, err := db.GetManager().VersionInfoDao().GetServicesAndCount("success", keepCount)
			if err != nil {
				logrus.Error(err)
				return err
			}
			for _, service := range services {
				// service.Count-5： 超过指定数量的版本数,一定是正整数
				versions, err := db.GetManager().VersionInfoDao().SearchExpireVersionInfo(service.ServiceID, service.Count-keepCount)
				if err != nil {
					logrus.Error("SearchExpireVersionInfo error: ", err.Error())
					continue
				}
				for _, v := range versions {
					if v.DeliveredType == "image" {
						//clean rbd-hub images
						imageInfo := sources.ImageNameHandle(v.DeliveredPath)
						if strings.Contains(imageInfo.Host, "goodrain.me") {
							reg, err := registry.NewInsecure(imageInfo.Host, builder.REGISTRYUSER, builder.REGISTRYPASS)
							if err != nil {
								logrus.Error(err)
								continue
							} else {
								err = reg.CleanRepoByTag(imageInfo.Name, imageInfo.Tag, keepCount)
								if err != nil {
									continue
								}
							}
						}
						err := t.imageClient.ImageRemove(v.DeliveredPath)

						// 如果删除镜像失败 并且不是镜像不存在的错误
						if err != nil && !(errdefs.IsNotFound(err) || dockercli.IsErrNotFound(err)) {
							logrus.Error(err)
							continue
						}

						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Error(err)
							continue
						}
						logrus.Info("Image deletion successful:", v.DeliveredPath)
						continue
					}
					if v.DeliveredType == "slug" {
						filePath := v.DeliveredPath
						if err := os.Remove(filePath); err != nil {
							// 如果删除文件失败 并且不是文件不存在的错误
							if !errors.Is(err, os.ErrNotExist) {
								logrus.Error(err)
								continue
							}
						}
						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Error(err)
							continue
						}
						logrus.Info("file deletion successful:", filePath)
					}
				}
			}
			// only registry garbage-collect
			cmd := []string{"registry", "garbage-collect", "/etc/docker/registry/config.yml"}
			out, b, err := t.PodExecCmd(t.config, t.clientset, "rbd-hub", cmd)
			if err != nil {
				logrus.Error("rbd-hub exec cmd fail: ", out.String(), b.String(), err.Error())
			} else {
				logrus.Info("rbd-hub exec cmd success.")
			}
			return nil
		}, duration)
		if err != nil {
			errchan <- err
		}
	}
	go run()
	return nil
}

// Stop stop
func (t *Manager) Stop() error {
	logrus.Info("CleanManager is stoping.")
	t.cancel()
	return nil
}

// PodExecCmd registry garbage-collect
func (t *Manager) PodExecCmd(config *rest.Config, clientset *kubernetes.Clientset, podName string, cmd []string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"name": podName}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	pods, err := clientset.CoreV1().Pods("rbd-system").List(context.TODO(), listOptions)
	if err != nil {
		return stdout, stderr, err
	}

	for _, pod := range pods.Items {
		req := clientset.CoreV1().RESTClient().Post().
			Namespace("rbd-system").
			Resource("pods").
			Name(pod.Name).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Command: cmd,
				Stdin:   false,
				Stdout:  true,
				Stderr:  true,
				TTY:     false,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			return stdout, stderr, err
		}
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			return stdout, stderr, err
		}
		return stdout, stderr, nil
	}
	return stdout, stderr, nil
}
