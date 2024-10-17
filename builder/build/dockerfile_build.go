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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/builder"
	jobc "github.com/goodrain/rainbond/builder/job"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"strings"
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

// The same component retains only one build task to perform
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

// Use buildkit to create a job to build an image
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
	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyOnFailure,
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/arch",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{re.Arch},
							},
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{os.Getenv("HOST_IP")},
							},
						},
					},
					},
				},
			},
		},
	}

	// only support never and onfailure
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
	secret, err := d.createAuthSecret(re)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	imageDomain, buildKitTomlCMName := sources.GetImageFirstPart(builder.REGISTRYDOMAIN)
	err = sources.PrepareBuildKitTomlCM(ctx, re.KubeClient, re.RbdNamespace, buildKitTomlCMName, imageDomain)
	if err != nil {
		return err
	}
	volumes, mounts := d.createVolumeAndMount(secret.Name, buildKitTomlCMName)
	podSpec.Volumes = volumes
	privileged := true
	container := corev1.Container{
		Name:      name,
		Image:     re.BuildKitImage,
		Stdin:     true,
		StdinOnce: true,
		Command:   []string{"buildctl-daemonless.sh"},
		Args: []string{
			"build",
			"--frontend",
			"dockerfile.v0",
			"--local",
			fmt.Sprintf("context=%v", re.SourceDir),
			"--local",
			fmt.Sprintf("dockerfile=%v", re.SourceDir),
			"--output",
			fmt.Sprintf("type=image,name=%s,push=true", buildImageName),
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
		},
	}
	if len(re.BuildKitArgs) > 0 {
		container.Args = append(container.Args, re.BuildKitArgs...)
	}
	for key := range re.BuildEnvs {
		if strings.HasPrefix(key, "ARG_") {
			envKey := strings.Replace(key, "ARG_", "", -1)
			container.Args = append(container.Args, fmt.Sprintf("--opt=build-arg:%s=%s", envKey, re.BuildEnvs[key]))
		}
	}

	container.VolumeMounts = mounts
	podSpec.Containers = append(podSpec.Containers, container)
	job.Spec = podSpec
	writer := re.Logger.GetWriter("builder", "info")
	reChan := channels.NewRingChannel(10)

	logrus.Debugf("create job[name: %s; namespace: %s]", job.Name, job.Namespace)
	err = jobc.GetJobController().ExecJob(ctx, &job, writer, reChan)
	if err != nil {
		logrus.Errorf("create new job:%s failed: %s", name, err.Error())
		return err
	}
	re.Logger.Info(util.Translation("create build code job success"), map[string]string{"step": "build-exector"})
	// delete job after complete
	defer d.deleteAuthSecret(re, secret.Name)
	defer jobc.GetJobController().DeleteJob(job.Name)
	return d.waitingComplete(re, reChan)
	return nil
}

func (d *dockerfileBuild) createVolumeAndMount(secretName string, buildKitTomlCMName string) (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	hostsFilePathType := corev1.HostPathFile
	volumes = []corev1.Volume{
		{
			Name: "dockerfile-build",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/rainbond/cache",
					Type: &hostPathType,
				},
			},
		},
		{
			Name: "grdata",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/rainbond/grdata",
					Type: &hostPathType,
				},
			},
		},
		{
			Name: "buildkittoml",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: buildKitTomlCMName},
					Items: []corev1.KeyToPath{
						{
							Key:  "buildkittoml",
							Path: "buildkitd.toml",
						},
					},
				},
			},
		},
		{
			Name: "buildkit-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
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
			Name:      "grdata",
			MountPath: "/grdata",
		},
		{
			Name:      "dockerfile-build",
			MountPath: "/cache",
		},
		{
			Name:      "buildkit-secret",
			MountPath: "/root/.docker",
		},
		{
			Name:      "hosts",
			MountPath: "/etc/hosts",
		},
		{
			Name:      "buildkittoml",
			MountPath: "/etc/buildkit",
		},
	}
	return volumes, volumeMounts
}

func (d *dockerfileBuild) createAuthSecret(re *Request) (sc corev1.Secret, err error) {
	var secret corev1.Secret
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(re.TenantID)
	if err != nil {
		logrus.Errorf("get tenant failed:%v", err.Error())
		return secret, err
	}
	secrets, err := re.KubeClient.CoreV1().Secrets(tenant.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("get secret failed:%v", err.Error())
		return secret, err
	}
	registryAuth := make(map[string]string)
	authOne := make(map[string]interface{})
	authAll := make(map[string]map[string]interface{})
	for _, secret = range secrets.Items {
		// Obtain the private warehouse configuration under goodrain.me
		if secret.Name == "rbd-hub-credentials" {
			configByte := secret.Data[".dockerconfigjson"]
			configStr := base64.StdEncoding.EncodeToString(configByte)
			config, err := base64.StdEncoding.DecodeString(configStr)
			if err != nil {
				logrus.Errorf("base64 decode domain error:%v", err.Error())
				continue
			}
			auths := make(map[string]interface{})
			err = json.Unmarshal(config, &auths)
			if err != nil {
				logrus.Debug("json unmarshal config str error:%v", err.Error())
				continue
			}
			hubConfig := auths["auths"]
			if rec, ok := hubConfig.(map[string]interface{}); ok {
				for key, val := range rec {
					authOne[key] = val
					authAll["auths"] = authOne
				}
			}
		}
		// Obtain the private warehouse configuration under the team
		if strings.Contains(secret.Name, "rbd-registry-auth") {
			domainStr := base64.StdEncoding.EncodeToString(secret.Data["Domain"])
			passwordStr := base64.StdEncoding.EncodeToString(secret.Data["Password"])
			usernameStr := base64.StdEncoding.EncodeToString(secret.Data["Username"])

			domain, err := base64.StdEncoding.DecodeString(domainStr)
			if err != nil {
				logrus.Debug("base64 decode domain error:%v", err.Error())
				continue
			}
			passWord, err := base64.StdEncoding.DecodeString(passwordStr)
			if err != nil {
				logrus.Debug("base64 decode password error:%v", err.Error())
				continue
			}
			userName, err := base64.StdEncoding.DecodeString(usernameStr)
			if err != nil {
				logrus.Debug("base64 decode username error:%v", err.Error())
				continue
			}
			registryAuth["username"] = string(userName)
			registryAuth["password"] = string(passWord)
			authOne[string(domain)] = registryAuth
			authAll["auths"] = authOne
		}
	}
	authByte, err := json.Marshal(authAll)
	if err != nil {
		logrus.Errorf("json unmarshal auth all error:%v", err.Error())
		return secret, err
	}
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("rbd-hub-auth-%v", util.NewUUID()),
			Namespace: re.RbdNamespace,
			Labels: map[string]string{
				"creator":                          "Rainbond",
				"rainbond.io/registry-auth-secret": "true",
			},
		},
		Data: map[string][]byte{
			".dockerconfigjson": authByte,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	sec, err := re.KubeClient.CoreV1().Secrets(re.RbdNamespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	if err != nil {
		logrus.Errorf("create secret error:%v", err.Error())
		return secret, nil
	}
	return *sec, nil
}

func (d *dockerfileBuild) deleteAuthSecret(re *Request, secretName string) {
	err := re.KubeClient.CoreV1().Secrets(re.RbdNamespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("delete auth secret error:%v", err.Error())
	}
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
