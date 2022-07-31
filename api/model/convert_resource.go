package model

import (
	"github.com/goodrain/rainbond/db/model"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//BasicManagement -
type BasicManagement struct {
	ResourceType string      `json:"resource_type"`
	Replicas     *int32      `json:"replicas"`
	Image        string      `json:"image"`
	Memory       int64       `json:"memory"`
	Cmd          string      `json:"command"`
	CPU          int64       `json:"cpu"`
	JobStrategy  JobStrategy `json:"job_strategy"`
}

//PortManagement -
type PortManagement struct {
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
	Inner    bool   `json:"inner"`
	Outer    bool   `json:"outer"`
}

//ENVManagement -
type ENVManagement struct {
	ENVKey     string `json:"env_key"`
	ENVValue   string `json:"env_value"`
	ENVExplain string `json:"env_explain"`
}

//ConfigManagement -
type ConfigManagement struct {
	ConfigName  string `json:"config_name"`
	ConfigPath  string `json:"config_path"`
	Mode        int32  `json:"mode"`
	ConfigValue string `json:"config_value"`
}

//HealthyCheckManagement -
type HealthyCheckManagement struct {
	Status             int    `json:"status"`
	ProbeID            string `json:"probe_id"`
	Port               int    `json:"port"`
	Path               string `json:"path"`
	HTTPHeader         string `json:"http_header"`
	Command            string `json:"cmd"`
	DetectionMethod    string `json:"detection_method"`
	Mode               string `json:"mode"`
	InitialDelaySecond int    `json:"initial_delay_second"`
	PeriodSecond       int    `json:"period_second"`
	TimeoutSecond      int    `json:"timeout_second"`
	SuccessThreshold   int    `json:"success_threshold"`
	FailureThreshold   int    `json:"failure_threshold"`
}

//TelescopicManagement -
type TelescopicManagement struct {
	Enable      bool                                        `json:"enable"`
	RuleID      string                                      `json:"rule_id"`
	MinReplicas int32                                       `json:"min_replicas"`
	MaxReplicas int32                                       `json:"max_replicas"`
	CPUOrMemory []*model.TenantServiceAutoscalerRuleMetrics `json:"cpu_or_memory"`
}

//KubernetesResources -
type KubernetesResources struct {
	Name                       string                  `json:"name"`
	Spec                       v1.ServiceSpec          `json:"spec"`
	Namespace                  string                  `json:"namespace"`
	Labels                     map[string]string       `json:"labels"`
	Annotations                map[string]string       `json:"annotations"`
	Kind                       string                  `json:"kind"`
	APIVersion                 string                  `json:"api_version"`
	GenerateName               string                  `json:"generate_name"`
	UID                        types.UID               `json:"uid"`
	ResourceVersion            string                  `json:"resource_version"`
	Generation                 int64                   `json:"generation"`
	CreationTimestamp          metav1.Time             `json:"creation_timestamp"`
	DeletionTimestamp          *metav1.Time            `json:"deletion_timestamp"`
	DeletionGracePeriodSeconds *int64                  `json:"deletion_grace_period_seconds"`
	OwnerReferences            []metav1.OwnerReference `json:"owner_references"`
	Finalizers                 []string                `json:"finalizers"`
	ClusterName                string                  `json:"cluster_name"`
}

//ApplicationResource -
type ApplicationResource struct {
	KubernetesResources []model.K8sResource `json:"kubernetes_resources"`
	ConvertResource     []ConvertResource   `json:"convert_resource"`
}

//ConvertResource -
type ConvertResource struct {
	ComponentsName                   string                          `json:"components_name"`
	BasicManagement                  BasicManagement                 `json:"basic_management"`
	PortManagement                   []PortManagement                `json:"port_management"`
	ENVManagement                    []ENVManagement                 `json:"env_management"`
	ConfigManagement                 []ConfigManagement              `json:"config_management"`
	HealthyCheckManagement           HealthyCheckManagement          `json:"health_check_management"`
	TelescopicManagement             TelescopicManagement            `json:"telescopic_management"`
	ComponentK8sAttributesManagement []*model.ComponentK8sAttributes `json:"component_k8s_attributes_management"`
}

//ComponentAttributes -
type ComponentAttributes struct {
	TS                     *model.TenantServices           `json:"ts"`
	Image                  string                          `json:"image"`
	Cmd                    string                          `json:"cmd"`
	ENV                    []ENVManagement                 `json:"env"`
	Config                 []ConfigManagement              `json:"config"`
	Port                   []PortManagement                `json:"port"`
	Telescopic             TelescopicManagement            `json:"telescopic"`
	HealthyCheck           HealthyCheckManagement          `json:"healthy_check"`
	ComponentK8sAttributes []*model.ComponentK8sAttributes `json:"component_k8s_attributes"`
}

//AppComponent -
type AppComponent struct {
	App          *model.Application    `json:"app"`
	K8sResources []model.K8sResource   `json:"k8s_resources"`
	Component    []ComponentAttributes `json:"component"`
}

//ReturnResourceImport -
type ReturnResourceImport struct {
	Tenant *model.Tenants `json:"tenant"`
	App    []AppComponent `json:"app"`
}
