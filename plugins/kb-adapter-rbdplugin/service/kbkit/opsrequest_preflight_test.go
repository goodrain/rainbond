package kbkit

import (
	"context"
	"errors"
	"testing"

	"github.com/furutachiKurea/block-mechanica/internal/testutil"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUniqueOpsDecide(t *testing.T) {
	tests := []struct {
		name           string
		setupBgOps     func(client.Client) error
		expectDecision preflightDecision
		expectError    bool
		errorMsg       string
	}{
		{
			name:           "no_existing_ops",
			setupBgOps:     func(client.Client) error { return nil },
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "all_ops_non_blocking",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("succeeded", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("aborted", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsAbortedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("cancelled", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCancelledPhase).
						WithCancel().
						Build(),
					testutil.NewOpsRequestBuilder("cancelling", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCancellingPhase).
						WithCancel().
						Build(),
					testutil.NewOpsRequestBuilder("failed", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "all_non_blocking_ops_cancelled",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-2", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCancelledPhase).
						WithCancel().
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "all_non_blocking_ops_failed",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-3", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "all_non_blocking_ops_aborted",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-4", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsAbortedPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "all_non_blocking_ops_cancelling",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-5", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCancellingPhase).
						WithCancel().
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "multiple_non_blocking_ops",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("succeeded-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("cancelled-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCancelledPhase).
						WithCancel().
						Build(),
					testutil.NewOpsRequestBuilder("failed-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed,
			expectError:    false,
		},
		{
			name: "has_running_ops",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-running", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
				})
			},
			expectDecision: preflightSkip,
			expectError:    false,
		},
		{
			name: "has_pending_ops",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-pending", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsPendingPhase).
						Build(),
				})
			},
			expectDecision: preflightSkip,
			expectError:    false,
		},
		{
			name: "has_creating_ops",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("test-ops-creating", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsCreatingPhase).
						Build(),
				})
			},
			expectDecision: preflightSkip,
			expectError:    false,
		},
		{
			name: "mixed_blocking_and_non_blocking",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					// 一个成功的（非阻塞）
					testutil.NewOpsRequestBuilder("succeeded-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					// 一个运行中的（阻塞）
					testutil.NewOpsRequestBuilder("running-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
				})
			},
			expectDecision: preflightSkip, // 有任何阻塞的就要跳过
			expectError:    false,
		},
		{
			name: "multiple_blocking_ops",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("running-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
					testutil.NewOpsRequestBuilder("pending-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsPendingPhase).
						Build(),
				})
			},
			expectDecision: preflightSkip,
			expectError:    false,
		},
		{
			name: "different_ops_type_should_not_interfere",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("different-type-ops", testutil.TestNamespace).
						WithClusterName("test-cluster").
						WithType(opsv1alpha1.HorizontalScalingType).
						WithInstanceLabel("test-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed, // 不应该被阻塞
			expectError:    false,
		},
		{
			name: "different_cluster_should_not_interfere",
			setupBgOps: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewOpsRequestBuilder("different-cluster-ops", testutil.TestNamespace).
						WithClusterName("different-cluster").
						WithType(opsv1alpha1.VolumeExpansionType).
						WithInstanceLabel("different-cluster").
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
				})
			},
			expectDecision: preflightProceed, // 不应该被阻塞
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testutil.NewFakeClient()
			ctx := context.Background()

			cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
				WithServiceID(testutil.TestServiceID).
				Build()
			require.NoError(t, client.Create(ctx, cluster))

			err := tt.setupBgOps(client)
			require.NoError(t, err, "setup OpsRequests failed")

			targetOps := testutil.NewOpsRequestBuilder("target-ops", testutil.TestNamespace).
				WithClusterName("test-cluster").
				WithType(opsv1alpha1.VolumeExpansionType).
				WithInstanceLabel("test-cluster").
				WithPhase(opsv1alpha1.OpsPendingPhase).
				Build()

			uniqueOpsChecker := uniqueOps{}
			result, err := uniqueOpsChecker.decide(ctx, client, targetOps)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectDecision, result.Decision, "Decision 不符合预期")
			}
		})
	}
}

func TestPriorityOpsDecide(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"

	testCases := []struct {
		name           string
		clientSetup    func() client.Client
		setup          func(client.Client) error
		targetType     opsv1alpha1.OpsType
		expectDecision preflightDecision
		expectErr      bool
		errContains    string
		verify         func(t *testing.T, c client.Client)
	}{
		{
			name:        "no_non_final_ops",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				succeeded := testutil.NewOpsRequestBuilder("succeeded", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					WithInstanceLabel(clusterName).
					Build()

				cancelled := testutil.NewOpsRequestBuilder("cancelled", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsCancelledPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{succeeded, cancelled})
			},
			targetType:     opsv1alpha1.RestartType,
			expectDecision: preflightProceed,
		},
		{
			name:        "blocking_ops_cleanup_succeeds",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				blockingToCancel := testutil.NewOpsRequestBuilder("scaling-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				blockingToExpire := testutil.NewOpsRequestBuilder("restart-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					WithInstanceLabel(clusterName).
					Build()
				blockingToExpire.Spec.TimeoutSeconds = ptr.To(int32(120))

				return testutil.CreateObjects(ctx, c, []client.Object{blockingToCancel, blockingToExpire})
			},
			targetType:     opsv1alpha1.StopType,
			expectDecision: preflightCleanupAndProceed,
			verify: func(t *testing.T, c client.Client) {
				updatedCancel := &opsv1alpha1.OpsRequest{}
				require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "scaling-block"}, updatedCancel))
				assert.True(t, updatedCancel.Spec.Cancel, "横向伸缩阻塞操作应被标记为取消")

				updatedExpire := &opsv1alpha1.OpsRequest{}
				require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "restart-block"}, updatedExpire))
				if assert.NotNil(t, updatedExpire.Spec.TimeoutSeconds) {
					assert.Equal(t, int32(1), *updatedExpire.Spec.TimeoutSeconds, "非伸缩阻塞操作的 timeoutSeconds 应被缩短")
				}
			},
		},
		{
			name: "list_existing_ops_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list failed")).
					Build()
			},
			targetType:  opsv1alpha1.RestartType,
			expectErr:   true,
			errContains: "get existing opsrequests",
		},
		{
			name: "cleanup_blocking_ops_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithPatchError(errors.New("patch failed")).
					Build()
			},
			setup: func(c client.Client) error {
				blockingToCancel := testutil.NewOpsRequestBuilder("scaling-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				blockingToExpire := testutil.NewOpsRequestBuilder("restart-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()
				blockingToExpire.Spec.TimeoutSeconds = ptr.To(int32(300))

				return testutil.CreateObjects(ctx, c, []client.Object{blockingToCancel, blockingToExpire})
			},
			targetType:  opsv1alpha1.RestartType,
			expectErr:   true,
			errContains: "patch failed",
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			targetOps := testutil.NewOpsRequestBuilder("priority", testutil.TestNamespace).
				WithClusterName(clusterName).
				WithType(tt.targetType).
				WithPhase(opsv1alpha1.OpsCreatingPhase).
				WithInstanceLabel(clusterName).
				Build()
			result, err := (priorityOps{}).decide(ctx, k8sClient, targetOps)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectDecision, result.Decision)

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}

// verifyCancelledOps 验证指定的 OpsRequest 是否被标记为 cancel
func verifyCancelledOps(t *testing.T, c client.Client, opsNames []string) {
	ctx := context.Background()
	for _, name := range opsNames {
		ops := &opsv1alpha1.OpsRequest{}
		err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: name}, ops)
		require.NoError(t, err, "failed to get OpsRequest %s", name)
		assert.True(t, ops.Spec.Cancel, "OpsRequest %s should be marked as cancelled", name)
	}
}

func TestCancelOpsDecide(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"

	testCases := []struct {
		name           string
		clientSetup    func() client.Client
		setup          func(client.Client) error
		targetType     opsv1alpha1.OpsType
		expectDecision preflightDecision
		expectErr      bool
		errContains    string
		verify         func(t *testing.T, c client.Client)
	}{
		{
			name:           "no_existing_ops",
			clientSetup:    func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup:          func(c client.Client) error { return nil },
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
		},
		{
			name:        "only_non_blocking_ops",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				succeeded := testutil.NewOpsRequestBuilder("succeeded", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					WithInstanceLabel(clusterName).
					Build()

				cancelled := testutil.NewOpsRequestBuilder("cancelled", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsCancelledPhase).
					WithCancel().
					WithInstanceLabel(clusterName).
					Build()

				failed := testutil.NewOpsRequestBuilder("failed", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsFailedPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{succeeded, cancelled, failed})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
		},
		{
			name:        "blocking_ops_cancel_success",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				running := testutil.NewOpsRequestBuilder("running-scale", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				pending := testutil.NewOpsRequestBuilder("pending-scale", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{running, pending})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				verifyCancelledOps(t, c, []string{"running-scale", "pending-scale"})
			},
		},
		{
			name:        "mixed_blocking_and_non_blocking",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				// 非阻塞操作：已成功
				succeeded := testutil.NewOpsRequestBuilder("succeeded", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					WithInstanceLabel(clusterName).
					Build()

				// 非阻塞操作：正在取消中
				cancelling := testutil.NewOpsRequestBuilder("cancelling", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsCancellingPhase).
					WithCancel().
					WithInstanceLabel(clusterName).
					Build()

				// 阻塞操作：运行中
				running := testutil.NewOpsRequestBuilder("running", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{succeeded, cancelling, running})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				// 只有运行中的操作应该被取消
				verifyCancelledOps(t, c, []string{"running"})

				// 验证其他操作保持不变
				ops := &opsv1alpha1.OpsRequest{}
				err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "cancelling"}, ops)
				require.NoError(t, err)
				assert.True(t, ops.Spec.Cancel, "already cancelling ops should remain cancelled")
			},
		},
		{
			name:        "multiple_blocking_ops",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				running := testutil.NewOpsRequestBuilder("running", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				pending := testutil.NewOpsRequestBuilder("pending", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					WithInstanceLabel(clusterName).
					Build()

				creating := testutil.NewOpsRequestBuilder("creating", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsCreatingPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{running, pending, creating})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				verifyCancelledOps(t, c, []string{"running", "pending", "creating"})
			},
		},
		{
			name:        "different_ops_type_isolation",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				// 不同类型的运行操作不应干扰
				restart := testutil.NewOpsRequestBuilder("restart", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				volumeExpansion := testutil.NewOpsRequestBuilder("volume", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.VolumeExpansionType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{restart, volumeExpansion})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				// 验证不同类型的操作没有被取消
				ops := &opsv1alpha1.OpsRequest{}
				err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "restart"}, ops)
				require.NoError(t, err)
				assert.False(t, ops.Spec.Cancel, "different type ops should not be cancelled")

				err = c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "volume"}, ops)
				require.NoError(t, err)
				assert.False(t, ops.Spec.Cancel, "different type ops should not be cancelled")
			},
		},
		{
			name:        "different_cluster_isolation",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				// 不同集群的同类型操作不应干扰
				otherCluster := testutil.NewOpsRequestBuilder("other-cluster-scale", testutil.TestNamespace).
					WithClusterName("other-cluster").
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel("other-cluster").
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{otherCluster})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				// 验证不同集群的操作没有被取消
				ops := &opsv1alpha1.OpsRequest{}
				err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "other-cluster-scale"}, ops)
				require.NoError(t, err)
				assert.False(t, ops.Spec.Cancel, "different cluster ops should not be cancelled")
			},
		},
		{
			name:        "vertical_scaling_target_type",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				running := testutil.NewOpsRequestBuilder("running-vertical", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.VerticalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{running})
			},
			targetType:     opsv1alpha1.VerticalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				verifyCancelledOps(t, c, []string{"running-vertical"})
			},
		},
		{
			name: "list_ops_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list failed")).
					Build()
			},
			targetType:  opsv1alpha1.HorizontalScalingType,
			expectErr:   true,
			errContains: "list opsrequests for preflight",
		},
		{
			name: "cancel_ops_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithPatchError(errors.New("mock patch failed")).
					Build()
			},
			setup: func(c client.Client) error {
				running := testutil.NewOpsRequestBuilder("failing-ops", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{running})
			},
			targetType:  opsv1alpha1.HorizontalScalingType,
			expectErr:   true,
			errContains: "failed to gracefully cancel",
		},
		{
			name:        "not_found_error_handled",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				// 不创建任何 OpsRequest，模拟 NotFound 情况
				return nil
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
		},
		{
			name:        "already_cancelled_ops_ignored",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				// 已经被标记为取消的操作
				alreadyCancelled := testutil.NewOpsRequestBuilder("already-cancelled", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					WithCancel().
					WithInstanceLabel(clusterName).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{alreadyCancelled})
			},
			targetType:     opsv1alpha1.HorizontalScalingType,
			expectDecision: preflightProceed,
			verify: func(t *testing.T, c client.Client) {
				// 验证已取消的操作保持 cancel 状态
				ops := &opsv1alpha1.OpsRequest{}
				err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: "already-cancelled"}, ops)
				require.NoError(t, err)
				assert.True(t, ops.Spec.Cancel, "already cancelled ops should remain cancelled")
			},
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			targetOps := testutil.NewOpsRequestBuilder("target", testutil.TestNamespace).
				WithClusterName(clusterName).
				WithType(tt.targetType).
				WithPhase(opsv1alpha1.OpsCreatingPhase).
				WithInstanceLabel(clusterName).
				Build()

			result, err := (cancelOps{}).decide(ctx, k8sClient, targetOps)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectDecision, result.Decision)

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}
