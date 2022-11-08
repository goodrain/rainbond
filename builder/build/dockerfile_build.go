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

package build

import (
	"context"
	"fmt"
	"github.com/eapache/channels"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"time"
)

func dockerfileBuilder() (Build, error) {
	return &dockerfileBuild{}, nil
}

type dockerfileBuild struct {
}

func (d *dockerfileBuild) Build(re *Request) (*Response, error) {
	filepath := path.Join(re.SourceDir, "Dockerfile")
	re.Logger.Info("Start parse Dockerfile", map[string]string{"step": "builder-exector"})
	_, err := sources.ParseFile(filepath)
	if err != nil {
		logrus.Error("parse dockerfile error.", err.Error())
		re.Logger.Error(fmt.Sprintf("Parse dockerfile error"), map[string]string{"step": "builder-exector"})
		return nil, err
	}
	buildImageName := CreateImageName(re.ServiceID, re.DeployVersion)
	if err := d.stopPreBuildJob(re); err != nil {
		logrus.Errorf("stop pre build job for service %s failure %s", re.ServiceID, err.Error())
	}
	if err := d.runBuildJob(re, buildImageName); err != nil {
		re.Logger.Error(util.Translation("Compiling the source code failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build dockerfile job error,", err.Error())
		return nil, err
	}
	re.Logger.Info("code build success", map[string]string{"step": "build-exector"})
	return &Response{
		MediumPath: buildImageName,
		MediumType: ImageMediumType,
	}, nil
}

//The same component retains only one build task to perform
func (d *dockerfileBuild) stopPreBuildJob(re *Request) error {
	jobList, err := jobc.GetJobController().GetServiceJobs(re.ServiceID)
	if err != nil {
		logrus.Errorf("get pre build job for service %s failure ,%s", re.ServiceID, err.Error())
	}
	if len(jobList) > 0 {
		for _, job := range jobList {
			jobc.GetJobController().DeleteJob(job.Name)
		}
	}
	return nil
}

// Use kaniko to create a job to build an image
func (d *dockerfileBuild) runBuildJob(re *Request, buildImageName string) error {
	name := fmt.Sprintf("%s-%s", re.ServiceID, re.DeployVersion)
	namespace := re.RbdNamespace
	job := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"service": re.ServiceID,
				"job":     "codebuild",
			},
		},
	}
	logrus.Debugf("dockerfile builder using hostpath, schedule job into current node")
	podSpec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyOnFailure} // only support never and onfailure
	hostIP := os.Getenv("HOST_IP")
	if hostIP != "" {
		podSpec.NodeSelector = map[string]string{
			"kubernetes.io/hostname": hostIP,
		}
		podSpec.Tolerations = []corev1.Toleration{
			{
				Operator: "Exists",
			},
		}
	}
	volumes, mounts := d.createVolumeAndMount(re)
	podSpec.Volumes = volumes
	container := corev1.Container{
		Name: name,
		//2022.11.4: latest==1.9.1
		Image:     re.KanikoImage,
		Stdin:     true,
		StdinOnce: true,
		Args:      []string{"--context=dir:///workspace", fmt.Sprintf("--destination=%s", buildImageName), "--skip-tls-verify"},
	}
	container.VolumeMounts = mounts
	podSpec.Containers = append(podSpec.Containers, container)
	job.Spec = podSpec
	writer := re.Logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logrus.Debugf("create job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err := jobc.GetJobController().ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create new job:%s failed: %s", name, err.Error())
		return err
	}
	re.Logger.Info(util.Translation("create build code job success"), map[string]string{"step": "build-exector"})
	// delete job after complete
	defer jobc.GetJobController().DeleteJob(job.Name)
	return d.waitingComplete(re, reChan)
	return nil
}

func (d *dockerfileBuild) createVolumeAndMount(re *Request) (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostsFilePathType := corev1.HostPathFile
	volumes = []corev1.Volume{
		{
			Name: "dockerfile-build",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: re.SourceDir,
					Type: &hostPathType,
				},
			},
		},
		{
			Name: "kaniko-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "rbd-hub-credentials",
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		},
		{
			Name: "hosts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/hosts",
					Type: &hostsFilePathType,
				},
			},
		},
	}
	volumeMounts = []corev1.VolumeMount{
		{
			Name:      "dockerfile-build",
			MountPath: "/workspace",
		},
		{
			Name:      "kaniko-secret",
			MountPath: "/kaniko/.docker",
		},
		{
			Name:      "hosts",
			MountPath: "/etc/hosts",
		},
	}
	return volumes, volumeMounts
}

func (d *dockerfileBuild) waitingComplete(re *Request, reChan *channels.RingChannel) (err error) {
	var logComplete = false
	var jobComplete = false
	timeout := time.NewTimer(time.Minute * 60)
	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("build time out (more than 60 minute)")
		case jobStatus := <-reChan.Out():
			status := jobStatus.(string)
			switch status {
			case "complete":
				jobComplete = true
				if logComplete {
					return nil
				}
				re.Logger.Info(util.Translation("build code job exec completed"), map[string]string{"step": "build-exector"})
			case "failed":
				jobComplete = true
				err = fmt.Errorf("build code job exec failure")
				if logComplete {
					return err
				}
				re.Logger.Info(util.Translation("build code job exec failed"), map[string]string{"step": "build-exector"})
			case "cancel":
				jobComplete = true
				err = fmt.Errorf("build code job is canceled")
				if logComplete {
					return err
				}
			case "logcomplete":
				logComplete = true
				if jobComplete {
					return err
				}
			}
		}
	}
}
