package handler

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
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

// capability_id: rainbond.vm-runtime.password-hash-input-boundary
func TestVMLoginPasswordHashAttributeAcceptsConsoleStringEncoding(t *testing.T) {
	hash := generatedVMLoginPasswordHash(t)
	encoded, err := json.Marshal(hash)
	if err != nil {
		t.Fatal("could not encode test hash")
	}
	for _, value := range []string{hash, string(encoded)} {
		attr := &apimodel.ComponentK8sAttribute{
			Name:           "vm_login_password_hash",
			SaveType:       "string",
			AttributeValue: value,
		}
		if err := normalizeVMLoginCredentialAttribute(attr); err != nil {
			t.Fatal("valid logical hash was rejected")
		}
		if attr.AttributeValue != hash || attr.SaveType != "string" {
			t.Fatal("hash attribute was not normalized to the logical string value")
		}
	}
	invalid := &apimodel.ComponentK8sAttribute{Name: "vm_login_password_hash", SaveType: "string", AttributeValue: "not-a-hash"}
	if err := normalizeVMLoginCredentialAttribute(invalid); err == nil || strings.Contains(err.Error(), invalid.AttributeValue) {
		t.Fatal("invalid hash was accepted or echoed in an error")
	}
}

// capability_id: rainbond.vm-runtime.password-hash-input-boundary
func TestPlainVMLoginPasswordAttributeRejectedBeforePersistence(t *testing.T) {
	plain := generatedVMLoginPlaintext(t)
	attribute := apimodel.ComponentK8sAttribute{Name: "vm_login_password", SaveType: "string", AttributeValue: plain}
	action := &ServiceAction{}
	assertRejectedWithoutCredentialEcho := func(call func() error) {
		t.Helper()
		defer func() {
			if recover() != nil {
				t.Fatal("plaintext attribute reached database access before rejection")
			}
		}()
		err := call()
		if err == nil || strings.Contains(err.Error(), plain) {
			t.Fatal("plaintext VM password attribute was accepted or echoed")
		}
	}
	assertRejectedWithoutCredentialEcho(func() error { return action.CreateK8sAttribute("tenant-a", "service-a", &attribute) })
	assertRejectedWithoutCredentialEcho(func() error { return action.UpdateK8sAttribute("service-a", &attribute) })
	assertRejectedWithoutCredentialEcho(func() error {
		return action.ServiceCreate(&apimodel.ServiceStruct{ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{attribute}})
	})
	invalidHash := apimodel.ComponentK8sAttribute{Name: "vm_login_password_hash", SaveType: "string", AttributeValue: "not-a-hash"}
	assertRejectedWithoutCredentialEcho(func() error {
		return action.ServiceCreate(&apimodel.ServiceStruct{ComponentK8sAttributes: []apimodel.ComponentK8sAttribute{invalidHash}})
	})
}

func generatedVMLoginPasswordHash(t *testing.T) string {
	t.Helper()
	salt := make([]byte, 8)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatal("could not generate test hash material")
	}
	return "$6$rounds=500000$" + hex.EncodeToString(salt) + "$" + strings.Repeat("A", 86)
}

func generatedVMLoginPlaintext(t *testing.T) string {
	t.Helper()
	value := make([]byte, 16)
	if _, err := cryptorand.Read(value); err != nil {
		t.Fatal("could not generate test value")
	}
	return hex.EncodeToString(value)
}
