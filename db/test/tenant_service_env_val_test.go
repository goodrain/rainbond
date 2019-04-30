package db_test

import (
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/db/test/fixtures"
)

func TestTenantServiceEnvVarDaoImpl_DelByServiceIDAndScope(t *testing.T) {
	err := fixtures.InitDBManager()
	if err != nil {
		t.Fatal(err)
	}

	sid := "1994d780485842a189441ed3eaa78d3b"
	innerEnv := &model.TenantServiceEnvVar{
		TenantID:      "c1a29fe4d7b0413993dc859430cf743d",
		ServiceID:     sid,
		ContainerPort: 0,
		Name:          "NGINX_VERSION",
		AttrName:      "NGINX_VERSION",
		AttrValue:     "1.15.12-1~stretch",
		IsChange:      true,
		Scope:         "inner",
	}
	if err := db.GetManager().TenantServiceEnvVarDao().AddModel(innerEnv); err != nil {
		t.Fatalf("error create outer env: %v", err)
	}
	outerEnv := &model.TenantServiceEnvVar{
		TenantID:      "c1a29fe4d7b0413993dc859430cf743d",
		ServiceID:     sid,
		ContainerPort: 3306,
		Name:          "连接地址",
		AttrName:      "MYSQL_HOST",
		AttrValue:     "127.0.0.1",
		IsChange:      true,
		Scope:         "inner",
	}
	if err := db.GetManager().TenantServiceEnvVarDao().AddModel(outerEnv); err != nil {
		t.Fatalf("error create inner env: %v", err)
	}

	if err := db.GetManager().TenantServiceEnvVarDao().
		DelByServiceIDAndScope(sid, "outer"); err != nil {
		t.Fatalf("failed to delete outer env: %v", err)
	}
	env, err := db.GetManager().TenantServiceEnvVarDao().GetEnv(sid, innerEnv.AttrName)
	if err != nil {
		t.Fatalf("serviceid: %s; attr name: %s;failed to get env: %v", err, sid, innerEnv.AttrName)
	}
	if env.Scope != innerEnv.Scope {
		t.Errorf("Expected %s for scope, but returned %s", innerEnv.Scope, env.Scope)
	}
	if env.AttrValue != innerEnv.AttrValue {
		t.Errorf("Expected %s for attr_value, but returned %s", innerEnv.AttrValue, env.AttrValue)
	}
}
