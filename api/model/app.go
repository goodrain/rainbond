package model

// AppPort -
type AppPort struct {
	ServiceID      string `json:"service_id" validate:"required"`
	ContainerPort  int    `json:"container_port" validate:"required"`
	PortAlias      string `json:"port_alias" validate:"required"`
	K8sServiceName string `json:"k8s_service_name" validate:"required"`
}

// AppStatus -
type AppStatus struct {
	Status         string `json:"status"`
	Cpu            int64  `json:"cpu"`
	Memory         int64  `json:"memory"`
	Disk           int64  `json:"disk"`
	Phase          string `json:"phase"`
	ValuesTemplate string `json:"valuesTemplate"`
	Readme         string `json:"readme"`
}

// AppDetectProcess -
type AppDetectProcess struct {
	Type  string `json:"type"`
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}

// AppService -
type AppService struct {
	ServiceName string    `json:"service_name"`
	TCPPorts    []int32   `json:"tcp_ports"`
	UDPPorts    []int32   `json:"udp_ports"`
	Pods        []*AppPod `json:"pods"`
}

type AppPod struct {
	PodName   string `json:"pod_name"`
	PodStatus string `json:"pod_status"`
}
