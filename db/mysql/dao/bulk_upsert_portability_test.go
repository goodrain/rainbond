package dao

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type portableUpsertSpec struct {
	model     interface{}
	columns   []string
	callCount int
}

var portableUpsertSpecs = []portableUpsertSpec{
	{model: &model.EnterpriseLanguageVersion{}, columns: []string{"lang", "version", "build_strategy"}, callCount: 2},
	{model: &model.ThirdPartySvcDiscoveryCfg{}, columns: []string{"service_id"}, callCount: 1},
	{model: &model.TenantServiceProbe{}, columns: []string{"probe_id"}, callCount: 1},
	{model: &model.TenantServices{}, columns: []string{"service_id"}, callCount: 1},
	{model: &model.TenantServicesPort{}, columns: []string{"service_id", "container_port"}, callCount: 1},
	{model: &model.TenantServiceRelation{}, columns: []string{"service_id", "dep_service_id"}, callCount: 1},
	{model: &model.TenantServiceEnvVar{}, columns: []string{"service_id", "attr_name"}, callCount: 1},
	{model: &model.TenantServiceMountRelation{}, columns: []string{"service_id", "dep_service_id", "volume_name"}, callCount: 1},
	{model: &model.TenantServiceVolume{}, columns: []string{"service_id", "volume_name"}, callCount: 1},
	{model: &model.TenantServiceConfigFile{}, columns: []string{"service_id", "volume_name"}, callCount: 1},
	{model: &model.TenantServiceLable{}, columns: []string{"service_id", "label_key", "label_value"}, callCount: 1},
	{model: &model.TenantServiceAutoscalerRules{}, columns: []string{"rule_id"}, callCount: 1},
	{model: &model.TenantServiceAutoscalerRuleMetrics{}, columns: []string{"rule_id", "metric_type", "metric_name"}, callCount: 1},
	{model: &model.ComponentK8sAttributes{}, columns: []string{"component_id", "name"}, callCount: 1},
	{model: &model.ApplicationConfigGroup{}, columns: []string{"app_id", "config_group_name"}, callCount: 1},
	{model: &model.ConfigGroupService{}, columns: []string{"app_id", "config_group_name", "service_id"}, callCount: 1},
	{model: &model.ConfigGroupItem{}, columns: []string{"app_id", "config_group_name", "item_key"}, callCount: 1},
	{model: &model.K8sResource{}, columns: []string{"app_id", "name", "kind"}, callCount: 1},
	{model: &model.TenantPlugin{}, columns: []string{"plugin_id", "tenant_id"}, callCount: 1},
	{model: &model.TenantPluginBuildVersion{}, columns: []string{"plugin_id", "version_id", "deploy_version"}, callCount: 1},
	{model: &model.TenantPluginVersionEnv{}, columns: []string{"plugin_id", "service_id", "env_name"}, callCount: 1},
	{model: &model.TenantPluginVersionDiscoverConfig{}, columns: []string{"plugin_id", "service_id"}, callCount: 1},
	{model: &model.TenantServicePluginRelation{}, columns: []string{"plugin_id", "service_id"}, callCount: 1},
	{model: &model.TenantServicesStreamPluginPort{}, columns: []string{"service_id", "plugin_model", "container_port"}, callCount: 1},
	{model: &model.RuleExtension{}, columns: []string{"rule_id", "value"}, callCount: 1},
	{model: &model.HTTPRule{}, columns: []string{"uuid"}, callCount: 1},
	{model: &model.HTTPRuleRewrite{}, columns: []string{"uuid"}, callCount: 1},
	{model: &model.TCPRule{}, columns: []string{"uuid"}, callCount: 1},
	{model: &model.GwRuleConfig{}, columns: []string{"rule_id", "key"}, callCount: 1},
	{model: &model.ServiceEvent{}, columns: []string{"ID"}, callCount: 2},
	{model: &model.TenantServiceMonitor{}, columns: []string{"tenant_id", "name"}, callCount: 1},
}

func TestBatchUpsertsUseCentralPortabilityBoundary(t *testing.T) {
	dbRoot := filepath.Join("..")
	portableCalls := 0
	actualSignatures := make(map[string]int)
	err := filepath.WalkDir(dbRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(source), "gormbulkups.BulkUpsert") ||
			strings.Contains(string(source), "gorm-bulk-upsert") {
			t.Errorf("%s bypasses db/portable.BulkUpsert", path)
		}

		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, source, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "BulkUpsert" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok || identifier.Name != "portable" {
				return true
			}
			portableCalls++
			if len(call.Args) < 4 {
				position := fileSet.Position(call.Pos())
				t.Errorf("%s:%d does not declare a business conflict key", path, position.Line)
				return true
			}

			conflictColumns := make([]string, 0, len(call.Args)-3)
			for _, arg := range call.Args[3:] {
				literal, ok := arg.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					position := fileSet.Position(arg.Pos())
					t.Errorf("%s:%d declares a non-literal conflict column", path, position.Line)
					return true
				}
				column, err := strconv.Unquote(literal.Value)
				if err != nil {
					position := fileSet.Position(arg.Pos())
					t.Errorf("%s:%d has an invalid conflict column: %v", path, position.Line, err)
					return true
				}
				conflictColumns = append(conflictColumns, column)
			}
			actualSignatures[strings.Join(conflictColumns, ",")]++
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if portableCalls == 0 {
		t.Fatal("expected database batch writes to use db/portable.BulkUpsert")
	}

	expectedSignatures := make(map[string]int)
	for _, spec := range portableUpsertSpecs {
		expectedSignatures[strings.Join(spec.columns, ",")] += spec.callCount
	}
	if !reflect.DeepEqual(actualSignatures, expectedSignatures) {
		t.Fatalf("portable upsert conflict keys changed without updating their model contract:\nactual: %#v\nexpected: %#v", actualSignatures, expectedSignatures)
	}
}

func TestPortableBatchUpsertConflictColumnsExistOnModels(t *testing.T) {
	for _, spec := range portableUpsertSpecs {
		scope := &gorm.Scope{Value: spec.model}
		modelColumns := make(map[string]struct{})
		for _, field := range scope.Fields() {
			modelColumns[strings.ToLower(field.DBName)] = struct{}{}
		}

		for _, column := range spec.columns {
			if _, ok := modelColumns[strings.ToLower(column)]; !ok {
				t.Errorf("%T does not contain portable upsert conflict column %q", spec.model, column)
			}
		}
	}
}

// capability_id: region.database.portable-dao-upsert-identities
func TestPortableBatchUpsertsUseBusinessIdentity(t *testing.T) {
	db := openPortableDAOTestDB(t,
		&model.RuleExtension{},
		&model.TenantPlugin{},
		&model.TenantPluginBuildVersion{},
		&model.TenantServicePluginRelation{},
		&model.TenantServiceLable{},
		&model.TenantServiceMountRelation{},
		&model.TenantServiceAutoscalerRuleMetrics{},
	)
	defer db.Close()

	t.Run("rule extension identity uses rule and value", func(t *testing.T) {
		dao := &RuleExtensionDaoImpl{DB: db}
		if err := dao.CreateOrUpdateRuleExtensionsInBatch([]*model.RuleExtension{
			{UUID: "extension-a", RuleID: "rule", Key: "header", Value: "one"},
			{UUID: "extension-b", RuleID: "rule", Key: "header", Value: "two"},
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.Model(&model.RuleExtension{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected two extension values, got %d", count)
		}
	})

	t.Run("plugin identity includes tenant", func(t *testing.T) {
		dao := &PluginDaoImpl{DB: db}
		if err := dao.CreateOrUpdatePluginsInBatch([]*model.TenantPlugin{
			{PluginID: "plugin", TenantID: "tenant-a", PluginInfo: "a"},
			{PluginID: "plugin", TenantID: "tenant-b", PluginInfo: "b"},
		}); err != nil {
			t.Fatal(err)
		}
		if err := dao.CreateOrUpdatePluginsInBatch([]*model.TenantPlugin{
			{PluginID: "plugin", TenantID: "tenant-a", PluginInfo: "updated"},
		}); err != nil {
			t.Fatal(err)
		}

		var plugins []model.TenantPlugin
		if err := db.Order("tenant_id").Find(&plugins).Error; err != nil {
			t.Fatal(err)
		}
		if len(plugins) != 2 || plugins[0].PluginInfo != "updated" || plugins[1].PluginInfo != "b" {
			t.Fatalf("unexpected plugins after upsert: %#v", plugins)
		}
	})

	t.Run("plugin build identity includes deploy version", func(t *testing.T) {
		dao := &PluginBuildVersionDaoImpl{DB: db}
		if err := dao.CreateOrUpdatePluginBuildVersionsInBatch([]*model.TenantPluginBuildVersion{
			{PluginID: "plugin", VersionID: "v1", DeployVersion: "d1", Status: "one"},
			{PluginID: "plugin", VersionID: "v1", DeployVersion: "d2", Status: "two"},
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.Model(&model.TenantPluginBuildVersion{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected two plugin build versions, got %d", count)
		}
	})

	t.Run("plugin relation identity is service and plugin", func(t *testing.T) {
		dao := &TenantServicePluginRelationDaoImpl{DB: db}
		if err := dao.CreateOrUpdatePluginRelsInBatch([]*model.TenantServicePluginRelation{
			{PluginID: "plugin", ServiceID: "service", PluginModel: "old"},
		}); err != nil {
			t.Fatal(err)
		}
		if err := dao.CreateOrUpdatePluginRelsInBatch([]*model.TenantServicePluginRelation{
			{PluginID: "plugin", ServiceID: "service", PluginModel: "new"},
		}); err != nil {
			t.Fatal(err)
		}
		var relations []model.TenantServicePluginRelation
		if err := db.Find(&relations).Error; err != nil {
			t.Fatal(err)
		}
		if len(relations) != 1 || relations[0].PluginModel != "new" {
			t.Fatalf("unexpected plugin relations after upsert: %#v", relations)
		}
	})

	t.Run("label identity includes value", func(t *testing.T) {
		dao := &ServiceLabelDaoImpl{DB: db}
		if err := dao.CreateOrUpdateLabelsInBatch([]*model.TenantServiceLable{
			{ServiceID: "service", LabelKey: "node-selector", LabelValue: "zone-a"},
			{ServiceID: "service", LabelKey: "node-selector", LabelValue: "zone-b"},
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.Model(&model.TenantServiceLable{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected two label values, got %d", count)
		}
	})

	t.Run("mount identity uses source volume name", func(t *testing.T) {
		dao := &TenantServiceMountRelationDaoImpl{DB: db}
		if err := dao.CreateOrUpdateVolumeRelsInBatch([]*model.TenantServiceMountRelation{
			{ServiceID: "target", DependServiceID: "source", VolumeName: "data-a", VolumePath: "/data"},
			{ServiceID: "target", DependServiceID: "source", VolumeName: "data-b", VolumePath: "/data"},
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.Model(&model.TenantServiceMountRelation{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected two source volumes, got %d", count)
		}
	})

	t.Run("autoscaler metric identity uses type and name", func(t *testing.T) {
		dao := &TenantServceAutoscalerRuleMetricsDaoImpl{DB: db}
		if err := dao.CreateOrUpdateScaleRuleMetricsInBatch([]*model.TenantServiceAutoscalerRuleMetrics{
			{RuleID: "rule", MetricsType: "Resource", MetricsName: "cpu", MetricTargetValue: 60},
			{RuleID: "rule", MetricsType: "Resource", MetricsName: "memory", MetricTargetValue: 70},
		}); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.Model(&model.TenantServiceAutoscalerRuleMetrics{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("expected two autoscaler metrics, got %d", count)
		}
	})
}

func openPortableDAOTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "portable-dao.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(models...).Error; err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}
