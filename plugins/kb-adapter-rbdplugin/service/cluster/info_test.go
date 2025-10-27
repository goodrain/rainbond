package cluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furutachiKurea/block-mechanica/internal/model"
	"github.com/furutachiKurea/block-mechanica/internal/testutil"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConnectInfo(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() client.Client
		setupCtx    func() context.Context // 添加 context 设置
		rbdService  model.RBDService
		expectInfo  []model.ConnectInfo
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful_connection_info",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-test", testutil.TestNamespace).
					WithServiceID("test-service").
					Build()

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-test-mysql-account-root",
						Namespace: testutil.TestNamespace,
					},
					Data: map[string][]byte{
						"username": []byte("root"),
						"password": []byte("secretpass"),
					},
				}

				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, testutil.CreateObjects(ctx, c, []client.Object{cluster, secret}))
				return c
			},
			setupCtx:   func() context.Context { return context.Background() },
			rbdService: model.RBDService{ServiceID: "test-service"},
			expectInfo: []model.ConnectInfo{
				{
					User:     "root",
					Password: "secretpass",
				},
			},
			expectError: false,
		},
		{
			name: "timeout_while_waiting_for_secret",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-test", testutil.TestNamespace).
					WithServiceID("test-service").
					Build()

				// 不创建 Secret，模拟 Secret 永远不会出现的情况
				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, c.Create(ctx, cluster))
				return c
			},
			setupCtx: func() context.Context {
				// 创建一个已经超时的 context
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				time.Sleep(2 * time.Millisecond) // 确保 context 已经超时
				defer cancel()
				return ctx
			},
			rbdService:  model.RBDService{ServiceID: "test-service"},
			expectError: true,
			errorMsg:    "wait for secret",
		},
		{
			name: "cluster_not_found",
			setupClient: func() client.Client {
				return testutil.NewFakeClient()
			},
			setupCtx:    func() context.Context { return context.Background() },
			rbdService:  model.RBDService{ServiceID: "nonexistent-service"},
			expectError: true,
			errorMsg:    "get cluster by service_id",
		},
		{
			name: "secret_empty_username",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-test", testutil.TestNamespace).
					WithServiceID("test-service").
					Build()

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-test-mysql-account-root",
						Namespace: testutil.TestNamespace,
					},
					Data: map[string][]byte{
						"username": []byte(""), // 空字符串
						"password": []byte("secretpass"),
					},
				}

				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, testutil.CreateObjects(ctx, c, []client.Object{cluster, secret}))
				return c
			},
			setupCtx:    func() context.Context { return context.Background() },
			rbdService:  model.RBDService{ServiceID: "test-service"},
			expectError: true,
			errorMsg:    "get username",
		},
		{
			name: "secret_empty_password",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-test", testutil.TestNamespace).
					WithServiceID("test-service").
					Build()

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mysql-test-mysql-account-root",
						Namespace: testutil.TestNamespace,
					},
					Data: map[string][]byte{
						"username": []byte("root"),
						"password": []byte(""), // 空字符串
					},
				}

				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, testutil.CreateObjects(ctx, c, []client.Object{cluster, secret}))
				return c
			},
			setupCtx:    func() context.Context { return context.Background() },
			rbdService:  model.RBDService{ServiceID: "test-service"},
			expectError: true,
			errorMsg:    "get password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			service := NewService(c)

			ctx := context.Background()
			if tt.setupCtx != nil {
				ctx = tt.setupCtx()
			}

			result, err := service.GetConnectInfo(ctx, tt.rbdService)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectInfo, result)
			}
		})
	}
}

func TestGetClusterDetail(t *testing.T) {
	tests := []struct {
		name         string
		setupClient  func() client.Client
		rbdService   model.RBDService
		expectDetail *model.ClusterDetail
		expectError  bool
		errorMsg     string
	}{
		{
			name: "cluster_detail_without_backup",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-no-backup", testutil.TestNamespace).
					WithServiceID("no-backup-service").
					WithComponentResources("mysql",
						testutil.Resources("250m", "512Mi"),
						testutil.Resources("500m", "1Gi")).
					WithComponentVolumeClaimTemplate("mysql", "data", "", "5Gi").
					WithPhase(kbappsv1.RunningClusterPhase).
					Build()

				// 简化的 fake c，InstanceSet 不存在时会跳过组件
				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, testutil.CreateObjects(ctx, c, []client.Object{cluster}))
				return c
			},
			rbdService: model.RBDService{ServiceID: "no-backup-service"},
			expectDetail: &model.ClusterDetail{
				Basic: model.BasicInfo{
					ClusterInfo: model.ClusterInfo{
						Name:              "mysql-no-backup",
						Namespace:         testutil.TestNamespace,
						Type:              "mysql",
						Version:           "", // NewMySQLCluster 没有设置 serviceVersion
						StorageClass:      "",
						TerminationPolicy: "", // NewMySQLCluster 没有设置 terminationPolicy
					},
					RBDService: model.RBDService{ServiceID: "no-backup-service"},
					Status: model.ClusterStatus{
						Status:    "running",
						StatusCN:  "运行中",
						StartTime: "",
					},
					Replicas:        []model.Status{}, // 空的副本列表，因为没有 InstanceSet
					IsSupportBackup: true,
				},
				Resource: model.ClusterResourceStatus{
					CPUMilli:  500,  // 500m
					MemoryMi:  1024, // 1Gi
					StorageGi: 5,    // 5Gi，通过 WithComponentVolumeClaimTemplate 设置
					Replicas:  1,
				},
				Backup: model.BackupInfo{}, // 空的备份信息，因为集群没有备份配置
			},
			expectError: false,
		},
		{
			name: "cluster_not_found",
			setupClient: func() client.Client {
				return testutil.NewFakeClient()
			},
			rbdService:  model.RBDService{ServiceID: "nonexistent-service"},
			expectError: true,
		},
		{
			name: "get_cluster_pods_error",
			setupClient: func() client.Client {
				cluster := testutil.NewMySQLCluster("mysql-pods-error", testutil.TestNamespace).
					WithServiceID("pods-error-service").
					Build()

				c := testutil.NewErrorClientBuilder(cluster).
					WithListError(errors.New("pods list failed")).
					Build()
				return c
			},
			rbdService:  model.RBDService{ServiceID: "pods-error-service"},
			expectError: true,
			errorMsg:    "pods list failed",
		},
		{
			name: "invalid_backup_cron_expression",
			setupClient: func() client.Client {
				backup := &kbappsv1.ClusterBackup{
					CronExpression:  "invalid-cron",
					RetentionPeriod: "7d",
					RepoName:        "test-repo",
				}

				cluster := testutil.NewMySQLCluster("mysql-invalid-cron", testutil.TestNamespace).
					WithServiceID("invalid-cron-service").
					WithBackup(backup).
					Build()

				c := testutil.NewFakeClient()
				ctx := context.Background()
				require.NoError(t, testutil.CreateObjects(ctx, c, []client.Object{cluster}))
				return c
			},
			rbdService:  model.RBDService{ServiceID: "invalid-cron-service"},
			expectError: true,
			errorMsg:    "build backup info", // 应该在解析 cron 表达式时失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			service := NewService(c)
			ctx := context.Background()

			result, err := service.GetClusterDetail(ctx, tt.rbdService)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.expectDetail != nil && result != nil {
					assert.Equal(t, tt.expectDetail.Basic.ClusterInfo, result.Basic.ClusterInfo)
					assert.Equal(t, tt.expectDetail.Basic.RBDService, result.Basic.RBDService)
					assert.Equal(t, tt.expectDetail.Basic.Status, result.Basic.Status)
					assert.Equal(t, tt.expectDetail.Basic.IsSupportBackup, result.Basic.IsSupportBackup)
					assert.Equal(t, tt.expectDetail.Resource, result.Resource)
					if tt.expectDetail.Backup.BackupRepo != "" {
						assert.Equal(t, tt.expectDetail.Backup, result.Backup)
					}
				}
			}
		})
	}
}
