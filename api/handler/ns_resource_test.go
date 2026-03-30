package handler

import (
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
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
