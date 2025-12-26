package cluster

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCleanupClusterOpsRequests(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"
	cluster := testutil.NewMySQLCluster(clusterName, testutil.TestNamespace).
		WithServiceID(testutil.TestServiceID).
		Build()

	tests := []struct {
		name        string
		clientSetup func() client.Client
		setup       func(client.Client) error
		expectErr   bool
		errContains string
		verify      func(*testing.T, client.Client)
	}{
		{
			name: "non_final_list_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list failed")).
					Build()
			},
			expectErr:   true,
			errContains: "get existing opsrequests",
		},
		{
			name: "blocking_cleanup_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithPatchError(errors.New("patch failed")).
					Build()
			},
			setup: func(c client.Client) error {
				blockingCancel := testutil.NewOpsRequestBuilder("cancel-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				blockingExpire := testutil.NewOpsRequestBuilder("expire-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					Build()
				timeout := int32(300)
				blockingExpire.Spec.TimeoutSeconds = &timeout
				return testutil.CreateObjects(ctx, c, []client.Object{blockingCancel, blockingExpire})
			},
			expectErr:   true,
			errContains: "cleanup blocking ops",
		},
		{
			name: "all_ops_list_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list all failed")).
					Build()
			},
			expectErr:   true,
			errContains: "get existing opsrequests",
		},
		{
			name: "no_ops_present",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			verify: func(t *testing.T, c client.Client) {
				var list opsv1alpha1.OpsRequestList
				err := c.List(ctx, &list, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, list.Items, 0)
			},
		},
		{
			name: "cleanup_and_delete_all",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) error {
				blockingCancel := testutil.NewOpsRequestBuilder("cancel-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				blockingExpire := testutil.NewOpsRequestBuilder("expire-block", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestartType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				timeout := int32(600)
				blockingExpire.Spec.TimeoutSeconds = &timeout
				finalOps := testutil.NewOpsRequestBuilder("final", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{blockingCancel, blockingExpire, finalOps})
			},
			verify: func(t *testing.T, c client.Client) {
				var list opsv1alpha1.OpsRequestList
				err := c.List(ctx, &list, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, list.Items, 0)
			},
		},
		{
			name: "only_final_ops",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) error {
				finalOps := []client.Object{
					testutil.NewOpsRequestBuilder("succeeded", testutil.TestNamespace).
						WithClusterName(clusterName).
						WithType(opsv1alpha1.BackupType).
						WithInstanceLabel(clusterName).
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("failed", testutil.TestNamespace).
						WithClusterName(clusterName).
						WithType(opsv1alpha1.RestoreType).
						WithInstanceLabel(clusterName).
						WithPhase(opsv1alpha1.OpsFailedPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, finalOps)
			},
			verify: func(t *testing.T, c client.Client) {
				var list opsv1alpha1.OpsRequestList
				err := c.List(ctx, &list, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, list.Items, 0)
			},
		},
		{
			name: "delete_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithDeleteError(errors.New("delete failed")).
					Build()
			},
			setup: func(c client.Client) error {
				op := testutil.NewOpsRequestBuilder("delete-me", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{op})
			},
			expectErr:   true,
			errContains: "delete all ops",
		},
		{
			name: "ignore_not_found",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) error {
				// 不创建任何对象，模拟对象已被其他进程删除的场景
				return nil
			},
			verify: func(t *testing.T, c client.Client) {
				// 验证没有创建任何OpsRequest对象
				var list opsv1alpha1.OpsRequestList
				err := c.List(ctx, &list, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, list.Items, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()
			svc := &Service{client: k8sClient}

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			err := svc.cleanupClusterOpsRequests(ctx, cluster.DeepCopy())

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

func TestDeleteAllOpsRequestsConcurrently(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"

	tests := []struct {
		name        string
		clientSetup func() client.Client
		setup       func(client.Client) error
		ops         []opsv1alpha1.OpsRequest
		expectErr   bool
		errContains string
		verify      func(*testing.T, client.Client)
	}{
		{
			name:        "empty_list",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			ops:         nil,
		},
		{
			name:        "delete_success",
			clientSetup: func() client.Client { return testutil.NewFakeClientWithIndexes() },
			setup: func(c client.Client) error {
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("success-1", testutil.TestNamespace).
						WithClusterName(clusterName).
						WithType(opsv1alpha1.HorizontalScalingType).
						WithInstanceLabel(clusterName).
						WithPhase(opsv1alpha1.OpsSucceedPhase).
						Build(),
					testutil.NewOpsRequestBuilder("success-2", testutil.TestNamespace).
						WithClusterName(clusterName).
						WithType(opsv1alpha1.VerticalScalingType).
						WithInstanceLabel(clusterName).
						WithPhase(opsv1alpha1.OpsCancelledPhase).
						WithCancel().
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			ops: []opsv1alpha1.OpsRequest{
				*testutil.NewOpsRequestBuilder("success-1", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					Build(),
				*testutil.NewOpsRequestBuilder("success-2", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.VerticalScalingType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsCancelledPhase).
					WithCancel().
					Build(),
			},
			verify: func(t *testing.T, c client.Client) {
				for _, name := range []string{"success-1", "success-2"} {
					op := &opsv1alpha1.OpsRequest{}
					err := c.Get(ctx, types.NamespacedName{Namespace: testutil.TestNamespace, Name: name}, op)
					assert.Error(t, err)
					assert.True(t, apierrors.IsNotFound(err))
				}
			},
		},
		{
			name: "delete_not_found",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) error {
				// 不创建任何对象，模拟要删除的对象已被其他进程删除
				return nil
			},
			ops: []opsv1alpha1.OpsRequest{
				*testutil.NewOpsRequestBuilder("gone", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.RestoreType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build(),
			},
		},
		{
			name: "delete_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithDeleteError(errors.New("delete failed")).
					Build()
			},
			setup: func(c client.Client) error {
				ops := []client.Object{
					testutil.NewOpsRequestBuilder("bad", testutil.TestNamespace).
						WithClusterName(clusterName).
						WithType(opsv1alpha1.BackupType).
						WithInstanceLabel(clusterName).
						WithPhase(opsv1alpha1.OpsRunningPhase).
						Build(),
				}
				return testutil.CreateObjects(ctx, c, ops)
			},
			ops: []opsv1alpha1.OpsRequest{
				*testutil.NewOpsRequestBuilder("bad", testutil.TestNamespace).
					WithClusterName(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithInstanceLabel(clusterName).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build(),
			},
			expectErr:   true,
			errContains: "failed to delete opsrequest bad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()
			svc := &Service{client: k8sClient}

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			err := svc.deleteAllOpsRequestsConcurrently(ctx, tt.ops)

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

func TestExtractSecretRefs(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *kbappsv1.Cluster
		expected []string
	}{
		{
			name: "single_secret",
			cluster: testutil.NewRedisCluster("test", testutil.TestNamespace).
				WithSystemAccountSecret("default", "secret-1").
				Build(),
			expected: []string{"secret-1"},
		},
		{
			name: "multiple_unique_secrets",
			cluster: func() *kbappsv1.Cluster {
				c := testutil.NewRedisCluster("test", testutil.TestNamespace).Build()
				c.Spec.ComponentSpecs = []kbappsv1.ClusterComponentSpec{
					{
						Name: "comp1",
						SystemAccounts: []kbappsv1.ComponentSystemAccount{
							{Name: "acc1", SecretRef: &kbappsv1.ProvisionSecretRef{Name: "secret-1"}},
						},
					},
					{
						Name: "comp2",
						SystemAccounts: []kbappsv1.ComponentSystemAccount{
							{Name: "acc2", SecretRef: &kbappsv1.ProvisionSecretRef{Name: "secret-2"}},
						},
					},
				}
				return c
			}(),
			expected: []string{"secret-1", "secret-2"},
		},
		{
			name: "duplicate_secrets_should_dedup",
			cluster: func() *kbappsv1.Cluster {
				c := testutil.NewRedisCluster("test", testutil.TestNamespace).Build()
				c.Spec.ComponentSpecs = []kbappsv1.ClusterComponentSpec{
					{
						Name: "comp1",
						SystemAccounts: []kbappsv1.ComponentSystemAccount{
							{Name: "acc1", SecretRef: &kbappsv1.ProvisionSecretRef{Name: "secret-1"}},
							{Name: "acc2", SecretRef: &kbappsv1.ProvisionSecretRef{Name: "secret-1"}},
						},
					},
				}
				return c
			}(),
			expected: []string{"secret-1"},
		},
		{
			name:     "nil_cluster",
			cluster:  nil,
			expected: nil,
		},
		{
			name:     "no_systemaccounts",
			cluster:  testutil.NewRedisCluster("test", testutil.TestNamespace).Build(),
			expected: []string{},
		},
		{
			name: "secretref_nil",
			cluster: func() *kbappsv1.Cluster {
				c := testutil.NewRedisCluster("test", testutil.TestNamespace).Build()
				c.Spec.ComponentSpecs[0].SystemAccounts = []kbappsv1.ComponentSystemAccount{
					{Name: "acc1", SecretRef: nil},
				}
				return c
			}(),
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSecretRefs(tt.cluster)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestClusterReferencesSecret(t *testing.T) {
	tests := []struct {
		name       string
		cluster    *kbappsv1.Cluster
		secretName string
		expected   bool
	}{
		{
			name: "references_secret",
			cluster: testutil.NewRedisCluster("test", testutil.TestNamespace).
				WithSystemAccountSecret("default", "my-secret").
				Build(),
			secretName: "my-secret",
			expected:   true,
		},
		{
			name: "does_not_reference_secret",
			cluster: testutil.NewRedisCluster("test", testutil.TestNamespace).
				WithSystemAccountSecret("default", "other-secret").
				Build(),
			secretName: "my-secret",
			expected:   false,
		},
		{
			name:       "nil_cluster",
			cluster:    nil,
			secretName: "my-secret",
			expected:   false,
		},
		{
			name:       "empty_secret_name",
			cluster:    testutil.NewRedisCluster("test", testutil.TestNamespace).Build(),
			secretName: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clusterReferencesSecret(tt.cluster, tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSecretReferences(t *testing.T) {
	tests := []struct {
		name       string
		clusters   []kbappsv1.Cluster
		secretName string
		expected   int
	}{
		{
			name: "single_reference",
			clusters: []kbappsv1.Cluster{
				*testutil.NewRedisCluster("c1", testutil.TestNamespace).
					WithSystemAccountSecret("default", "secret-1").
					Build(),
			},
			secretName: "secret-1",
			expected:   1,
		},
		{
			name: "multiple_references",
			clusters: []kbappsv1.Cluster{
				*testutil.NewRedisCluster("c1", testutil.TestNamespace).
					WithSystemAccountSecret("default", "secret-1").
					Build(),
				*testutil.NewRedisCluster("c2", testutil.TestNamespace).
					WithSystemAccountSecret("default", "secret-1").
					Build(),
				*testutil.NewRedisCluster("c3", testutil.TestNamespace).
					WithSystemAccountSecret("default", "secret-2").
					Build(),
			},
			secretName: "secret-1",
			expected:   2,
		},
		{
			name: "no_references",
			clusters: []kbappsv1.Cluster{
				*testutil.NewRedisCluster("c1", testutil.TestNamespace).
					WithSystemAccountSecret("default", "other-secret").
					Build(),
			},
			secretName: "secret-1",
			expected:   0,
		},
		{
			name:       "empty_clusters",
			clusters:   []kbappsv1.Cluster{},
			secretName: "secret-1",
			expected:   0,
		},
		{
			name: "empty_secret_name",
			clusters: []kbappsv1.Cluster{
				*testutil.NewRedisCluster("c1", testutil.TestNamespace).Build(),
			},
			secretName: "",
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countSecretReferences(tt.clusters, tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeleteSecretsByCluster(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func(client.Client) (*kbappsv1.Cluster, error)
		clientSetup func() client.Client
		expectErr   bool
		errContains string
		verify      func(*testing.T, client.Client)
	}{
		{
			name: "single_cluster_single_secret_should_delete",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				cluster := testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
					WithServiceID("svc-001").
					WithSystemAccountSecret("default", "test-secret").
					Build()

				secret := testutil.NewSecretBuilder("test-secret", testutil.TestNamespace).
					WithServiceID("svc-001").
					Build()

				return cluster, testutil.CreateObjects(ctx, c, []client.Object{cluster, secret})
			},
			verify: func(t *testing.T, c client.Client) {
				var secret corev1.Secret
				err := c.Get(ctx, types.NamespacedName{
					Name:      "test-secret",
					Namespace: testutil.TestNamespace,
				}, &secret)
				assert.True(t, apierrors.IsNotFound(err), "secret should be deleted")
			},
		},
		{
			name: "two_clusters_share_secret_delete_one_should_preserve",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				sharedSecret := testutil.NewSecretBuilder("shared-secret", testutil.TestNamespace).
					WithServiceID("svc-001").
					Build()

				clusterA := testutil.NewRedisCluster("cluster-a", testutil.TestNamespace).
					WithServiceID("svc-001").
					WithSystemAccountSecret("default", "shared-secret").
					Build()

				clusterB := testutil.NewRedisCluster("cluster-b", testutil.TestNamespace).
					WithServiceID("svc-002").
					WithSystemAccountSecret("default", "shared-secret").
					Build()

				err := testutil.CreateObjects(ctx, c, []client.Object{sharedSecret, clusterA, clusterB})
				return clusterA, err
			},
			verify: func(t *testing.T, c client.Client) {
				var secret corev1.Secret
				err := c.Get(ctx, types.NamespacedName{
					Name:      "shared-secret",
					Namespace: testutil.TestNamespace,
				}, &secret)
				assert.NoError(t, err, "secret should still exist because cluster-b references it")
			},
		},
		{
			name: "restored_cluster_scenario",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				originalSecret := testutil.NewSecretBuilder("cluster-a-redis-account", testutil.TestNamespace).
					WithServiceID("svc-original").
					Build()

				originalCluster := testutil.NewRedisCluster("cluster-a", testutil.TestNamespace).
					WithServiceID("svc-original").
					WithSystemAccountSecret("default", "cluster-a-redis-account").
					Build()

				restoredCluster := testutil.NewRedisCluster("cluster-a-restore-abcd", testutil.TestNamespace).
					WithServiceID("svc-restored").
					WithSystemAccountSecret("default", "cluster-a-redis-account").
					Build()

				err := testutil.CreateObjects(ctx, c, []client.Object{originalSecret, originalCluster, restoredCluster})
				return originalCluster, err
			},
			verify: func(t *testing.T, c client.Client) {
				var secret corev1.Secret
				err := c.Get(ctx, types.NamespacedName{
					Name:      "cluster-a-redis-account",
					Namespace: testutil.TestNamespace,
				}, &secret)
				assert.NoError(t, err, "secret should be preserved for restored cluster")
			},
		},
		{
			name: "cluster_without_systemaccounts_should_skip",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				cluster := testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
					WithServiceID("svc-001").
					Build()
				cluster.Spec.ComponentSpecs[0].SystemAccounts = nil

				return cluster, testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
		},
		{
			name: "secret_not_found_should_not_fail",
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				cluster := testutil.NewRedisCluster("test-cluster", testutil.TestNamespace).
					WithSystemAccountSecret("default", "non-existent-secret").
					Build()

				return cluster, testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
		},
		{
			name: "list_clusters_error_should_fail",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(fmt.Errorf("list failed")).
					Build()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				cluster := testutil.NewRedisCluster("test", testutil.TestNamespace).
					WithSystemAccountSecret("default", "test-secret").
					Build()
				return cluster, nil
			},
			expectErr:   true,
			errContains: "list clusters in namespace",
		},
		{
			name: "delete_secret_error_should_record",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithDeleteError(fmt.Errorf("delete failed")).
					Build()
			},
			setup: func(c client.Client) (*kbappsv1.Cluster, error) {
				cluster := testutil.NewRedisCluster("test", testutil.TestNamespace).
					WithSystemAccountSecret("default", "test-secret").
					Build()

				secret := testutil.NewSecretBuilder("test-secret", testutil.TestNamespace).Build()

				return cluster, testutil.CreateObjects(ctx, c, []client.Object{cluster, secret})
			},
			expectErr:   true,
			errContains: "failed to delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()
			svc := &Service{client: k8sClient}

			cluster, err := tt.setup(k8sClient)
			require.NoError(t, err, "setup should not fail")

			err = svc.deleteSecretsByCluster(ctx, cluster)

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

func TestCreateCluster(t *testing.T) {
	tests := []struct {
		name        string
		input       model.ClusterInput
		clientSetup func() client.Client
		setup       func(client.Client) error
		useTimeout  bool
		expectErr   bool
		errContains string
		verify      func(*testing.T, client.Client)
	}{
		{
			name: "empty_name",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name: "",
					Type: "mysql",
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			expectErr:   true,
			errContains: "name is required",
		},
		{
			name: "unsupported_cluster_type",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:      "test-unsupported",
					Namespace: testutil.TestNamespace,
					Type:      "unsupported-db",
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			expectErr:   true,
			errContains: "unsupported cluster type",
		},
		{
			name: "build_cluster_fails_invalid_resources",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:              "test-invalid-resources",
					Namespace:         testutil.TestNamespace,
					Type:              "mysql",
					Version:           "8.0.30",
					StorageClass:      "standard",
					TerminationPolicy: kbappsv1.Delete,
				},
				ClusterResource: model.ClusterResource{
					CPU:      "invalid-cpu",
					Memory:   "2Gi",
					Storage:  "10Gi",
					Replicas: 1,
				},
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "default-repo",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: datav1alpha.RetentionPeriod("7d"),
				},
				RBDService: model.RBDService{
					ServiceID: testutil.TestServiceID,
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			expectErr:   true,
			errContains: "build mysql cluster",
		},
		{
			name: "success_without_custom_secret",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:              "test-mysql",
					Namespace:         testutil.TestNamespace,
					Type:              "mysql",
					Version:           "8.0.30",
					StorageClass:      "standard",
					TerminationPolicy: kbappsv1.Delete,
				},
				ClusterResource: model.ClusterResource{
					CPU:      "1",
					Memory:   "2Gi",
					Storage:  "10Gi",
					Replicas: 1,
				},
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "default-repo",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: datav1alpha.RetentionPeriod("7d"),
				},
				RBDService: model.RBDService{
					ServiceID: testutil.TestServiceID,
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			verify: func(t *testing.T, c client.Client) {
				ctx := context.Background()
				var clusterList kbappsv1.ClusterList
				err := c.List(ctx, &clusterList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				require.Len(t, clusterList.Items, 1)

				cluster := clusterList.Items[0]
				assert.Equal(t, testutil.TestServiceID, cluster.Labels[index.ServiceIDLabel])

				var secretList corev1.SecretList
				err = c.List(ctx, &secretList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, secretList.Items, 0)
			},
		},
		{
			name: "success_with_system_account",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:              "test-redis",
					Namespace:         testutil.TestNamespace,
					Type:              "redis",
					Version:           "7.0.6",
					StorageClass:      "standard",
					TerminationPolicy: kbappsv1.Delete,
				},
				ClusterResource: model.ClusterResource{
					CPU:      "1",
					Memory:   "2Gi",
					Storage:  "10Gi",
					Replicas: 1,
				},
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "default-repo",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: datav1alpha.RetentionPeriod("7d"),
				},
				RBDService: model.RBDService{
					ServiceID: testutil.TestServiceID,
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewFakeClientWithIndexes()
			},
			verify: func(t *testing.T, c client.Client) {
				ctx := context.Background()
				// 验证 Cluster 被创建
				var clusterList kbappsv1.ClusterList
				err := c.List(ctx, &clusterList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				require.Len(t, clusterList.Items, 1)

				cluster := clusterList.Items[0]
				assert.Equal(t, testutil.TestServiceID, cluster.Labels[index.ServiceIDLabel])

				// 验证 SystemAccounts 配置正确
				found := false
				for _, comp := range cluster.Spec.ComponentSpecs {
					if comp.Name == "redis" {
						require.NotEmpty(t, comp.SystemAccounts)
						assert.Equal(t, "default", comp.SystemAccounts[0].Name)
						assert.NotNil(t, comp.SystemAccounts[0].SecretRef)
						found = true
						break
					}
				}
				assert.True(t, found, "redis component not found")

				// 验证 Secret 被创建且内容正确
				var secretList corev1.SecretList
				err = c.List(ctx, &secretList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				require.Len(t, secretList.Items, 1)

				secret := secretList.Items[0]
				assert.Equal(t, testutil.TestServiceID, secret.Labels[index.ServiceIDLabel])
				assert.Contains(t, secret.Data, "username")
				assert.Contains(t, secret.Data, "password")
				assert.Equal(t, "default", string(secret.Data["username"]))
				assert.NotEmpty(t, secret.Data["password"])
			},
		},
		{
			name: "secret_create_fails_no_resources_left",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:              "test-redis-fail",
					Namespace:         testutil.TestNamespace,
					Type:              "redis",
					Version:           "7.0.6",
					StorageClass:      "standard",
					TerminationPolicy: kbappsv1.Delete,
				},
				ClusterResource: model.ClusterResource{
					CPU:      "1",
					Memory:   "2Gi",
					Storage:  "10Gi",
					Replicas: 1,
				},
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "default-repo",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: datav1alpha.RetentionPeriod("7d"),
				},
				RBDService: model.RBDService{
					ServiceID: testutil.TestServiceID,
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("secret create failed")).
					Build()
			},
			expectErr:   true,
			errContains: "create system account secret",
			verify: func(t *testing.T, c client.Client) {
				ctx := context.Background()
				// 验证没有 Cluster 被创建
				var clusterList kbappsv1.ClusterList
				err := c.List(ctx, &clusterList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, clusterList.Items, 0)

				// 验证没有 Secret 被创建
				var secretList corev1.SecretList
				err = c.List(ctx, &secretList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, secretList.Items, 0)
			},
		},
		{
			name: "cluster_create_fails_secret_cleaned",
			input: model.ClusterInput{
				ClusterInfo: model.ClusterInfo{
					Name:              "test-redis-conflict",
					Namespace:         testutil.TestNamespace,
					Type:              "redis",
					Version:           "7.0.6",
					StorageClass:      "standard",
					TerminationPolicy: kbappsv1.Delete,
				},
				ClusterResource: model.ClusterResource{
					CPU:      "1",
					Memory:   "2Gi",
					Storage:  "10Gi",
					Replicas: 1,
				},
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "default-repo",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: datav1alpha.RetentionPeriod("7d"),
				},
				RBDService: model.RBDService{
					ServiceID: testutil.TestServiceID,
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateErrorForType(&kbappsv1.Cluster{}, errors.New("cluster create failed")).
					Build()
			},
			expectErr:   true,
			errContains: "create cluster",
			verify: func(t *testing.T, c client.Client) {
				ctx := context.Background()
				// 验证没有 Cluster 被创建
				var clusterList kbappsv1.ClusterList
				err := c.List(ctx, &clusterList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, clusterList.Items, 0)

				// 验证 Secret 被清理（不存在）
				var secretList corev1.SecretList
				err = c.List(ctx, &secretList, client.InNamespace(testutil.TestNamespace))
				require.NoError(t, err)
				assert.Len(t, secretList.Items, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := tt.clientSetup()
			svc := &Service{client: k8sClient}

			if tt.setup != nil {
				require.NoError(t, tt.setup(k8sClient))
			}

			ctx := context.Background()
			if tt.useTimeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
			}

			cluster, err := svc.CreateCluster(ctx, tt.input)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, cluster)
				if tt.verify != nil {
					tt.verify(t, k8sClient)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cluster)

			if tt.verify != nil {
				tt.verify(t, k8sClient)
			}
		})
	}
}
