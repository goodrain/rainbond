package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/testutil"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRestoreFromBackup(t *testing.T) {
	testCases := []struct {
		name         string
		clientSetup  func() client.Client
		setup        func(client.Client) error
		oldServiceID string
		newServiceID string
		backupName   string
		expectErr    bool
		errContains  string
	}{
		{
			name:         "source_cluster_not_found",
			clientSetup:  func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup:        func(c client.Client) error { return nil }, // 不创建任何集群
			oldServiceID: "nonexistent-service",
			newServiceID: "new-service",
			backupName:   "test-backup",
			expectErr:    true,
			errContains:  "get cluster by service_id",
		},
		{
			name: "get_cluster_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list clusters failed")).
					Build()
			},
			oldServiceID: "any-service",
			newServiceID: "new-service",
			backupName:   "test-backup",
			expectErr:    true,
			errContains:  "get cluster by service_id",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient := tt.clientSetup()

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			service := NewService(k8sClient)

			result, err := service.RestoreFromBackup(ctx, tt.oldServiceID, tt.newServiceID, tt.backupName)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, result)
		})
	}
}

func TestWaitForRestoredCluster(t *testing.T) {
	testCases := []struct {
		name              string
		clientSetup       func() client.Client
		setup             func(client.Client) error
		opsPhase          opsv1alpha1.OpsPhase
		createCluster     bool
		expectErr         bool
		expectTimeout     bool
		expectClusterName string
		verify            func(*testing.T, client.Client)
	}{
		{
			name:              "cluster_created_successfully",
			clientSetup:       func() client.Client { return testutil.NewFakeClientWithIndexes() },
			opsPhase:          opsv1alpha1.OpsSucceedPhase,
			createCluster:     true,
			expectClusterName: "restore-cluster-xyz",
		},
		{
			name:        "ops_failed_calls_handler",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			opsPhase:    opsv1alpha1.OpsFailedPhase,
			expectErr:   true,
			verify: func(t *testing.T, c client.Client) {
				// 验证失败的 OpsRequest 被正确处理
				verifyOpsRequestLabelUpdated(t, c, "restore-ops", testutil.TestNamespace, "old-cluster")
			},
		},
		{
			name:        "ops_cancelled_calls_handler",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			opsPhase:    opsv1alpha1.OpsCancelledPhase,
			expectErr:   true,
			verify: func(t *testing.T, c client.Client) {
				verifyOpsRequestLabelUpdated(t, c, "restore-ops", testutil.TestNamespace, "old-cluster")
			},
		},
		{
			name:        "ops_aborted_calls_handler",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			opsPhase:    opsv1alpha1.OpsAbortedPhase,
			expectErr:   true,
			verify: func(t *testing.T, c client.Client) {
				verifyOpsRequestLabelUpdated(t, c, "restore-ops", testutil.TestNamespace, "old-cluster")
			},
		},
		{
			name:          "timeout_calls_cleanup",
			clientSetup:   func() client.Client { return testutil.NewFakeClientWithIndexes() },
			opsPhase:      opsv1alpha1.OpsRunningPhase,
			expectErr:     true,
			expectTimeout: true,
			verify: func(t *testing.T, c client.Client) {
				// 验证超时后 OpsRequest 被清理
				verifyOpsRequestDeleted(t, c, "restore-ops", testutil.TestNamespace)
			},
		},
		{
			name: "get_ops_status_error_continues",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithGetError(errors.New("get ops failed")).
					Build()
			},
			opsPhase:      opsv1alpha1.OpsRunningPhase,
			expectErr:     true,
			expectTimeout: true,
		},
		{
			name: "general_polling_error",
			clientSetup: func() client.Client {
				// 创建一个会导致 poll 函数返回非超时错误的客户端
				return testutil.NewErrorClientBuilder().
					WithGetError(errors.New("unexpected polling error")).
					Build()
			},
			opsPhase:  opsv1alpha1.OpsRunningPhase,
			expectErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient := tt.clientSetup()

			objects := []client.Object{}

			restoreOps := testutil.NewOpsRequestBuilder("restore-ops", testutil.TestNamespace).
				WithClusterName("restore-cluster-xyz").
				WithType(opsv1alpha1.RestoreType).
				WithRestore("test-backup").
				WithPhase(tt.opsPhase).
				WithInstanceLabel("restore-cluster-xyz").
				Build()
			objects = append(objects, restoreOps)

			if tt.createCluster {
				newCluster := testutil.NewMySQLCluster("restore-cluster-xyz", testutil.TestNamespace).Build()
				objects = append(objects, newCluster)
			}

			require.NoError(t, testutil.CreateObjects(ctx, k8sClient, objects))

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			service := NewService(k8sClient)

			testCtx := ctx
			if tt.expectTimeout {
				var cancel context.CancelFunc
				testCtx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}

			cluster, err := service.waitForRestoredCluster(testCtx, restoreOps, "old-cluster")

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectTimeout {
					assert.Contains(t, err.Error(), "timeout")
				}
				if tt.verify != nil {
					tt.verify(t, k8sClient)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cluster)
			if tt.expectClusterName != "" {
				assert.Equal(t, tt.expectClusterName, cluster.Name)
			}

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}

func TestHandleFailedRestoreOps(t *testing.T) {
	testCases := []struct {
		name        string
		clientSetup func() client.Client
		setup       func(client.Client) error
		oldCluster  string
		expectErr   bool
		verify      func(*testing.T, client.Client, *opsv1alpha1.OpsRequest)
	}{
		{
			name:        "patch_label_success",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			oldCluster:  "source-cluster",
			verify: func(t *testing.T, c client.Client, ops *opsv1alpha1.OpsRequest) {
				verifyOpsRequestLabelUpdated(t, c, ops.Name, ops.Namespace, "source-cluster")
			},
		},
		{
			name: "patch_operation_failure",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithPatchError(errors.New("patch failed")).
					Build()
			},
			oldCluster: "source-cluster",
			expectErr:  true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient := tt.clientSetup()

			ops := testutil.NewOpsRequestBuilder("failed-restore", testutil.TestNamespace).
				WithClusterName("restore-cluster-xyz").
				WithType(opsv1alpha1.RestoreType).
				WithPhase(opsv1alpha1.OpsFailedPhase).
				WithInstanceLabel("restore-cluster-xyz").
				Build()

			require.NoError(t, testutil.CreateObjects(ctx, k8sClient, []client.Object{ops}))

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			service := NewService(k8sClient)

			err := service.handleFailedRestoreOps(ctx, ops, tt.oldCluster)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.verify != nil {
				tt.verify(t, k8sClient, ops)
			}
		})
	}
}

func TestCleanupOpsRequest(t *testing.T) {
	testCases := []struct {
		name        string
		clientSetup func() client.Client
		createOps   bool
		reason      string
		expectErr   bool
		verify      func(*testing.T, client.Client)
	}{
		{
			name:        "delete_success",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			createOps:   true,
			reason:      "timeout",
			verify: func(t *testing.T, c client.Client) {
				verifyOpsRequestDeleted(t, c, "cleanup-ops", testutil.TestNamespace)
			},
		},
		{
			name: "delete_operation_failure",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithDeleteError(errors.New("delete failed")).
					Build()
			},
			createOps: true,
			reason:    "timeout",
			expectErr: true,
		},
		{
			name:        "resource_not_found_error",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			createOps:   false,
			reason:      "timeout",
			expectErr:   true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient := tt.clientSetup()

			ops := testutil.NewOpsRequestBuilder("cleanup-ops", testutil.TestNamespace).
				WithClusterName("test-cluster").
				WithType(opsv1alpha1.RestoreType).
				WithPhase(opsv1alpha1.OpsRunningPhase).
				Build()

			if tt.createOps {
				require.NoError(t, testutil.CreateObjects(ctx, k8sClient, []client.Object{ops}))
			}

			service := NewService(k8sClient)

			err := service.cleanupOpsRequest(ctx, ops, tt.reason)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}

// verifyOpsRequestDeleted 验证 OpsRequest 是否被正确删除
func verifyOpsRequestDeleted(t *testing.T, c client.Client, opsName, namespace string) {
	ctx := context.Background()
	ops := &opsv1alpha1.OpsRequest{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      opsName,
		Namespace: namespace,
	}, ops)
	assert.True(t, client.IgnoreNotFound(err) == nil, "OpsRequest should be deleted")
}

// verifyOpsRequestLabelUpdated 验证 OpsRequest 的 app.kubernetes.io/instance 标签被正确更新
func verifyOpsRequestLabelUpdated(t *testing.T, c client.Client, opsName, namespace, expectedClusterName string) {
	ctx := context.Background()
	ops := &opsv1alpha1.OpsRequest{}
	err := c.Get(ctx, types.NamespacedName{
		Name:      opsName,
		Namespace: namespace,
	}, ops)
	require.NoError(t, err, "failed to get OpsRequest")

	if assert.NotNil(t, ops.Labels, "OpsRequest should have labels") {
		assert.Equal(t, expectedClusterName, ops.Labels[constant.AppInstanceLabelKey],
			"OpsRequest should have updated app instance label")
	}
}
