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
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/util"
	utils "github.com/goodrain/rainbond/util"
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
func CreateCleanManager(imageClient sources.ImageClient) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Manager{
		imageClient:   imageClient,
		ctx:           ctx,
		cancel:        cancel,
		config:        k8s.Default().RestConfig,
		keepCount:     uint(configs.Default().ChaosConfig.KeepCount),
		clientset:     k8s.Default().Clientset,
		cleanInterval: configs.Default().ChaosConfig.CleanInterval,
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
			logrus.Infof("[clean] start cleanup task, keep count: %d, querying components with more than %d success versions", keepCount, keepCount)
			// 获取构建成功的 并且大于5个版本的serviceId和具体的版本数
			services, err := db.GetManager().VersionInfoDao().GetServicesAndCount("success", keepCount)
			if err != nil {
				logrus.Error("[clean] failed to query components with excess versions: ", err)
				return nil
			}
			logrus.Infof("[clean] found %d components with more than %d versions", len(services), keepCount)
			for _, service := range services {
				needDelete := service.Count - keepCount
				logrus.Infof("[clean] component %s has %d success versions, need to delete oldest %d", service.ServiceID, service.Count, needDelete)
				// service.Count-5： 超过指定数量的版本数,一定是正整数
				versions, err := db.GetManager().VersionInfoDao().SearchExpireVersionInfo(service.ServiceID, needDelete)
				if err != nil {
					logrus.Errorf("[clean] failed to query expired versions for component %s: %s", service.ServiceID, err.Error())
					continue
				}
				logrus.Infof("[clean] component %s has %d expired versions to delete", service.ServiceID, len(versions))
				for _, v := range versions {
					logrus.Infof("[clean] version to delete: ID=%s, DeliveredType=%q, FinalStatus=%s, Path=%s", v.BuildVersion, v.DeliveredType, v.FinalStatus, v.DeliveredPath)
					if v.DeliveredType == "image" {
						// 跳过系统镜像（builder 和 runner）
						if strings.Contains(strings.ToLower(v.DeliveredPath), "builder") ||
							strings.Contains(strings.ToLower(v.DeliveredPath), "runner") {
							logrus.Infof("[clean] skipping system image: %s", v.DeliveredPath)
							continue
						}

						//clean rbd-hub images
						imageInfo := sources.ImageNameHandle(v.DeliveredPath)
						logrus.Infof("[clean] processing image version ID=%s, path=%s, host=%s", v.BuildVersion, v.DeliveredPath, imageInfo.Host)
						if strings.Contains(imageInfo.Host, "goodrain.me") {
							logrus.Infof("[clean] deleting tag from rbd-hub registry: %s/%s", imageInfo.Name, imageInfo.Tag)
							reg, err := registry.NewInsecure(imageInfo.Host, builder.REGISTRYUSER, builder.REGISTRYPASS)
							if err != nil {
								logrus.Errorf("[clean] failed to connect rbd-hub: %v", err)
								continue
							} else {
								err = reg.CleanRepoByTag(imageInfo.Name, imageInfo.Tag, keepCount)
								if err != nil {
									logrus.Errorf("[clean] failed to delete rbd-hub tag %s:%s, err: %v", imageInfo.Name, imageInfo.Tag, err)
									continue
								}
							}
						} else {
							logrus.Infof("[clean] image host is not goodrain.me, skip registry cleanup: %s", imageInfo.Host)
						}
						logrus.Infof("[clean] removing local image: %s", v.DeliveredPath)
						err := t.imageClient.ImageRemove(v.DeliveredPath)

						// 如果删除镜像失败 并且不是镜像不存在的错误
						if err != nil && !(errdefs.IsNotFound(err) || dockercli.IsErrNotFound(err)) {
							logrus.Errorf("[clean] failed to remove local image %s: %v", v.DeliveredPath, err)
							continue
						}

						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Errorf("[clean] failed to delete version record %s: %v", v.BuildVersion, err)
							continue
						}
						logrus.Infof("[clean] image version deleted successfully: %s", v.DeliveredPath)
						continue
					}
					if v.DeliveredType == "slug" {
						filePath := v.DeliveredPath
						logrus.Infof("[clean] deleting slug file: %s", filePath)
						if err := os.Remove(filePath); err != nil {
							// 如果删除文件失败 并且不是文件不存在的错误
							if !errors.Is(err, os.ErrNotExist) {
								logrus.Errorf("[clean] failed to delete slug file %s: %v", filePath, err)
								continue
							}
							logrus.Infof("[clean] slug file not found, skipping: %s", filePath)
						}
						if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(v); err != nil {
							logrus.Errorf("[clean] failed to delete version record %s: %v", v.BuildVersion, err)
							continue
						}
						logrus.Infof("[clean] slug file deleted successfully: %s", filePath)
					}
				}
			}
			// only registry garbage-collect
			logrus.Info("[clean] running rbd-hub registry garbage-collect")
			cmd := []string{"registry", "garbage-collect", "/etc/docker/registry/config.yml"}
			out, b, err := t.PodExecCmd(t.config, t.clientset, "rbd-hub", cmd)
			if err != nil {
				logrus.Error("[clean] rbd-hub garbage-collect failed: ", out.String(), b.String(), err.Error())
			} else {
				logrus.Info("[clean] rbd-hub garbage-collect succeeded")
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
	pods, err := clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).List(context.TODO(), listOptions)
	if err != nil {
		return stdout, stderr, err
	}

	for _, pod := range pods.Items {
		req := clientset.CoreV1().RESTClient().Post().
			Namespace(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).
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
