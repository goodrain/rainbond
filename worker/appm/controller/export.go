package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/db"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"strings"
)

type exportController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	ctx          context.Context
	AppName      string
	AppVersion   string
	EventIDs     []string
	End          bool
}

//RainbondExport -
type RainbondExport struct {
	ImageDomain  string                       `json:"imageDomain"`
	StorageClass string                       `json:"storageClass"`
	ConfigGroups map[string]map[string]string `json:"-"`
}

func (s *exportController) Begin() {
	var r RainbondExport
	r.ConfigGroups = make(map[string]map[string]string)
	exportApp := fmt.Sprintf("%v-%v", s.AppName, s.AppVersion)
	exportPath := fmt.Sprintf("/grdata/app/helm-chart/%v/%v-helm/%v", exportApp, exportApp, s.AppName)
	for _, service := range s.appService {
		err := s.exportOne(service, &r)
		if err != nil {
			logrus.Errorf("worker export %v failure %v", service.ServiceAlias, err)
		}
	}

	if s.End {
		if len(r.ConfigGroups) != 0 {
			configGroupByte, err := yaml.Marshal(r.ConfigGroups)
			if err != nil {
				logrus.Errorf("yaml marshal valueYaml failure %v", err)
			} else {
				err = s.write(path.Join(exportPath, "values.yaml"), configGroupByte, "")
				if err != nil {
					logrus.Errorf("write values.yaml configgroup failure %v", err)
				}
			}
		}
		r.StorageClass = "rainbondvolumerwx"
		volumeYamlByte, err := yaml.Marshal(r)
		err = s.write(path.Join(exportPath, "values.yaml"), volumeYamlByte, "\n")
		if err != nil {
			logrus.Errorf("write values.yaml other failure %v", err)
		}
		err = s.write(path.Join(exportPath, "dependent_image.txt"), []byte(v1.GetOnlineProbeMeshImageName()), "\n")
		if err != nil {
			logrus.Errorf("write dependent_image.txt failure %v", err)
		}
		err = db.GetManager().ServiceEventDao().DeleteEvents(s.EventIDs)
		if err != nil {
			logrus.Errorf("delete event failure %v", err)
		}
	}
	s.manager.callback(s.controllerID, nil)
}

func (s *exportController) Stop() error {
	close(s.stopChan)
	return nil
}

func prepareExportDir(exportPath string) error {
	return os.MkdirAll(exportPath, 0755)
}

func (s *exportController) exportOne(app v1.AppService, r *RainbondExport) error {
	exportApp := fmt.Sprintf("%v-%v", s.AppName, s.AppVersion)
	exportPath := fmt.Sprintf("/grdata/app/helm-chart/%v/%v-helm/%v", exportApp, exportApp, s.AppName)
	logrus.Infof("start export app %s to helm chart spec", s.AppName)
	// Delete the old application group directory and then regenerate the application package
	if err := prepareExportDir(exportPath); err != nil {
		logrus.Errorf("prepare export dir failure %s", err.Error())
		return fmt.Errorf("create exportPath( %v )failure %v", exportPath, err)
	}
	logrus.Infof("success prepare export dir")

	exportTemplatePath := path.Join(exportPath, "templates")
	if err := prepareExportDir(exportTemplatePath); err != nil {
		return fmt.Errorf("create exportTemplatePath( %v )failure %v", exportTemplatePath, err)
	}
	logrus.Infof("success prepare helm chart template dir")

	if len(app.GetManifests()) > 0 {
		for _, manifest := range app.GetManifests() {
			manifest.SetNamespace("")
			resourceBytes, err := yaml.Marshal(manifest)
			if err != nil {
				return fmt.Errorf("manifest to yaml failure %v", err)
			}
			err = s.write(path.Join(exportTemplatePath, fmt.Sprintf("%v.yaml", manifest.GetKind())), resourceBytes, "\n---\n")
			if err != nil {
				return fmt.Errorf("write manifest yaml failure %v", err)
			}
		}
	}

	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			config.Kind = "ConfigMap"
			config.APIVersion = APIVersionConfigMap
			config.Namespace = ""
			cmBytes, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("configmap to yaml failure %v", err)
			}
			err = s.write(path.Join(exportTemplatePath, "ConfigMap.yaml"), cmBytes, "\n---\n")
			if err != nil {
				return fmt.Errorf("write configmap yaml failure %v", err)
			}
		}
	}

	for _, claim := range app.GetClaimsManually() {
		if *claim.Spec.StorageClassName != "" {
			sc := "{{ .Values.storageClass }}"
			claim.Spec.StorageClassName = &sc
		}
		claim.Kind = "PersistentVolumeClaim"
		claim.APIVersion = APIVersionPersistentVolumeClaim
		claim.Namespace = ""
		claim.Status = corev1.PersistentVolumeClaimStatus{}
		pvcBytes, err := yaml.Marshal(claim)
		if err != nil {
			return fmt.Errorf("pvc to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "PersistentVolumeClaim.yaml"), pvcBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write pvc yaml failure %v", err)
		}
	}

	if statefulset := app.GetStatefulSet(); statefulset != nil {
		statefulset.Name = app.K8sComponentName
		statefulset.Spec.Template.Name = app.K8sComponentName + "-pod-spec"
		statefulset.Kind = "StatefulSet"
		statefulset.APIVersion = APIVersionStatefulSet
		statefulset.Namespace = ""
		image := statefulset.Spec.Template.Spec.Containers[0].Image
		imageCut := strings.Split(image, "/")
		Image := fmt.Sprintf("{{ default \"%v\" .Values.imageDomain }}/%v", strings.Join(imageCut[:len(imageCut)-1], "/"), imageCut[len(imageCut)-1])
		statefulset.Spec.Template.Spec.Containers[0].Image = Image
		for i := range statefulset.Spec.VolumeClaimTemplates {
			if *statefulset.Spec.VolumeClaimTemplates[i].Spec.StorageClassName != "" {
				sc := "{{ .Values.storageClass }}"
				statefulset.Spec.VolumeClaimTemplates[i].Spec.StorageClassName = &sc
			}
		}
		statefulset.Status = appv1.StatefulSetStatus{}
		statefulsetBytes, err := yaml.Marshal(statefulset)
		if err != nil {
			return fmt.Errorf("statefulset to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "StatefulSet.yaml"), statefulsetBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write statefulset yaml failure %v", err)
		}
	}
	if deployment := app.GetDeployment(); deployment != nil {
		deployment.Name = app.K8sComponentName
		deployment.Spec.Template.Name = app.K8sComponentName + "-pod-spec"
		deployment.Kind = "Deployment"
		deployment.Namespace = ""
		deployment.APIVersion = APIVersionDeployment
		image := deployment.Spec.Template.Spec.Containers[0].Image
		imageCut := strings.Split(image, "/")
		Image := fmt.Sprintf("{{ default \"%v\" .Values.imageDomain }}/%v", strings.Join(imageCut[:len(imageCut)-1], "/"), imageCut[len(imageCut)-1])
		deployment.Spec.Template.Spec.Containers[0].Image = Image
		deployment.Status = appv1.DeploymentStatus{}
		deploymentBytes, err := yaml.Marshal(deployment)
		if err != nil {
			return fmt.Errorf("deployment to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "Deployment.yaml"), deploymentBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write deployment yaml failure %v", err)
		}
	}
	if job := app.GetJob(); job != nil {
		job.Name = app.K8sComponentName
		job.Spec.Template.Name = app.K8sComponentName + "-pod-spec"
		job.Kind = "Job"
		job.Namespace = ""
		job.APIVersion = APIVersionJob
		image := job.Spec.Template.Spec.Containers[0].Image
		imageCut := strings.Split(image, "/")
		Image := fmt.Sprintf("{{ default \"%v\" .Values.imageDomain }}/%v", strings.Join(imageCut[:len(imageCut)-1], "/"), imageCut[len(imageCut)-1])
		job.Spec.Template.Spec.Containers[0].Image = Image
		job.Status = batchv1.JobStatus{}
		jobBytes, err := yaml.Marshal(job)
		if err != nil {
			return fmt.Errorf("job to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "Job.yaml"), jobBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write job yaml failure %v", err)
		}
	}
	if cronjob := app.GetCronJob(); cronjob != nil {
		cronjob.Name = app.K8sComponentName
		cronjob.Spec.JobTemplate.Name = app.K8sComponentName
		cronjob.Spec.JobTemplate.Spec.Template.Name = app.K8sComponentName + "-pod-spec"
		cronjob.Kind = "CronJob"
		cronjob.Namespace = ""
		cronjob.APIVersion = APIVersionCronJob
		image := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
		imageCut := strings.Split(image, "/")
		Image := fmt.Sprintf("{{ default \"%v\" .Values.imageDomain }}/%v", strings.Join(imageCut[:len(imageCut)-1], "/"), imageCut[len(imageCut)-1])
		cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = Image
		cronjob.Status = batchv1.CronJobStatus{}
		cronjobBytes, err := yaml.Marshal(cronjob)
		if err != nil {
			return fmt.Errorf("cronjob to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "CronJob.yaml"), cronjobBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write cronjob yaml failure %v", err)
		}
	}
	if cronjob := app.GetBetaCronJob(); cronjob != nil {
		cronjob.Name = app.K8sComponentName
		cronjob.Spec.JobTemplate.Name = app.K8sComponentName
		cronjob.Spec.JobTemplate.Spec.Template.Name = app.K8sComponentName + "-pod-spec"
		cronjob.Kind = "CronJob"
		cronjob.APIVersion = APIVersionBetaCronJob
		cronjob.Namespace = ""
		image := cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image
		imageCut := strings.Split(image, "/")
		Image := fmt.Sprintf("{{ default \"%v\" .Values.imageDomain }}/%v", strings.Join(imageCut[:len(imageCut)-1], "/"), imageCut[len(imageCut)-1])
		cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = Image
		cronjob.Status = v1beta1.CronJobStatus{}
		cronjobBytes, err := yaml.Marshal(cronjob)
		if err != nil {
			return fmt.Errorf("cronjob to yaml failure %v", err)
		}
		err = s.write(path.Join(exportTemplatePath, "CronJob.yaml"), cronjobBytes, "\n---\n")
		if err != nil {
			return fmt.Errorf("write cronjob yaml failure %v", err)
		}
	}

	if services := app.GetServices(true); services != nil {
		for _, svc := range services {
			svc.Kind = "Service"
			svc.Namespace = ""
			svc.APIVersion = APIVersionService
			if svc.Labels["service_type"] == "outer" {
				svc.Spec.Type = corev1.ServiceTypeNodePort
			}
			svc.Status = corev1.ServiceStatus{}
			svcBytes, err := yaml.Marshal(svc)
			if err != nil {
				return fmt.Errorf("svc to yaml failure %v", err)
			}
			err = s.write(path.Join(exportTemplatePath, "Service.yaml"), svcBytes, "\n---\n")
			if err != nil {
				return fmt.Errorf("write svc yaml failure %v", err)
			}
		}
	}
	if secrets := app.GetSecrets(true); secrets != nil {
		for _, secret := range secrets {
			if len(secret.ResourceVersion) == 0 {
				secret.Kind = "Secret"
				secret.APIVersion = APIVersionSecret
				secret.Namespace = ""
				secret.Type = ""
				secretBytes, err := yaml.Marshal(secret)
				if err != nil {
					return fmt.Errorf("secret to yaml failure %v", err)
				}
				err = s.write(path.Join(exportTemplatePath, "Secret.yaml"), secretBytes, "\n---\n")
				if err != nil {
					return fmt.Errorf("write secret yaml failure %v", err)
				}
			}
		}
	}
	type SecretType string
	type YamlSecret struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
		Immutable         *bool      `json:"immutable,omitempty" protobuf:"varint,5,opt,name=immutable"`
		Data              string     `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
		StringData        string     `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`
		Type              SecretType `json:"type,omitempty" protobuf:"bytes,3,opt,name=type,casttype=SecretType"`
	}
	if s.End {
		if secrets := app.GetEnvVarSecrets(true); secrets != nil {
			for _, secret := range secrets {
				if len(secret.ResourceVersion) == 0 {
					secret.APIVersion = APIVersionSecret
					secret.Namespace = ""
					secret.Type = ""
					secret.Kind = "Secret"
					data := secret.Data
					secret.Data = nil
					var ySecret YamlSecret
					jsonSecret, err := json.Marshal(secret)
					if err != nil {
						return fmt.Errorf("json.Marshal configGroup secret failure %v", err)
					}
					err = json.Unmarshal(jsonSecret, &ySecret)
					if err != nil {
						return fmt.Errorf("json.Unmarshal configGroup secret failure %v", err)
					}
					templateConfigGroupName := strings.Split(ySecret.Name, "-")[0]
					templateConfigGroup := make(map[string]string)
					for key, value := range data {
						templateConfigGroup[key] = string(value)
					}
					r.ConfigGroups[templateConfigGroupName] = templateConfigGroup
					dataTemplate := fmt.Sprintf("  {{- range $key, $val := .Values.%v }}\n  {{ $key }}: {{ $val | quote}}\n  {{- end }}", templateConfigGroupName)
					ySecret.StringData = dataTemplate
					secretBytes, err := yaml.Marshal(ySecret)
					if err != nil {
						return fmt.Errorf("configGroup secret to yaml failure %v", err)
					}
					secretStr := strings.Replace(string(secretBytes), "|2-", "", 1)
					err = s.write(path.Join(exportTemplatePath, "Secret.yaml"), []byte(secretStr), "\n---\n")
					if err != nil {
						return fmt.Errorf("configGroup write secret yaml failure %v", err)
					}
				}
			}
		}
	}

	if hpas := app.GetHPAs(); len(hpas) != 0 {
		for _, hpa := range hpas {
			hpa.Kind = "HorizontalPodAutoscaler"
			hpa.Namespace = ""
			hpa.APIVersion = APIVersionHorizontalPodAutoscaler
			hpa.Status = v2beta2.HorizontalPodAutoscalerStatus{}
			if len(hpa.ResourceVersion) == 0 {
				hpaBytes, err := yaml.Marshal(hpa)
				if err != nil {
					return fmt.Errorf("hpa to yaml failure %v", err)
				}
				err = s.write(path.Join(exportTemplatePath, "HorizontalPodAutoscaler.yaml"), hpaBytes, "\n---\n")
				if err != nil {
					return fmt.Errorf("write hpa yaml failure %v", err)
				}
			}
		}
	}
	logrus.Infof("Create all app yaml file success, will waiting app export")
	return nil
}

//CheckFileExist check whether the file exists
func CheckFileExist(fileName string) bool {
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func (s *exportController) write(helmChartFilePath string, meta []byte, endString string) error {
	var fl *os.File
	var err error
	if CheckFileExist(helmChartFilePath) {
		fl, err = os.OpenFile(helmChartFilePath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
	} else {
		fl, err = os.Create(helmChartFilePath)
		if err != nil {
			return err
		}
	}
	defer fl.Close()
	n, err := fl.Write(append(meta, []byte(endString)...))
	if err != nil {
		return err
	}
	if n < len(append(meta, []byte(endString)...)) {
		return fmt.Errorf("write insufficient length")
	}
	return nil
}

var (
	//APIVersionSecret -
	APIVersionSecret = "v1"
	//APIVersionConfigMap -
	APIVersionConfigMap = "v1"
	//APIVersionPersistentVolumeClaim -
	APIVersionPersistentVolumeClaim = "v1"
	//APIVersionStatefulSet -
	APIVersionStatefulSet = "apps/v1"
	//APIVersionDeployment -
	APIVersionDeployment = "apps/v1"
	//APIVersionJob -
	APIVersionJob = "batch/v1"
	//APIVersionCronJob -
	APIVersionCronJob = "batch/v1"
	//APIVersionBetaCronJob -
	APIVersionBetaCronJob = "batch/v1beta1"
	//APIVersionService -
	APIVersionService = "v1"
	//APIVersionHorizontalPodAutoscaler -q
	APIVersionHorizontalPodAutoscaler = "autoscaling/v2"
	//APIVersionGateway -
	APIVersionGateway = "gateway.networking.k8s.io/v1beta1"
	//APIVersionHTTPRoute -
	APIVersionHTTPRoute = "gateway.networking.k8s.io/v1beta1"
)
