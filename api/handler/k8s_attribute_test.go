package handler

import (
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.worker.conversion.cmd-args-yaml
func TestUpdateK8sAttributeUpdatesSaveType(t *testing.T) {
	attrDao := &resourceSyncComponentK8sAttributeDao{
		attributes: map[string]*dbmodel.ComponentK8sAttributes{
			dbmodel.K8sAttributeNameCmd: {
				ComponentID:    "service-a",
				Name:           dbmodel.K8sAttributeNameCmd,
				SaveType:       "string",
				AttributeValue: "/entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done",
			},
		},
	}
	db.SetTestManager(resourceSyncTestManager{attributeDao: attrDao})
	defer db.SetTestManager(nil)

	nextValue := "- /entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done\n"
	action := &ServiceAction{}
	err := action.UpdateK8sAttribute("service-a", &apimodel.ComponentK8sAttribute{
		Name:           dbmodel.K8sAttributeNameCmd,
		SaveType:       "yaml",
		AttributeValue: nextValue,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated := attrDao.attributes[dbmodel.K8sAttributeNameCmd]
	if updated.SaveType != "yaml" {
		t.Fatalf("expected save_type yaml, got %q", updated.SaveType)
	}
	if updated.AttributeValue != nextValue {
		t.Fatalf("expected updated attribute value %q, got %q", nextValue, updated.AttributeValue)
	}
}
