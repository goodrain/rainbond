package cluster_test

import (
	"context"
	"errors"
	"testing"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/cluster"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestExpansionCluster(t *testing.T) {
	testCases := []struct {
		name        string
		clientSetup func() client.Client
		setup       func(client.Client) error
		input       model.ExpansionInput
		expectErr   bool
		errContains string
		verify      func(*testing.T, client.Client)
	}{
		{
			name:        "cluster_not_found",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup:       func(c client.Client) error { return nil }, // 不创建任何集群
			input:       newExpansionInput("nonexistent-service", "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "resource not found",
		},
		{
			name:        "multiple_clusters_found",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("cluster1", testutil.TestNamespace).
						WithServiceID("duplicate-service").
						Build(),
					testutil.NewMySQLCluster("cluster2", testutil.TestNamespace).
						WithServiceID("duplicate-service").
						Build(),
				})
			},
			input:       newExpansionInput("duplicate-service", "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "multiple resources found",
		},
		{
			name:        "cluster_stopped",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithPhase(kbappsv1.StoppedClusterPhase).
						Build(),
				})
			},
			input:       newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "is not running",
		},
		{
			name:        "cluster_stopping",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithPhase(kbappsv1.StoppingClusterPhase).
						Build(),
				})
			},
			input:       newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "is not running",
		},
		{
			name:        "no_component_specs",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewClusterBuilder("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						Build(),
				})
			},
			input:       newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "has no componentSpecs",
		},
		{
			name:        "invalid_resource_parsing",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						Build(),
				})
			},
			input:       newExpansionInput(testutil.TestServiceID, "invalid-cpu", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "parse desired resources",
		},

		// 组件上下文构建测试
		{
			name:        "single_component_expansion",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			verify: verifyVerticalScalingCreated,
		},
		{
			name:        "empty_component_name_uses_clusterdef",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewClusterBuilder("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithClusterDef("mysql").
						WithComponent("", "mysql-8.0").
						WithComponentResources("",
							testutil.Resources("250m", "500Mi"),
							testutil.Resources("250m", "500Mi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			verify: verifyVerticalScalingCreated,
		},

		// 扩容操作组合测试
		{
			name:        "no_expansion_needed",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("mysql", 2).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			verify: verifyNoOpsRequestCreated,
		},
		{
			name:        "horizontal_scale_up",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("mysql", 1).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3),
			verify: verifyHorizontalScalingCreated,
		},
		{
			name:        "horizontal_scale_down",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("mysql", 3).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 1),
			verify: verifyHorizontalScalingCreated,
		},
		{
			name:        "vertical_cpu_only",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources(
							"mysql",
							testutil.Resources("250m", "1Gi"),
							testutil.Resources("250m", "1Gi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 1),
			verify: verifyVerticalScalingCreated,
		},
		{
			name:        "vertical_memory_only",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources(
							"mysql",
							testutil.Resources("500m", "512Mi"),
							testutil.Resources("500m", "512Mi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 1),
			verify: verifyVerticalScalingCreated,
		},
		{
			name:        "horizontal_and_vertical",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("mysql", 1).
						WithComponentResources(
							"mysql",
							testutil.Resources("250m", "512Mi"),
							testutil.Resources("250m", "512Mi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3),
			verify: verifyBothHorizontalAndVerticalScalingCreated,
		},

		// 存储扩容专项测试
		{
			name:        "volume_expansion_no_pvc",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "20Gi", 1),
			verify: verifyNoVolumeExpansionCreated,
		},
		{
			name:        "volume_expansion_enabled",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					// 创建支持扩容的 StorageClass
					testutil.NewStorageClassBuilder("expandable-storage").
						WithAllowVolumeExpansion(true).
						Build(),

					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentVolumeClaimTemplate("mysql", "data", "expandable-storage", "10Gi").
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "20Gi", 1),
			verify: verifyVolumeExpansionCreated,
		},
		{
			name:        "volume_expansion_disabled",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					// 创建不支持扩容的 StorageClass
					testutil.NewStorageClassBuilder("non-expandable-storage").
						WithAllowVolumeExpansion(false).
						Build(),

					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentVolumeClaimTemplate("mysql", "data", "non-expandable-storage", "10Gi").
						Build(),
				})
			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "20Gi", 1),
			verify: verifyNoVolumeExpansionCreated, // 应该跳过存储扩容
		},
		{
			name:        "storage_class_not_found",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{

					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources("mysql",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentVolumeClaimTemplate("mysql", "data", "nonexistent-storage", "10Gi").
						Build(),
				})

			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "20Gi", 1),
			verify: verifyNoVolumeExpansionCreated, // 应该跳过存储扩容
		},

		// 错误处理测试
		{
			name: "get_cluster_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list failed")).
					Build()
			},
			input:       newExpansionInput("any-service", "500m", "1Gi", "10Gi", 2),
			expectErr:   true,
			errContains: "list clusters by service_id",
		},
		{
			name:        "ops_create_skipped_handling",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentResources(
							"mysql",
							testutil.Resources("250m", "512Mi"),
							testutil.Resources("250m", "512Mi")).
						Build(),
					testutil.NewOpsRequestBuilder("blocking-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithPhase(opsv1alpha1.OpsRunningPhase).
						WithInstanceLabel("test-cluster").
						Build(),
				})

			},
			input:  newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 1),
			verify: verifyCancelOpsStrategy,
		},

		// 多组件扩容测试
		{
			name:        "multi_component_horizontal_scaling_both",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).
						WithComponentReplicas("redis-sentinel", 3).
						WithComponentResources("redis",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 5),
			verify: func(t *testing.T, c client.Client) {
				verifyHorizontalScalingDetails(t, c, []model.ComponentHorizontalScaling{
					{Name: "redis", DeltaReplicas: 2},
					{Name: "redis-sentinel", DeltaReplicas: 2},
				})
			},
		},
		{
			name:        "multi_component_vertical_scaling_both",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).
						WithComponentReplicas("redis-sentinel", 3).
						WithComponentResources("redis",
							testutil.Resources("250m", "512Mi"),
							testutil.Resources("250m", "512Mi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("250m", "512Mi"),
							testutil.Resources("250m", "512Mi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3),
			verify: func(t *testing.T, c client.Client) {
				verifyVerticalScalingDetails(t, c, []model.ComponentVerticalScaling{
					{Name: "redis", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
					{Name: "redis-sentinel", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
				})
			},
		},
		{
			name:        "multi_component_scale_down",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 5).
						WithComponentReplicas("redis-sentinel", 5).
						WithComponentResources("redis",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 2),
			verify: func(t *testing.T, c client.Client) {
				verifyHorizontalScalingDetails(t, c, []model.ComponentHorizontalScaling{
					{Name: "redis", DeltaReplicas: -3},
					{Name: "redis-sentinel", DeltaReplicas: -3},
				})
			},
		},

		// 多组件部分扩容测试
		{
			name:        "multi_component_partial_horizontal_scaling",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).
						WithComponentReplicas("redis-sentinel", 5). // sentinel 已经是期望副本数
						WithComponentResources("redis",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 5),
			verify: func(t *testing.T, c client.Client) {
				// 只有 redis 组件需要扩容，sentinel 保持不变
				verifyHorizontalScalingDetails(t, c, []model.ComponentHorizontalScaling{
					{Name: "redis", DeltaReplicas: 2}, // redis 从 3 扩到 5
				})
			},
		},
		{
			name:        "multi_component_partial_vertical_scaling",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).
						WithComponentReplicas("redis-sentinel", 3).
						WithComponentResources("redis",
							testutil.Resources("250m", "512Mi"), // redis 需要垂直扩容
							testutil.Resources("250m", "512Mi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("500m", "1Gi"), // sentinel 已经是期望资源
							testutil.Resources("500m", "1Gi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3),
			verify: func(t *testing.T, c client.Client) {
				// 只有 redis 组件需要垂直扩容，sentinel 保持不变
				verifyVerticalScalingDetails(t, c, []model.ComponentVerticalScaling{
					{Name: "redis", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
				})
			},
		},

		// 混合扩容类型测试
		{
			name:        "multi_component_mixed_scaling_types",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).          // redis 需要水平扩容
						WithComponentReplicas("redis-sentinel", 5). // sentinel 不需要水平扩容
						WithComponentResources("redis",
							testutil.Resources("500m", "1Gi"), // redis 不需要垂直扩容
							testutil.Resources("500m", "1Gi")).
						WithComponentResources("redis-sentinel",
							testutil.Resources("250m", "512Mi"), // sentinel 需要垂直扩容
							testutil.Resources("250m", "512Mi")).
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 5),
			verify: func(t *testing.T, c client.Client) {
				// 验证水平扩容
				verifyHorizontalScalingDetails(t, c, []model.ComponentHorizontalScaling{
					{Name: "redis", DeltaReplicas: 2}, // redis 从 3 扩到 5
				})
				// 验证垂直扩容
				verifyVerticalScalingDetails(t, c, []model.ComponentVerticalScaling{
					{Name: "redis-sentinel", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
				})
			},
		},
		{
			name:        "multi_component_all_scaling_types",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					// 创建支持扩容的 StorageClass
					testutil.NewStorageClassBuilder("expandable-storage").
						WithAllowVolumeExpansion(true).
						Build(),

					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3). // redis 需要水平扩容
						WithComponentReplicas("redis-sentinel", 3).
						WithComponentResources("redis",
							testutil.Resources("250m", "512Mi"), // redis 需要垂直扩容
							testutil.Resources("250m", "512Mi")).
						WithComponentResources("redis-sentinel",
																		testutil.Resources("250m", "512Mi"), // sentinel 需要垂直扩容
																		testutil.Resources("250m", "512Mi")).
						WithComponentVolumeClaimTemplate("redis", "data", "expandable-storage", "5Gi").           // redis 需要存储扩容
						WithComponentVolumeClaimTemplate("redis-sentinel", "data", "expandable-storage", "10Gi"). // sentinel 不需要存储扩容
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 5),
			verify: func(t *testing.T, c client.Client) {
				// 验证水平扩容
				verifyHorizontalScalingDetails(t, c, []model.ComponentHorizontalScaling{
					{Name: "redis", DeltaReplicas: 2},          // redis 从 3 扩到 5
					{Name: "redis-sentinel", DeltaReplicas: 2}, // sentinel 从 3 扩到 5
				})
				// 验证垂直扩容
				verifyVerticalScalingDetails(t, c, []model.ComponentVerticalScaling{
					{Name: "redis", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
					{Name: "redis-sentinel", CPU: resource.MustParse("500m"), Memory: resource.MustParse("1Gi")},
				})
				// 验证存储扩容
				verifyVolumeExpansionDetails(t, c, []model.ComponentVolumeExpansion{
					{Name: "redis", VolumeClaimTemplateName: "data", Storage: resource.MustParse("10Gi")},
				})
			},
		},

		// 存储缩容警告测试
		{
			name:        "storage_shrinking_warning_test",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					// 创建支持扩容的 StorageClass
					testutil.NewStorageClassBuilder("expandable-storage").
						WithAllowVolumeExpansion(true).
						Build(),

					testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("mysql", 3).
						WithComponentResources("mysql",
																	testutil.Resources("500m", "1Gi"),
																	testutil.Resources("500m", "1Gi")).
						WithComponentVolumeClaimTemplate("mysql", "data", "expandable-storage", "20Gi"). // 当前存储 20Gi
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3), // 期望存储 10Gi < 当前 20Gi
			verify: func(t *testing.T, c client.Client) {
				// 验证没有创建 VolumeExpansion OpsRequest，因为存储缩容不支持
				verifyNoVolumeExpansionCreated(t, c)
				// 注意：这里只验证没有创建 OpsRequest，实际的日志验证需要 mock logger
			},
		},
		{
			name:        "multi_component_mixed_storage_shrinking",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				return testutil.CreateObjects(context.Background(), c, []client.Object{
					// 创建支持扩容的 StorageClass
					testutil.NewStorageClassBuilder("expandable-storage").
						WithAllowVolumeExpansion(true).
						Build(),

					testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
						WithServiceID(testutil.TestServiceID).
						WithComponentReplicas("redis", 3).
						WithComponentReplicas("redis-sentinel", 3).
						WithComponentResources("redis",
							testutil.Resources("500m", "1Gi"),
							testutil.Resources("500m", "1Gi")).
						WithComponentResources("redis-sentinel",
																		testutil.Resources("500m", "1Gi"),
																		testutil.Resources("500m", "1Gi")).
						WithComponentVolumeClaimTemplate("redis", "data", "expandable-storage", "5Gi").           // redis 需要扩容 5Gi -> 10Gi
						WithComponentVolumeClaimTemplate("redis-sentinel", "data", "expandable-storage", "20Gi"). // sentinel 尝试缩容 20Gi -> 10Gi
						Build(),
				})
			},
			input: newExpansionInput(testutil.TestServiceID, "500m", "1Gi", "10Gi", 3),
			verify: func(t *testing.T, c client.Client) {
				// 只有 redis 组件应该创建存储扩容，sentinel 组件应该被跳过
				verifyVolumeExpansionDetails(t, c, []model.ComponentVolumeExpansion{
					{Name: "redis", VolumeClaimTemplateName: "data", Storage: resource.MustParse("10Gi")},
				})
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient := tt.clientSetup()

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			service := cluster.NewService(k8sClient)

			err := service.ExpansionCluster(ctx, tt.input)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}

// newExpansionInput 构建 ExpansionInput 测试数据
func newExpansionInput(serviceID, cpu, memory, storage string, replicas int32) model.ExpansionInput {
	return model.ExpansionInput{
		RBDService: model.RBDService{
			ServiceID: serviceID,
		},
		ClusterResource: model.ClusterResource{
			CPU:      cpu,
			Memory:   memory,
			Storage:  storage,
			Replicas: replicas,
		},
	}
}

// verifyOpsRequestCreated 验证指定类型的 OpsRequest 是否被创建
func verifyOpsRequestCreated(t *testing.T, c client.Client, clusterName string, opsType opsv1alpha1.OpsType) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        clusterName,
		"operations.kubeblocks.io/ops-type": string(opsType),
	}))
	require.NoError(t, err)
	assert.NotEmpty(t, opsList.Items, "Expected %s OpsRequest to be created for cluster %s", opsType, clusterName)
}

// verifyNoOpsRequest 验证指定类型的 OpsRequest 是否未被创建
func verifyNoOpsRequest(t *testing.T, c client.Client, clusterName string, opsType opsv1alpha1.OpsType) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        clusterName,
		"operations.kubeblocks.io/ops-type": string(opsType),
	}))
	require.NoError(t, err)
	assert.Empty(t, opsList.Items, "Expected no %s OpsRequest for cluster %s", opsType, clusterName)
}

// verifyBothHorizontalAndVerticalScalingCreated 验证水平和垂直扩容 OpsRequest 都被创建
func verifyBothHorizontalAndVerticalScalingCreated(t *testing.T, c client.Client) {
	verifyHorizontalScalingCreated(t, c)
	verifyVerticalScalingCreated(t, c)
}

// verifyCancelOpsStrategy 验证取消操作策略是否正确执行
func verifyCancelOpsStrategy(t *testing.T, c client.Client) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        "test-cluster",
		"operations.kubeblocks.io/ops-type": string(opsv1alpha1.VerticalScalingType),
	}))
	require.NoError(t, err)

	// 应该有两个 VerticalScaling OpsRequest：原有的（被取消）和新创建的
	assert.Len(t, opsList.Items, 2, "Expected original blocking OpsRequest and new created OpsRequest")

	// 验证原有的 blocking-ops 是否被取消
	var blockingOps, newOps *opsv1alpha1.OpsRequest
	for i := range opsList.Items {
		if opsList.Items[i].Name == "blocking-ops" {
			blockingOps = &opsList.Items[i]
		} else {
			newOps = &opsList.Items[i]
		}
	}

	require.NotNil(t, blockingOps, "Original blocking OpsRequest should exist")
	require.NotNil(t, newOps, "New OpsRequest should be created")

	// 验证原有的 OpsRequest 被设置为取消状态
	assert.True(t, blockingOps.Spec.Cancel, "Original blocking OpsRequest should be cancelled")
}

// verifyHorizontalScalingCreated 验证水平扩容 OpsRequest 被创建
func verifyHorizontalScalingCreated(t *testing.T, c client.Client) {
	verifyOpsRequestCreated(t, c, "test-cluster", opsv1alpha1.HorizontalScalingType)
}

// verifyNoOpsRequestCreated 验证没有任何 OpsRequest 被创建
func verifyNoOpsRequestCreated(t *testing.T, c client.Client) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance": "test-cluster",
	}))
	require.NoError(t, err)
	assert.Empty(t, opsList.Items, "Expected no OpsRequest to be created")
}

// verifyNoVolumeExpansionCreated 验证存储扩容 OpsRequest 未被创建
func verifyNoVolumeExpansionCreated(t *testing.T, c client.Client) {
	verifyNoOpsRequest(t, c, "test-cluster", opsv1alpha1.VolumeExpansionType)
}

// verifyVerticalScalingCreated 验证垂直扩容 OpsRequest 被创建
func verifyVerticalScalingCreated(t *testing.T, c client.Client) {
	verifyOpsRequestCreated(t, c, "test-cluster", opsv1alpha1.VerticalScalingType)
}

// verifyVolumeExpansionCreated 验证存储扩容 OpsRequest 被创建
func verifyVolumeExpansionCreated(t *testing.T, c client.Client) {
	verifyOpsRequestCreated(t, c, "test-cluster", opsv1alpha1.VolumeExpansionType)
}

// verifyHorizontalScalingDetails 验证水平扩容的具体参数
func verifyHorizontalScalingDetails(t *testing.T, c client.Client, expectedComponents []model.ComponentHorizontalScaling) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        "test-cluster",
		"operations.kubeblocks.io/ops-type": string(opsv1alpha1.HorizontalScalingType),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, opsList.Items, "Expected HorizontalScaling OpsRequest to exist")

	opsRequest := opsList.Items[0]
	require.NotNil(t, opsRequest.Spec.HorizontalScalingList, "HorizontalScalingList should not be nil")

	// 验证组件数量
	assert.Len(t, opsRequest.Spec.HorizontalScalingList, len(expectedComponents), "Component count should match")

	// 验证每个组件的 DeltaReplicas
	actualComponents := make(map[string]int32)
	for _, comp := range opsRequest.Spec.HorizontalScalingList {
		var deltaReplicas int32
		if comp.ScaleOut != nil && comp.ScaleOut.ReplicaChanges != nil {
			deltaReplicas = *comp.ScaleOut.ReplicaChanges
		} else if comp.ScaleIn != nil && comp.ScaleIn.ReplicaChanges != nil {
			deltaReplicas = -*comp.ScaleIn.ReplicaChanges // ScaleIn 时为负数
		}
		actualComponents[comp.ComponentName] = deltaReplicas
	}

	for _, expected := range expectedComponents {
		actual, exists := actualComponents[expected.Name]
		assert.True(t, exists, "Component %s should exist in OpsRequest", expected.Name)
		assert.Equal(t, expected.DeltaReplicas, actual, "DeltaReplicas for component %s should match", expected.Name)
	}
}

// verifyVerticalScalingDetails 验证垂直扩容的具体资源值
func verifyVerticalScalingDetails(t *testing.T, c client.Client, expectedComponents []model.ComponentVerticalScaling) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        "test-cluster",
		"operations.kubeblocks.io/ops-type": string(opsv1alpha1.VerticalScalingType),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, opsList.Items, "Expected VerticalScaling OpsRequest to exist")

	opsRequest := opsList.Items[0]
	require.NotNil(t, opsRequest.Spec.VerticalScalingList, "VerticalScalingList should not be nil")

	// 验证组件数量
	assert.Len(t, opsRequest.Spec.VerticalScalingList, len(expectedComponents), "Component count should match")

	// 验证每个组件的 CPU/Memory
	actualComponents := make(map[string]opsv1alpha1.VerticalScaling)
	for _, comp := range opsRequest.Spec.VerticalScalingList {
		actualComponents[comp.ComponentName] = comp
	}

	for _, expected := range expectedComponents {
		actual, exists := actualComponents[expected.Name]
		assert.True(t, exists, "Component %s should exist in OpsRequest", expected.Name)

		// 验证 CPU
		actualCPU := actual.ResourceRequirements.Limits.Cpu()
		assert.Equal(t, expected.CPU.Cmp(*actualCPU), 0, "CPU for component %s should match", expected.Name)

		// 验证 Memory
		actualMem := actual.ResourceRequirements.Limits.Memory()
		assert.Equal(t, expected.Memory.Cmp(*actualMem), 0, "Memory for component %s should match", expected.Name)
	}
}

// verifyVolumeExpansionDetails 验证存储扩容的具体参数
func verifyVolumeExpansionDetails(t *testing.T, c client.Client, expectedComponents []model.ComponentVolumeExpansion) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        "test-cluster",
		"operations.kubeblocks.io/ops-type": string(opsv1alpha1.VolumeExpansionType),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, opsList.Items, "Expected VolumeExpansion OpsRequest to exist")

	opsRequest := opsList.Items[0]
	require.NotNil(t, opsRequest.Spec.VolumeExpansionList, "VolumeExpansionList should not be nil")

	// 验证组件数量
	assert.Len(t, opsRequest.Spec.VolumeExpansionList, len(expectedComponents), "Component count should match")

	// 验证每个组件的存储大小
	actualComponents := make(map[string]opsv1alpha1.VolumeExpansion)
	for _, comp := range opsRequest.Spec.VolumeExpansionList {
		actualComponents[comp.ComponentName] = comp
	}

	for _, expected := range expectedComponents {
		actual, exists := actualComponents[expected.Name]
		assert.True(t, exists, "Component %s should exist in OpsRequest", expected.Name)

		// 验证存储大小
		for _, vct := range actual.VolumeClaimTemplates {
			if vct.Name == expected.VolumeClaimTemplateName {
				actualStorage := vct.Storage
				assert.Equal(t, expected.Storage.Cmp(actualStorage), 0,
					"Storage size for component %s volume %s should match", expected.Name, expected.VolumeClaimTemplateName)
			}
		}
	}
}

// verifyMultiComponentOpsRequest 验证多组件 OpsRequest 的组件数量
func verifyMultiComponentOpsRequest(t *testing.T, c client.Client, opsType opsv1alpha1.OpsType, expectedComponentCount int) {
	ctx := context.Background()
	var opsList opsv1alpha1.OpsRequestList

	err := c.List(ctx, &opsList, client.MatchingLabels(map[string]string{
		"app.kubernetes.io/instance":        "test-cluster",
		"operations.kubeblocks.io/ops-type": string(opsType),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, opsList.Items, "Expected %s OpsRequest to exist", opsType)

	opsRequest := opsList.Items[0]
	var actualComponentCount int

	switch opsType {
	case opsv1alpha1.HorizontalScalingType:
		actualComponentCount = len(opsRequest.Spec.HorizontalScalingList)
	case opsv1alpha1.VerticalScalingType:
		actualComponentCount = len(opsRequest.Spec.VerticalScalingList)
	case opsv1alpha1.VolumeExpansionType:
		actualComponentCount = len(opsRequest.Spec.VolumeExpansionList)
	default:
		t.Fatalf("Unsupported OpsType: %s", opsType)
	}

	assert.Equal(t, expectedComponentCount, actualComponentCount,
		"Component count in %s OpsRequest should match", opsType)
}
