package model

import "github.com/goodrain/rainbond/db/model"

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
	Status                string `json:"status"`
	DetectionMethod       string `json:"detection_method"`
	UnhealthyHandleMethod string `json:"unhealthy_handle_method"`
}

//TelescopicManagement -
type TelescopicManagement struct {
	MinReplicas int32  `json:"min_replicas"`
	MaxReplicas int32  `json:"max_replicas"`
	CPUUse      string `json:"cpu_use"`
	MemoryUse   string `json:"memory_use"`
}

//Toleration -
type Toleration struct {
}

//SpecialManagement -
type SpecialManagement struct {
	NodeSelector []map[string]string `json:"node_selector"`
	Label        []map[string]string `json:"label"`
	Toleration   []map[string]string `json:"toleration"`
}

//ConvertResource -
type ConvertResource struct {
	ComponentsName         string                 `json:"components_name"`
	BasicManagement        BasicManagement        `json:"basic_management"`
	PortManagement         []PortManagement       `json:"port_management"`
	ENVManagement          []ENVManagement        `json:"env_management"`
	ConfigManagement       []ConfigManagement     `json:"config_management"`
	HealthyCheckManagement HealthyCheckManagement `json:"health_check_management"`
	TelescopicManagement   TelescopicManagement   `json:"telescopic_management"`
	SpecialManagement      SpecialManagement      `json:"special_management"`
}

//ComponentAttributes -
type ComponentAttributes struct {
	Ct     *model.TenantServices `json:"ct"`
	Image  string                `json:"image"`
	Cmd    string                `json:"cmd"`
	ENV    []ENVManagement       `json:"env"`
	Config []ConfigManagement    `json:"config"`
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
