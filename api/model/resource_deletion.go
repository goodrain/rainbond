package model

// K8sResourceDeletionRequest describes an application resource deletion operation.
type K8sResourceDeletionRequest struct {
	AppID        string           `json:"app_id" validate:"required"`
	CascadeCRD   bool             `json:"cascade_crd"`
	K8sResources []HandleResource `json:"k8s_resources" validate:"required"`
}

// CRDDeletionImpact describes the cluster-wide impact of deleting one CRD.
type CRDDeletionImpact struct {
	Name                 string   `json:"name"`
	Group                string   `json:"group"`
	Version              string   `json:"version"`
	Kind                 string   `json:"kind"`
	Plural               string   `json:"plural"`
	Scope                string   `json:"scope"`
	CurrentAppCRCount    int      `json:"current_app_cr_count"`
	OtherAppCRCount      int      `json:"other_app_cr_count"`
	UnownedCRCount       int      `json:"unowned_cr_count"`
	AffectedRegionAppIDs []string `json:"affected_region_app_ids"`
}

// K8sResourceDeletionImpact summarizes a deletion plan without mutating resources.
type K8sResourceDeletionImpact struct {
	HasCRD          bool                `json:"has_crd"`
	RequiresCascade bool                `json:"requires_cascade"`
	CRDCount        int                 `json:"crd_count"`
	CRCount         int                 `json:"cr_count"`
	OtherAppCount   int                 `json:"other_app_count"`
	UnownedCRCount  int                 `json:"unowned_cr_count"`
	CRDs            []CRDDeletionImpact `json:"crds"`
}

// K8sResourceDeletionResult reports a confirmed deletion.
type K8sResourceDeletionResult struct {
	Status           string              `json:"status"`
	DeletedClientIDs []string            `json:"deleted_client_ids"`
	CascadedCRDs     []CRDDeletionImpact `json:"cascaded_crds"`
}

// K8sResourceReconcileRequest describes resources that should be checked against Kubernetes.
type K8sResourceReconcileRequest struct {
	AppID        string           `json:"app_id" validate:"required"`
	K8sResources []HandleResource `json:"k8s_resources"`
}

// K8sResourceUnknown describes a resource whose existence could not be confirmed.
type K8sResourceUnknown struct {
	ClientID string `json:"client_id"`
	Error    string `json:"error"`
}

// K8sResourceReconcileResult reports missing resources and inconclusive checks.
type K8sResourceReconcileResult struct {
	MissingClientIDs []string             `json:"missing_client_ids"`
	Unknown          []K8sResourceUnknown `json:"unknown"`
}
