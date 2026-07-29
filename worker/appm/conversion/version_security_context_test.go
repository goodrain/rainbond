package conversion

import (
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	typesv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

// capability_id: rainbond.worker.conversion.pod-security-context
func TestCreatePodSecurityContextUsesK8sAttribute(t *testing.T) {
	app := &typesv1.AppService{
		AppServiceBase: typesv1.AppServiceBase{
			ServiceID: "service-1",
		},
	}
	manager := hostNetworkTestManager{
		attributeDao: hostNetworkAttributeDao{
			attributes: map[string]*dbmodel.ComponentK8sAttributes{
				dbmodel.K8sAttributeNamePodSecurityContext: {
					Name: dbmodel.K8sAttributeNamePodSecurityContext,
					AttributeValue: strings.Join([]string{
						"runAsNonRoot: true",
						"runAsUser: 10001",
						"runAsGroup: 10001",
						"seccompProfile:",
						"  type: RuntimeDefault",
					}, "\n"),
				},
			},
		},
	}

	securityContext, err := createPodSecurityContext(app, manager)
	if err != nil {
		t.Fatalf("createPodSecurityContext() error = %v", err)
	}
	if securityContext == nil {
		t.Fatalf("expected pod security context")
	}
	if securityContext.RunAsNonRoot == nil || !*securityContext.RunAsNonRoot {
		t.Fatalf("expected runAsNonRoot=true")
	}
	if securityContext.RunAsUser == nil || *securityContext.RunAsUser != 10001 {
		t.Fatalf("expected runAsUser=10001, got %v", securityContext.RunAsUser)
	}
	if securityContext.RunAsGroup == nil || *securityContext.RunAsGroup != 10001 {
		t.Fatalf("expected runAsGroup=10001, got %v", securityContext.RunAsGroup)
	}
	if securityContext.SeccompProfile == nil || securityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("expected RuntimeDefault seccomp profile")
	}
}
