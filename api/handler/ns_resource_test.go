package handler

import (
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

type testManager struct {
	db.Manager
	tenantDao dbdao.TenantDao
}

func (m testManager) TenantDao() dbdao.TenantDao {
	return m.tenantDao
}

type testTenantDao struct {
	dbdao.TenantDao
	tenant       *dbmodel.Tenants
	err          error
	requestedFor string
}

func (d *testTenantDao) GetTenantIDByName(name string) (*dbmodel.Tenants, error) {
	d.requestedFor = name
	return d.tenant, d.err
}

// capability_id: rainbond.ns-resource.handler-singleton
func TestGetNsResourceHandlerSingleton(t *testing.T) {
	h1 := GetNsResourceHandler()
	h2 := GetNsResourceHandler()
	assert.NotNil(t, h1)
	assert.Equal(t, h1, h2)
}

// capability_id: rainbond.ns-resource.mark-source
func TestInjectSourceLabelYaml(t *testing.T) {
	labels := map[string]string{}
	injectSourceLabel(labels, "yaml")
	assert.Equal(t, "yaml", labels["rainbond.io/source"])
}

// capability_id: rainbond.ns-resource.mark-source
func TestInjectSourceLabelManual(t *testing.T) {
	labels := map[string]string{}
	injectSourceLabel(labels, "manual")
	assert.Equal(t, "manual", labels["rainbond.io/source"])
}

// capability_id: rainbond.ns-resource.detect-source
func TestDetectResourceSource(t *testing.T) {
	tests := []struct {
		labels   map[string]string
		expected string
	}{
		{map[string]string{"app.kubernetes.io/managed-by": "Helm"}, "helm"},
		{map[string]string{"rainbond.io/source": "yaml"}, "yaml"},
		{map[string]string{"rainbond.io/source": "manual"}, "manual"},
		{map[string]string{}, "external"},
		{nil, "external"},
	}
	for _, tt := range tests {
		result := detectResourceSource(tt.labels)
		assert.Equal(t, tt.expected, result)
	}
}

// capability_id: rainbond.ns-resource.resolve-tenant-namespace
func TestGetTenantNamespaceUsesNamespaceField(t *testing.T) {
	tenantDao := &testTenantDao{
		tenant: &dbmodel.Tenants{
			Name:      "demo-team",
			UUID:      "tenant-uuid",
			Namespace: "tenant-namespace",
		},
	}
	db.SetTestManager(testManager{tenantDao: tenantDao})
	defer db.SetTestManager(nil)

	ns, err := GetNsResourceHandler().getTenantNamespace("demo-team")
	assert.NoError(t, err)
	assert.Equal(t, "demo-team", tenantDao.requestedFor)
	assert.Equal(t, "tenant-namespace", ns)
}

// capability_id: rainbond.ns-resource.resolve-tenant-namespace
func TestGetTenantNamespaceFallsBackToUUIDWhenNamespaceEmpty(t *testing.T) {
	tenantDao := &testTenantDao{
		tenant: &dbmodel.Tenants{
			Name: "demo-team",
			UUID: "tenant-uuid",
		},
	}
	db.SetTestManager(testManager{tenantDao: tenantDao})
	defer db.SetTestManager(nil)

	ns, err := GetNsResourceHandler().getTenantNamespace("demo-team")
	assert.NoError(t, err)
	assert.Equal(t, "demo-team", tenantDao.requestedFor)
	assert.Equal(t, "tenant-uuid", ns)
}

// capability_id: rainbond.ns-resource.batch-create
func TestCreateNsResourceBatchAggregatesPartialSuccess(t *testing.T) {
	tenantDao := &testTenantDao{
		tenant: &dbmodel.Tenants{
			Name:      "demo-team",
			UUID:      "tenant-uuid",
			Namespace: "team-namespace",
		},
	}
	db.SetTestManager(testManager{tenantDao: tenantDao})
	defer db.SetTestManager(nil)

	scheme := runtime.NewScheme()
	assert.NoError(t, appsv1.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, rbacv1.AddToScheme(scheme))

	client := dynamicfake.NewSimpleDynamicClient(scheme, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-service",
			Namespace: "team-namespace",
		},
	})
	mapper := newTestNsResourceMapper(
		appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace,
		corev1.SchemeGroupVersion.WithKind("Service"), meta.RESTScopeNamespace,
		rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"), meta.RESTScopeRoot,
	)

	originalMapper := nsResourceRESTMapper
	originalDynamicClient := nsResourceDynamicClient
	nsResourceRESTMapper = func() meta.RESTMapper { return mapper }
	nsResourceDynamicClient = func() dynamic.Interface { return client }
	defer func() {
		nsResourceRESTMapper = originalMapper
		nsResourceDynamicClient = originalDynamicClient
	}()

	result, statusCode, err := GetNsResourceHandler().CreateNsResource("demo-team", "yaml", []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-deploy
spec:
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
        - name: demo
          image: nginx
---
apiVersion: v1
kind: Service
metadata:
  name: demo-service
spec:
  selector:
    app: demo
  ports:
    - port: 80
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: demo-binding
subjects: []
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
`))

	assert.NoError(t, err)
	assert.Equal(t, 207, statusCode)
	assert.Equal(t, 3, result.Summary.Total)
	assert.Equal(t, 2, result.Summary.SuccessCount)
	assert.Equal(t, 1, result.Summary.FailureCount)
	assert.True(t, result.Summary.PartialSuccess)
	assert.Len(t, result.Results, 3)
	assert.Equal(t, "Deployment", result.Results[0].Kind)
	assert.Equal(t, "team-namespace", result.Results[0].Namespace)
	assert.True(t, result.Results[0].Success)
	assert.Equal(t, "Service", result.Results[1].Kind)
	assert.False(t, result.Results[1].Success)
	assert.Contains(t, result.Results[1].Message, "already exists")
	assert.Equal(t, "cluster", result.Results[2].ResourceScope)
	assert.True(t, result.Results[2].Success)

	deployment, err := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).
		Namespace("team-namespace").
		Get(t.Context(), "demo-deploy", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "yaml", deployment.GetLabels()["rainbond.io/source"])

	clusterRoleBinding, err := client.Resource(schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterrolebindings",
	}).Get(t.Context(), "demo-binding", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "yaml", clusterRoleBinding.GetLabels()["rainbond.io/source"])
	assert.Equal(t, "", clusterRoleBinding.GetNamespace())
}

// capability_id: rainbond.ns-resource.batch-create
func TestCreateNsResourceBatchPreservesExplicitNamespace(t *testing.T) {
	tenantDao := &testTenantDao{
		tenant: &dbmodel.Tenants{
			Name:      "demo-team",
			UUID:      "tenant-uuid",
			Namespace: "team-namespace",
		},
	}
	db.SetTestManager(testManager{tenantDao: tenantDao})
	defer db.SetTestManager(nil)

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	client := dynamicfake.NewSimpleDynamicClient(scheme)
	mapper := newTestNsResourceMapper(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)

	originalMapper := nsResourceRESTMapper
	originalDynamicClient := nsResourceDynamicClient
	nsResourceRESTMapper = func() meta.RESTMapper { return mapper }
	nsResourceDynamicClient = func() dynamic.Interface { return client }
	defer func() {
		nsResourceRESTMapper = originalMapper
		nsResourceDynamicClient = originalDynamicClient
	}()

	result, statusCode, err := GetNsResourceHandler().CreateNsResource("demo-team", "manual", []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: explicit-config
  namespace: custom-namespace
data:
  demo: "1"
`))

	assert.NoError(t, err)
	assert.Equal(t, 200, statusCode)
	assert.Equal(t, "custom-namespace", result.Results[0].Namespace)

	configMap, err := client.Resource(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}).
		Namespace("custom-namespace").
		Get(t.Context(), "explicit-config", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "custom-namespace", configMap.GetNamespace())
	assert.Equal(t, "manual", configMap.GetLabels()["rainbond.io/source"])
}

func newTestNsResourceMapper(entries ...interface{}) meta.RESTMapper {
	groupVersions := make([]schema.GroupVersion, 0)
	for i := 0; i < len(entries); i += 2 {
		groupVersions = append(groupVersions, entries[i].(schema.GroupVersionKind).GroupVersion())
	}
	mapper := meta.NewDefaultRESTMapper(groupVersions)
	for i := 0; i < len(entries); i += 2 {
		mapper.Add(entries[i].(schema.GroupVersionKind), entries[i+1].(meta.RESTScope))
	}
	return mapper
}
