package model

import (
	"github.com/goodrain/rainbond/db/model"
)

//BasicManagement -
type BasicManagement struct {
	ResourceType string `json:"resource_type"`
	Replicas     int32  `json:"replicas"`
	Image        string `json:"image"`
	Memory       int64  `json:"memory"`
	Cmd          string `json:"command"`
	CPU          int64  `json:"cpu"`
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
	HttpHeader         string `json:"http_header"`
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
	CpuOrMemory []*model.TenantServiceAutoscalerRuleMetrics `json:"cpu_or_memory"`
}

type KubernetesResources struct {
}
type ApplicationResource struct {
	KubernetesResources KubernetesResources `json:"kubernetes_resources"`
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
	Ct                     *model.TenantServices           `json:"ct"`
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
	App       *model.Application    `json:"app"`
	Component []ComponentAttributes `json:"component"`
}

//ReturnResourceImport -
type ReturnResourceImport struct {
	Tenant *model.Tenants `json:"tenant"`
	App    []AppComponent `json:"app"`
}
