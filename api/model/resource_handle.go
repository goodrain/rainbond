package model

//LabelResource -
type LabelResource struct {
	Workloads WorkLoadsResource   `json:"workloads,omitempty"`
	Others    OtherResource       `json:"others,omitempty"`
	UnSupport map[string][]string `json:"un_support"`
	Status    string              `json:"status"`
}

//LabelWorkloadsResourceProcess -
type LabelWorkloadsResourceProcess struct {
	Deployments  map[string][]string `json:"deployments,omitempty"`
	Jobs         map[string][]string `json:"jobs,omitempty"`
	CronJobs     map[string][]string `json:"cronJobs,omitempty"`
	StateFulSets map[string][]string `json:"stateFulSets,omitempty"`
}

//LabelOthersResourceProcess -
type LabelOthersResourceProcess struct {
	Services                 map[string][]string `json:"services,omitempty"`
	PVC                      map[string][]string `json:"PVC,omitempty"`
	Ingresses                map[string][]string `json:"ingresses,omitempty"`
	NetworkPolicies          map[string][]string `json:"networkPolicies,omitempty"`
	ConfigMaps               map[string][]string `json:"configMaps,omitempty"`
	Secrets                  map[string][]string `json:"secrets,omitempty"`
	ServiceAccounts          map[string][]string `json:"serviceAccounts,omitempty"`
	RoleBindings             map[string][]string `json:"roleBindings,omitempty"`
	HorizontalPodAutoscalers map[string][]string `json:"horizontalPodAutoscalers,omitempty"`
	Roles                    map[string][]string `json:"roles,omitempty"`
}

//WorkLoadsResource -
type WorkLoadsResource struct {
	Deployments  []string `json:"deployments,omitempty"`
	Jobs         []string `json:"jobs,omitempty"`
	CronJobs     []string `json:"cron_jobs,omitempty"`
	StateFulSets []string `json:"state_ful_sets,omitempty"`
}

//OtherResource -
type OtherResource struct {
	Services                 []string `json:"services,omitempty"`
	PVC                      []string `json:"pvc,omitempty"`
	Ingresses                []string `json:"ingresses,omitempty"`
	NetworkPolicies          []string `json:"network_policies,omitempty"`
	ConfigMaps               []string `json:"config_maps,omitempty"`
	Secrets                  []string `json:"secrets,omitempty"`
	ServiceAccounts          []string `json:"service_accounts,omitempty"`
	RoleBindings             []string `json:"role_bindings,omitempty"`
	HorizontalPodAutoscalers []string `json:"horizontal_pod_autoscalers,omitempty"`
	Roles                    []string `json:"roles,omitempty"`
}
