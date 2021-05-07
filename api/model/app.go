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
	Status    string            `json:"status"`
	Cpu       *int64            `json:"cpu"`
	Memory    *int64            `json:"memory"`
	Disk      int64             `json:"disk"`
	Phase     string            `json:"phase"`
	Values    map[string]string `json:"values"`
	Readme    string            `json:"readme"`
	Version   string            `json:"version"`
	Revision  int               `json:"revision"`
	Overrides []string          `json:"overrides"`
	Questions string            `json:"questions"`
}

// AppService -
type AppService struct {
	ServiceName string    `json:"service_name"`
	Address     string    `json:"address"`
	TCPPorts    []int32   `json:"tcp_ports"`
	Pods        []*AppPod `json:"pods"`
}

// ByServiceName implements sort.Interface for []*AppService based on
// the ServiceName field.
type ByServiceName []*AppService

func (a ByServiceName) Len() int           { return len(a) }
func (a ByServiceName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByServiceName) Less(i, j int) bool { return a[i].ServiceName < a[j].ServiceName }

type AppPod struct {
	PodName   string `json:"pod_name"`
	PodStatus string `json:"pod_status"`
}

// ByPodName implements sort.Interface for []*AppPod based on
// the PodName field.
type ByPodName []*AppPod

func (a ByPodName) Len() int           { return len(a) }
func (a ByPodName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPodName) Less(i, j int) bool { return a[i].PodName < a[j].PodName }
