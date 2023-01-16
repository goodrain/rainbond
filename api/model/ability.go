package model

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// UpdateAbilityReq -
type UpdateAbilityReq struct {
	//AbilityID string                    `json:"ability_id" validate:"required"`
	Object *unstructured.Unstructured `json:"object" validate:"required"`
}

// AbilityResp -
type AbilityResp struct {
	Name       string `json:"name"`
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	AbilityID  string `json:"ability_id"`
}
