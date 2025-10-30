package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestFindFailedCondition(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		expected   *metav1.Condition
	}{
		{
			name: "has failed condition - any type with false status",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Available", Status: metav1.ConditionFalse, Reason: "TestFailure", Message: "Service unavailable"},
			},
			expected: &metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "TestFailure", Message: "Service unavailable"},
		},
		{
			name: "multiple failed conditions - returns first one",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionFalse, Reason: "FirstFailure", Message: "First failed condition"},
				{Type: "Available", Status: metav1.ConditionFalse, Reason: "SecondFailure", Message: "Second failed condition"},
			},
			expected: &metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "FirstFailure", Message: "First failed condition"},
		},
		{
			name: "no failed condition",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Available", Status: metav1.ConditionTrue},
			},
			expected: nil,
		},
		{
			name:       "empty conditions",
			conditions: []metav1.Condition{},
			expected:   nil,
		},
		{
			name:       "nil conditions",
			conditions: nil,
			expected:   nil,
		},
		{
			name: "only true conditions",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Available", Status: metav1.ConditionTrue},
				{Type: "Progressing", Status: metav1.ConditionTrue},
			},
			expected: nil,
		},
		{
			name: "mixed with unknown status - only false matters",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Unknown", Status: metav1.ConditionUnknown},
				{Type: "Failed", Status: metav1.ConditionFalse, Reason: "ActualFailure"},
			},
			expected: &metav1.Condition{Type: "Failed", Status: metav1.ConditionFalse, Reason: "ActualFailure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findFailedCondition(tt.conditions)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Type, result.Type)
				assert.Equal(t, tt.expected.Status, result.Status)
				assert.Equal(t, tt.expected.Reason, result.Reason)
			}
		})
	}
}

// TestGetClusterEvents 测试获取集群事件列表功能
func TestGetClusterEvents(t *testing.T) {
	tests := []struct {
		name          string
		serviceID     string
		pagination    model.Pagination
		setupCluster  func(client.Client) error
		setupOpsReqs  func(client.Client) error
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
		expectCount   int
		verifyResult  func(*testing.T, *model.PaginatedResult[model.EventItem])
	}{
		{
			name:      "multiple_ops_mixed_states_with_sorting",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				now := time.Now()

				// 创建不同时间和状态的 OpsRequest，用于测试排序和过滤
				ops := []client.Object{
					// 支持的操作类型 - 垂直伸缩（成功）- 最早
					testutil.NewOpsRequestBuilder("vertical-success", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),

					// 支持的操作类型 - 水平伸缩（运行中）- 最新
					testutil.NewOpsRequestBuilder("horizontal-running", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.HorizontalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),

					// 支持的操作类型 - 存储扩容（失败）- 中间
					testutil.NewOpsRequestBuilder("volume-failed", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),

					// 不支持的操作类型 - 应该被过滤掉
					testutil.NewOpsRequestBuilder("restart-unsupported", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.RestartType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}

				// 设置创建时间以确保排序测试
				for i, obj := range ops {
					opsReq := obj.(*opsv1alpha1.OpsRequest)
					opsReq.CreationTimestamp = metav1.NewTime(now.Add(time.Duration(i) * time.Minute))
				}

				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 3, // 应该只返回3个支持的操作类型
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				require.Len(t, result.Items, 3)

				// 验证排序：按创建时间降序（最新的在前）
				// 根据时间设置：vertical-success(i=0, +0min), horizontal-running(i=1, +1min), volume-failed(i=2, +2min)
				// 排序后：volume-failed(最新), horizontal-running(中间), vertical-success(最早)
				assert.Equal(t, "volume-failed", result.Items[0].OpsName)
				assert.Equal(t, "horizontal-running", result.Items[1].OpsName)
				assert.Equal(t, "vertical-success", result.Items[2].OpsName)

				// 验证操作类型映射
				assert.Equal(t, "update-service-volume", result.Items[0].OpsType)
				assert.Equal(t, "horizontal-service", result.Items[1].OpsType)
				assert.Equal(t, "vertical-service", result.Items[2].OpsType)

				// 验证状态映射
				assert.Equal(t, "failure", result.Items[0].Status)
				assert.Equal(t, "running", result.Items[1].Status)
				assert.Equal(t, "success", result.Items[2].Status)

			},
		},
		{
			name:      "pagination_first_page",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 2,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("ops-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("ops-2", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.HorizontalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("ops-3", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 3, // 总共3个事件
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				// 第一页应该返回2个项目
				assert.Len(t, result.Items, 2)
				assert.Equal(t, 3, result.Total) // 总数应该是3
			},
		},
		{
			name:      "pagination_second_page",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     2,
				PageSize: 2,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("ops-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("ops-2", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.HorizontalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("ops-3", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 3, // 总共3个事件
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				// 第二页应该返回1个项目
				assert.Len(t, result.Items, 1)
				assert.Equal(t, 3, result.Total) // 总数应该是3
			},
		},
		{
			name:      "service_id_not_found",
			serviceID: "non-existent-service-id",
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				// 创建一个不同serviceID的集群
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID("different-service-id").
					Build()
				return c.Create(context.Background(), cluster)
			},
			expectError:   true,
			errorContains: "get cluster by service_id",
		},
		{
			name:      "empty_ops_history",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			// 不设置setupOpsReqs，表示没有任何OpsRequest
			expectCount: 0,
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				assert.Empty(t, result.Items)
				assert.Equal(t, 0, result.Total)
			},
		},
		{
			name:      "business_contract_operation_filtering",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				// 测试业务契约：只保留 Block Mechanica 支持的操作类型
				ops := []client.Object{
					// 支持的操作类型
					testutil.NewOpsRequestBuilder("supported-backup", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.BackupType). // 支持 -> "backup-database"
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("supported-vertical", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType). // 支持 -> "vertical-service"
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					// 不支持的操作类型 - 应该被过滤掉
					testutil.NewOpsRequestBuilder("unsupported-restart", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.RestartType). // 不支持 -> ""
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("unsupported-start", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.StartType). // 不支持 -> ""
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 2, // 只保留2个支持的操作类型
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				require.Len(t, result.Items, 2)
				assert.Equal(t, 2, result.Total)

				// 业务契约验证：确保只包含支持的操作类型
				opsNames := make([]string, len(result.Items))
				opsTypes := make([]string, len(result.Items))
				for i, item := range result.Items {
					opsNames[i] = item.OpsName
					opsTypes[i] = item.OpsType
				}

				// 应该包含支持的操作
				assert.Contains(t, opsNames, "supported-backup")
				assert.Contains(t, opsNames, "supported-vertical")

				// 不应该包含不支持的操作
				assert.NotContains(t, opsNames, "unsupported-restart")
				assert.NotContains(t, opsNames, "unsupported-start")

				// 验证操作类型映射正确
				assert.Contains(t, opsTypes, "backup-database")
				assert.Contains(t, opsTypes, "vertical-service")
				assert.NotContains(t, opsTypes, "") // 不应该有空的操作类型
			},
		},
		{
			name:      "all_ops_filtered_out",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				// 创建只有不支持类型的OpsRequest
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("restart-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.RestartType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("start-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.StartType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 0, // 所有操作都被过滤掉
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				assert.Empty(t, result.Items)
				assert.Equal(t, 0, result.Total)
			},
		},
		{
			name:      "pagination_out_of_range",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     10, // 超出范围的页码
				PageSize: 2,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("ops-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 1, // 总数应该是1
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				// 超出范围的页码应该返回空结果
				assert.Empty(t, result.Items)
				assert.Equal(t, 1, result.Total) // 但总数应该正确
			},
		},
		{
			name:      "invalid_pagination_params",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     0, // 无效的页码
				PageSize: 0, // 无效的页大小
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("ops-1", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 1,
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				// pagination.Validate()会处理无效的分页参数
				// 通常会设置默认值而不是返回空结果
				assert.NotNil(t, result.Items)
				assert.Equal(t, 1, result.Total)
			},
		},
		{
			name:      "get_cluster_by_service_id_fails",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("mock k8s list error")).
					Build()
			},
			expectError:   true,
			errorContains: "get cluster by service_id",
		},
		{
			name:      "k8s_list_operation_fails",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			clientSetup: func() client.Client {
				// FailingClientBuilder的ListError会影响所有List操作
				// 包括GetClusterByServiceID中的集群查询
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("mock k8s list error")).
					Build()
			},
			expectError:   true,
			errorContains: "get cluster by service_id",
		},
		{
			name:      "business_contract_time_sorting",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

				// 测试业务契约：事件必须按创建时间降序排序，无论什么操作类型
				ops := []client.Object{
					// 最早的操作
					testutil.NewOpsRequestBuilder("early-backup", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.BackupType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					// 中间的操作
					testutil.NewOpsRequestBuilder("middle-scaling", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
					// 最新的操作
					testutil.NewOpsRequestBuilder("latest-volume", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),
				}

				// 明确设置创建时间以确保业务契约测试
				for i, obj := range ops {
					opsReq := obj.(*opsv1alpha1.OpsRequest)
					// 时间间隔确保排序稳定性
					opsReq.CreationTimestamp = metav1.NewTime(baseTime.Add(time.Duration(i) * time.Hour))
				}

				return testutil.CreateObjects(ctx, c, ops)
			},
			expectCount: 3,
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				require.Len(t, result.Items, 3)

				// 业务契约验证：必须按创建时间降序排序
				// latest-volume (最新) -> middle-scaling (中间) -> early-backup (最早)
				assert.Equal(t, "latest-volume", result.Items[0].OpsName, "Latest operation should be first")
				assert.Equal(t, "middle-scaling", result.Items[1].OpsName, "Middle operation should be second")
				assert.Equal(t, "early-backup", result.Items[2].OpsName, "Earliest operation should be last")

				// 验证创建时间确实是降序的
				for i := 0; i < len(result.Items)-1; i++ {
					currentTime, _ := time.Parse(time.RFC3339, result.Items[i].CreateTime)
					nextTime, _ := time.Parse(time.RFC3339, result.Items[i+1].CreateTime)
					assert.True(t, currentTime.After(nextTime), "Events should be sorted by creation time in descending order")
				}
			},
		},
		{
			name:      "data_correctness_verification",
			serviceID: testutil.TestServiceID,
			pagination: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
			setupCluster: func(c client.Client) error {
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return c.Create(context.Background(), cluster)
			},
			setupOpsReqs: func(c client.Client) error {
				ctx := context.Background()
				now := time.Now()

				// 创建一个失败的OpsRequest，包含Condition信息用于测试错误信息提取
				failedOps := testutil.NewOpsRequestBuilder("failed-with-condition", testutil.TestNamespace).
					WithClusterName("test-cluster").
					WithType(opsv1alpha1.BackupType).
					WithInstanceLabel("test-cluster").
					WithPhase(opsv1alpha1.OpsFailedPhase).
					Build()

				// 设置失败条件
				failedOps.Status.Conditions = []metav1.Condition{
					{
						Type:    "Ready",
						Status:  metav1.ConditionTrue,
						Reason:  "Progressing",
						Message: "Operation is progressing",
					},
					{
						Type:    "Failed",
						Status:  metav1.ConditionFalse,
						Reason:  "BackupFailed",
						Message: "Backup operation failed due to insufficient storage",
					},
				}

				// 设置创建时间和完成时间
				failedOps.CreationTimestamp = metav1.NewTime(now.Add(-10 * time.Minute))
				failedOps.Status.CompletionTimestamp = metav1.NewTime(now.Add(-5 * time.Minute))

				// 创建一个成功的OpsRequest用于验证其他操作类型
				successOps := testutil.NewOpsRequestBuilder("restore-success", testutil.TestNamespace).
					WithClusterName("test-cluster").
					WithType(opsv1alpha1.RestoreType).
					WithInstanceLabel("test-cluster").
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					Build()

				successOps.CreationTimestamp = metav1.NewTime(now.Add(-15 * time.Minute))
				successOps.Status.CompletionTimestamp = metav1.NewTime(now.Add(-12 * time.Minute))

				// 创建一个重配置操作
				reconfOps := testutil.NewOpsRequestBuilder("reconfig-pending", testutil.TestNamespace).
					WithClusterName("test-cluster").
					WithType(opsv1alpha1.ReconfiguringType).
					WithInstanceLabel("test-cluster").
					WithPhase(opsv1alpha1.OpsPendingPhase).
					Build()

				reconfOps.CreationTimestamp = metav1.NewTime(now.Add(-8 * time.Minute))

				return testutil.CreateObjects(ctx, c, []client.Object{failedOps, successOps, reconfOps})
			},
			expectCount: 3, // 3个支持的操作类型
			verifyResult: func(t *testing.T, result *model.PaginatedResult[model.EventItem]) {
				require.Len(t, result.Items, 3)

				// 找到特定的事件进行验证
				var failedEvent, restoreEvent, reconfigEvent *model.EventItem
				for i := range result.Items {
					switch result.Items[i].OpsName {
					case "failed-with-condition":
						failedEvent = &result.Items[i]
					case "restore-success":
						restoreEvent = &result.Items[i]
					case "reconfig-pending":
						reconfigEvent = &result.Items[i]
					}
				}

				// 验证失败事件的错误信息提取
				require.NotNil(t, failedEvent)
				assert.Equal(t, "backup-database", failedEvent.OpsType)
				assert.Equal(t, "failure", failedEvent.Status)
				assert.Equal(t, "complete", failedEvent.FinalStatus)
				assert.Equal(t, "Backup operation failed due to insufficient storage", failedEvent.Message)
				assert.Equal(t, "BackupFailed", failedEvent.Reason)
				assert.NotEmpty(t, failedEvent.CreateTime)
				assert.NotEmpty(t, failedEvent.EndTime)

				// 验证时间格式是否符合RFC3339
				_, err := time.Parse(time.RFC3339, failedEvent.CreateTime)
				assert.NoError(t, err, "CreateTime should be in RFC3339 format")
				_, err = time.Parse(time.RFC3339, failedEvent.EndTime)
				assert.NoError(t, err, "EndTime should be in RFC3339 format")

				// 验证恢复操作
				require.NotNil(t, restoreEvent)
				assert.Equal(t, "restore-database", restoreEvent.OpsType)
				assert.Equal(t, "success", restoreEvent.Status)
				assert.Equal(t, "complete", restoreEvent.FinalStatus)
				assert.Equal(t, "Operation completed successfully", restoreEvent.Message)
				assert.Empty(t, restoreEvent.Reason) // 成功的操作不应该有reason
				assert.NotEmpty(t, restoreEvent.EndTime)

				// 验证重配置操作
				require.NotNil(t, reconfigEvent)
				assert.Equal(t, "reconfiguring-cluster", reconfigEvent.OpsType)
				assert.Equal(t, "pending", reconfigEvent.Status)
				assert.Equal(t, "running", reconfigEvent.FinalStatus)
				assert.Equal(t, "Operation is pending", reconfigEvent.Message)
				assert.Empty(t, reconfigEvent.EndTime) // 进行中的操作没有结束时间

				// 验证所有事件的公共字段
				for _, item := range result.Items {
					assert.Equal(t, "system", item.UserName)
					assert.NotEmpty(t, item.CreateTime)

					// 验证CreateTime格式
					_, err := time.Parse(time.RFC3339, item.CreateTime)
					assert.NoError(t, err, "CreateTime should be in RFC3339 format for %s", item.OpsName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var k8sClient client.Client
			if tt.clientSetup != nil {
				k8sClient = tt.clientSetup()
			} else {
				k8sClient = testutil.NewFakeClientWithIndexes()
			}

			ctx := context.Background()

			// 设置集群数据
			if tt.setupCluster != nil {
				require.NoError(t, tt.setupCluster(k8sClient))
			}

			// 设置OpsRequest数据
			if tt.setupOpsReqs != nil {
				require.NoError(t, tt.setupOpsReqs(k8sClient))
			}

			// 创建服务并执行测试
			service := &Service{client: k8sClient}
			result, err := service.GetClusterEvents(ctx, tt.serviceID, tt.pagination)

			// 验证错误情况
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			// 验证正常情况
			require.NoError(t, err)
			require.NotNil(t, result)

			// 验证基本计数
			if tt.expectCount >= 0 {
				assert.Equal(t, tt.expectCount, result.Total)
			}

			// 执行自定义验证
			if tt.verifyResult != nil {
				tt.verifyResult(t, result)
			}
		})
	}
}

// TestConvertOpsRequestToEventItem 测试 OpsRequest 转换为 EventItem
func TestConvertOpsRequestToEventItem(t *testing.T) {
	service := &Service{}
	baseTime := time.Now()

	tests := []struct {
		name                string
		phase               opsv1alpha1.OpsPhase
		expectedStatus      string
		expectedFinalStatus string
		expectedMessage     string
		completionTime      *metav1.Time
		conditions          []metav1.Condition
	}{
		{
			name:                "OpsSucceedPhase",
			phase:               opsv1alpha1.OpsSucceedPhase,
			expectedStatus:      "success",
			expectedFinalStatus: "complete",
			expectedMessage:     "Operation completed successfully",
			completionTime:      &metav1.Time{Time: baseTime},
		},
		{
			name:                "OpsFailedPhase with condition",
			phase:               opsv1alpha1.OpsFailedPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "Test failure message",
			completionTime:      &metav1.Time{Time: baseTime},
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionFalse, Message: "Test failure message", Reason: "TestFailure"},
			},
		},
		{
			name:                "OpsFailedPhase without condition",
			phase:               opsv1alpha1.OpsFailedPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "Operation failed with unknown reason",
			completionTime:      &metav1.Time{Time: baseTime},
		},
		{
			name:                "OpsCancelledPhase",
			phase:               opsv1alpha1.OpsCancelledPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "Operation was cancelled",
			completionTime:      &metav1.Time{Time: baseTime},
		},
		{
			name:                "OpsAbortedPhase",
			phase:               opsv1alpha1.OpsAbortedPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "Operation was aborted",
			completionTime:      &metav1.Time{Time: baseTime},
		},
		{
			name:                "OpsPendingPhase",
			phase:               opsv1alpha1.OpsPendingPhase,
			expectedStatus:      "pending",
			expectedFinalStatus: "running",
			expectedMessage:     "Operation is pending",
		},
		{
			name:                "OpsCreatingPhase",
			phase:               opsv1alpha1.OpsCreatingPhase,
			expectedStatus:      "running",
			expectedFinalStatus: "running",
			expectedMessage:     "Operation is being created",
		},
		{
			name:                "OpsRunningPhase",
			phase:               opsv1alpha1.OpsRunningPhase,
			expectedStatus:      "running",
			expectedFinalStatus: "running",
			expectedMessage:     "Operation is running",
		},
		{
			name:                "OpsCancellingPhase",
			phase:               opsv1alpha1.OpsCancellingPhase,
			expectedStatus:      "cancelling",
			expectedFinalStatus: "running",
			expectedMessage:     "Operation is being cancelled",
		},
		{
			name:                "UnknownPhase",
			phase:               opsv1alpha1.OpsPhase("UnknownPhase"),
			expectedStatus:      "unknown",
			expectedFinalStatus: "running",
			expectedMessage:     "Operation status unknown",
		},
		{
			name:                "OpsFailedPhase with multiple conditions",
			phase:               opsv1alpha1.OpsFailedPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "First failure message",
			completionTime:      &metav1.Time{Time: baseTime},
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Message: "All good"},
				{Type: "Available", Status: metav1.ConditionFalse, Message: "First failure message", Reason: "FirstFailure"},
				{Type: "Progressing", Status: metav1.ConditionFalse, Message: "Second failure message", Reason: "SecondFailure"},
			},
		},
		{
			name:                "OpsFailedPhase with empty message in condition",
			phase:               opsv1alpha1.OpsFailedPhase,
			expectedStatus:      "failure",
			expectedFinalStatus: "complete",
			expectedMessage:     "",
			completionTime:      &metav1.Time{Time: baseTime},
			conditions: []metav1.Condition{
				{Type: "Failed", Status: metav1.ConditionFalse, Message: "", Reason: "EmptyMessage"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opsRequest := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-ops",
					CreationTimestamp: metav1.Time{Time: baseTime},
				},
				Spec: opsv1alpha1.OpsRequestSpec{
					Type: opsv1alpha1.VerticalScalingType,
				},
				Status: opsv1alpha1.OpsRequestStatus{
					Phase:      tt.phase,
					Conditions: tt.conditions,
				},
			}

			if tt.completionTime != nil {
				opsRequest.Status.CompletionTimestamp = *tt.completionTime
			}

			result := service.convertOpsRequestToEventItem(opsRequest)

			assert.Equal(t, "test-ops", result.OpsName)
			assert.Equal(t, "vertical-service", result.OpsType) // VerticalScalingType -> vertical-service
			assert.Equal(t, "system", result.UserName)
			assert.Equal(t, tt.expectedStatus, result.Status)
			assert.Equal(t, tt.expectedFinalStatus, result.FinalStatus)
			assert.Equal(t, tt.expectedMessage, result.Message)

			// 验证时间格式
			assert.NotEmpty(t, result.CreateTime)
			if tt.completionTime != nil {
				assert.NotEmpty(t, result.EndTime)
			} else {
				assert.Empty(t, result.EndTime)
			}

			// 验证失败条件的 Reason
			if tt.phase == opsv1alpha1.OpsFailedPhase && len(tt.conditions) > 0 {
				for _, cond := range tt.conditions {
					if cond.Status == metav1.ConditionFalse {
						assert.Equal(t, cond.Reason, result.Reason)
						break
					}
				}
			}
		})
	}
}
