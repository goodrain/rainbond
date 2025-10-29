package kbkit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreateLifecycleOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *kbappsv1.Cluster
		opsType       opsv1alpha1.OpsType
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil cluster should return error",
			cluster:       nil,
			opsType:       opsv1alpha1.RestartType,
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name:        "successful restart",
			cluster:     testutil.NewMySQLCluster("test-cluster", "default").Build(),
			opsType:     opsv1alpha1.RestartType,
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:        "successful stop",
			cluster:     testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
			opsType:     opsv1alpha1.StopType,
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:        "successful start",
			cluster:     testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
			opsType:     opsv1alpha1.StartType,
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:    "client create error should return error",
			cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
			opsType: opsv1alpha1.RestartType,
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create opsrequest",
		},
		{
			name:    "already exists error should be handled gracefully",
			cluster: testutil.NewMySQLCluster("existing-cluster", "default").Build(),
			opsType: opsv1alpha1.RestartType,
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateLifecycleOpsRequest(context.Background(), client, tt.cluster, tt.opsType)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateBackupOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *kbappsv1.Cluster
		backupMethod  string
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil cluster should return error",
			cluster:       nil,
			backupMethod:  "",
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name:         "successful backup with default method",
			cluster:      testutil.NewMySQLCluster("mysql-cluster", "default").Build(),
			backupMethod: "",
			clientSetup:  func() client.Client { return testutil.NewFakeClient() },
			expectError:  false,
		},
		{
			name:         "backup for PostgreSQL cluster",
			cluster:      testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
			backupMethod: "",
			clientSetup:  func() client.Client { return testutil.NewFakeClient() },
			expectError:  false,
		},
		{
			name:         "backup for MySQL cluster with different name",
			cluster:      testutil.NewMySQLCluster("mysql-cluster-2", "default").Build(),
			backupMethod: "",
			clientSetup:  func() client.Client { return testutil.NewFakeClient() },
			expectError:  false,
		},
		{
			name:         "custom backup",
			cluster:      testutil.NewMySQLCluster("custom-cluster", "default").Build(),
			backupMethod: "xtrabackup",
			clientSetup:  func() client.Client { return testutil.NewFakeClient() },
			expectError:  false,
		},
		{
			name:         "client create error",
			cluster:      testutil.NewMySQLCluster("test-cluster", "default").Build(),
			backupMethod: "",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name:         "already exists error",
			cluster:      testutil.NewMySQLCluster("existing-cluster", "default").Build(),
			backupMethod: "",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateBackupOpsRequest(context.Background(), client, tt.cluster, tt.backupMethod)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateHorizontalScalingOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		params        model.HorizontalScalingOpsParams
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name: "nil cluster should return error",
			params: model.HorizontalScalingOpsParams{
				Cluster:    nil,
				Components: []model.ComponentHorizontalScaling{},
			},
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name: "successful horizontal scaling with single component",
			params: model.HorizontalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentHorizontalScaling{
					{
						Name:          "mysql",
						DeltaReplicas: 1,
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "horizontal scaling with multiple components",
			params: model.HorizontalScalingOpsParams{
				Cluster: testutil.NewPostgreSQLCluster("redis-cluster", "default").Build(),
				Components: []model.ComponentHorizontalScaling{
					{
						Name:          "redis",
						DeltaReplicas: 2,
					},
					{
						Name:          "redis-sentinel",
						DeltaReplicas: 1,
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "scale down",
			params: model.HorizontalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("scale-down-cluster", "default").Build(),
				Components: []model.ComponentHorizontalScaling{
					{
						Name:          "mysql",
						DeltaReplicas: -1,
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "empty components list should still work",
			params: model.HorizontalScalingOpsParams{
				Cluster:    testutil.NewMySQLCluster("empty-components", "default").Build(),
				Components: []model.ComponentHorizontalScaling{},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "client create error should return error",
			params: model.HorizontalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentHorizontalScaling{
					{
						Name:          "mysql",
						DeltaReplicas: 1,
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name: "already exists error should be handled gracefully",
			params: model.HorizontalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("existing-cluster", "default").Build(),
				Components: []model.ComponentHorizontalScaling{
					{
						Name:          "mysql",
						DeltaReplicas: 1,
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateHorizontalScalingOpsRequest(context.Background(), client, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateVerticalScalingOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		params        model.VerticalScalingOpsParams
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name: "nil cluster should return error",
			params: model.VerticalScalingOpsParams{
				Cluster:    nil,
				Components: []model.ComponentVerticalScaling{},
			},
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name: "successful vertical scaling with single component",
			params: model.VerticalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentVerticalScaling{
					{
						Name:   "mysql",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("4Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "vertical scaling with multiple components",
			params: model.VerticalScalingOpsParams{
				Cluster: testutil.NewPostgreSQLCluster("redis-cluster", "default").Build(),
				Components: []model.ComponentVerticalScaling{
					{
						Name:   "redis",
						CPU:    resource.MustParse("1"),
						Memory: resource.MustParse("2Gi"),
					},
					{
						Name:   "redis-sentinel",
						CPU:    resource.MustParse("500m"),
						Memory: resource.MustParse("1Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "scale down resources",
			params: model.VerticalScalingOpsParams{
				Cluster: testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
				Components: []model.ComponentVerticalScaling{
					{
						Name:   "postgresql",
						CPU:    resource.MustParse("500m"),
						Memory: resource.MustParse("1Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "empty components list should still work",
			params: model.VerticalScalingOpsParams{
				Cluster:    testutil.NewMySQLCluster("empty-components", "default").Build(),
				Components: []model.ComponentVerticalScaling{},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "client create error should return error",
			params: model.VerticalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentVerticalScaling{
					{
						Name:   "mysql",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("4Gi"),
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name: "already exists error should be handled gracefully",
			params: model.VerticalScalingOpsParams{
				Cluster: testutil.NewMySQLCluster("existing-cluster", "default").Build(),
				Components: []model.ComponentVerticalScaling{
					{
						Name:   "mysql",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("4Gi"),
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateVerticalScalingOpsRequest(context.Background(), client, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateVolumeExpansionOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		params        model.VolumeExpansionOpsParams
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name: "nil cluster should return error",
			params: model.VolumeExpansionOpsParams{
				Cluster:    nil,
				Components: []model.ComponentVolumeExpansion{},
			},
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name: "successful volume expansion with single component",
			params: model.VolumeExpansionOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentVolumeExpansion{
					{
						Name:                    "mysql",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("20Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "volume expansion with multiple components",
			params: model.VolumeExpansionOpsParams{
				Cluster: testutil.NewPostgreSQLCluster("redis-cluster", "default").Build(),
				Components: []model.ComponentVolumeExpansion{
					{
						Name:                    "redis",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("30Gi"),
					},
					{
						Name:                    "redis-sentinel",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("10Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "multiple volume claim templates for single component",
			params: model.VolumeExpansionOpsParams{
				Cluster: testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
				Components: []model.ComponentVolumeExpansion{
					{
						Name:                    "postgresql",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("50Gi"),
					},
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "empty components list should still work",
			params: model.VolumeExpansionOpsParams{
				Cluster:    testutil.NewMySQLCluster("empty-components", "default").Build(),
				Components: []model.ComponentVolumeExpansion{},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name: "client create error should return error",
			params: model.VolumeExpansionOpsParams{
				Cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
				Components: []model.ComponentVolumeExpansion{
					{
						Name:                    "mysql",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("20Gi"),
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name: "already exists error should be handled gracefully",
			params: model.VolumeExpansionOpsParams{
				Cluster: testutil.NewMySQLCluster("existing-cluster", "default").Build(),
				Components: []model.ComponentVolumeExpansion{
					{
						Name:                    "mysql",
						VolumeClaimTemplateName: "data",
						Storage:                 resource.MustParse("25Gi"),
					},
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateVolumeExpansionOpsRequest(context.Background(), client, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateParameterChangeOpsRequest(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *kbappsv1.Cluster
		parameters    []model.ParameterEntry
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil cluster should return error",
			cluster:       nil,
			parameters:    []model.ParameterEntry{},
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name:    "successful parameter change with single parameter",
			cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
			parameters: []model.ParameterEntry{
				{
					Name:  "max_connections",
					Value: "200",
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:    "parameter change with multiple parameters",
			cluster: testutil.NewPostgreSQLCluster("pg-cluster", "default").Build(),
			parameters: []model.ParameterEntry{
				{
					Name:  "max_connections",
					Value: "300",
				},
				{
					Name:  "shared_buffers",
					Value: "256MB",
				},
				{
					Name:  "log_statement",
					Value: "all",
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:        "empty parameters list should still work",
			cluster:     testutil.NewMySQLCluster("empty-params", "default").Build(),
			parameters:  []model.ParameterEntry{},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:    "parameter with numeric value",
			cluster: testutil.NewPostgreSQLCluster("redis-cluster", "default").Build(),
			parameters: []model.ParameterEntry{
				{
					Name:  "timeout",
					Value: 300,
				},
				{
					Name:  "databases",
					Value: 16,
				},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
		},
		{
			name:    "client create error should return error",
			cluster: testutil.NewMySQLCluster("test-cluster", "default").Build(),
			parameters: []model.ParameterEntry{
				{
					Name:  "max_connections",
					Value: "200",
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name:    "already exists error should be handled gracefully",
			cluster: testutil.NewMySQLCluster("existing-cluster", "default").Build(),
			parameters: []model.ParameterEntry{
				{
					Name:  "max_connections",
					Value: "200",
				},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			err := kbkit.CreateParameterChangeOpsRequest(context.Background(), client, tt.cluster, tt.parameters)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateRestoreOpsRequest(t *testing.T) {
	restoredNamePrefix := func(original string) string {
		if strings.Contains(original, "-restore-") {
			idx := strings.LastIndex(original, "-restore-")
			return original[:idx] + "-restore-"
		}
		if lastDash := strings.LastIndex(original, "-"); lastDash > 0 {
			return original[:lastDash] + "-restore-"
		}
		return original + "-restore-"
	}

	tests := []struct {
		name          string
		cluster       *kbappsv1.Cluster
		backupName    string
		clientSetup   func() client.Client
		expectError   bool
		errorContains string
		validate      func(t *testing.T, result *opsv1alpha1.OpsRequest, err error)
	}{
		{
			name:          "nil cluster should return error",
			cluster:       nil,
			backupName:    "test-backup",
			clientSetup:   func() client.Client { return testutil.NewFakeClient() },
			expectError:   true,
			errorContains: "cluster is required",
		},
		{
			name:        "successful restore operation",
			cluster:     testutil.NewMySQLCluster("test-cluster", "default").Build(),
			backupName:  "mysql-backup-20240101",
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
			validate: func(t *testing.T, result *opsv1alpha1.OpsRequest, err error) {
				assert.NotNil(t, result)
				assert.Equal(t, opsv1alpha1.RestoreType, result.Spec.Type)
				assert.Equal(t, "default", result.Namespace)
				assert.NotEmpty(t, result.Name)
				assert.True(t, strings.HasPrefix(result.Name, "test-cluster-restore-"))

				// 验证恢复规格
				assert.NotNil(t, result.Spec.Restore)
				assert.Equal(t, "mysql-backup-20240101", result.Spec.Restore.BackupName)
			},
		},
		{
			name:        "empty backup name should succeed, kubeblocks will handle it",
			cluster:     testutil.NewMySQLCluster("test-cluster", "default").Build(),
			backupName:  "",
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			expectError: false,
			validate: func(t *testing.T, result *opsv1alpha1.OpsRequest, err error) {
				assert.Equal(t, "", result.Spec.Restore.BackupName)
			},
		},
		{
			name:       "client create error should return error",
			cluster:    testutil.NewMySQLCluster("test-cluster", "default").Build(),
			backupName: "test-backup",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(errors.New("create failed")).
					Build()
			},
			expectError:   true,
			errorContains: "create failed",
		},
		{
			name:       "already exists error should be handled gracefully",
			cluster:    testutil.NewMySQLCluster("existing-cluster", "default").Build(),
			backupName: "existing-backup",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithCreateError(apierrors.NewAlreadyExists(schema.GroupResource{}, "test-resource")).
					Build()
			},
			expectError:   true,
			errorContains: "operation skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			result, err := kbkit.CreateRestoreOpsRequest(context.Background(), client, tt.cluster, tt.backupName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, result) {
					assert.True(t, strings.HasPrefix(result.Spec.ClusterName, restoredNamePrefix(tt.cluster.Name)))
				}
				if tt.validate != nil {
					tt.validate(t, result, err)
				}
			}
		})
	}
}

func TestGetAllNonFinalOpsRequests(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		clientSetup   func() client.Client
		namespace     string
		clusterName   string
		expectError   bool
		errorContains string
		verify        func(*testing.T, []opsv1alpha1.OpsRequest)
	}{
		{
			name: "list error should bubble up",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().
					WithListError(errors.New("list failed")).
					Build()
			},
			namespace:     testutil.TestNamespace,
			clusterName:   "test-cluster",
			expectError:   true,
			errorContains: "list all opsrequests",
		},
		{
			name: "filter out non blocking operations",
			clientSetup: func() client.Client {
				namespace := testutil.TestNamespace
				clusterName := "filter-cluster"
				timeout := int32(600)

				runningOps := testutil.NewOpsRequestBuilder("running-ops", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				runningOps.Spec.TimeoutSeconds = &timeout

				pendingOps := testutil.NewOpsRequestBuilder("pending-ops", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.VerticalScalingType).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					Build()
				pendingOps.Spec.TimeoutSeconds = &timeout

				succeededOps := testutil.NewOpsRequestBuilder("succeeded-ops", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					Build()

				return testutil.NewFakeClient(runningOps, pendingOps, succeededOps)
			},
			namespace:   testutil.TestNamespace,
			clusterName: "filter-cluster",
			verify: func(t *testing.T, ops []opsv1alpha1.OpsRequest) {
				assert.Len(t, ops, 2)
				var names []string
				for _, op := range ops {
					names = append(names, op.Name)
				}
				assert.ElementsMatch(t, []string{"running-ops", "pending-ops"}, names)
			},
		},
		{
			name: "all non blocking operations should return empty list",
			clientSetup: func() client.Client {
				namespace := testutil.TestNamespace
				clusterName := "non-blocking"

				succeededOps := testutil.NewOpsRequestBuilder("ops-succeed", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithPhase(opsv1alpha1.OpsSucceedPhase).
					Build()

				cancelledOps := testutil.NewOpsRequestBuilder("ops-cancelled", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.RestoreType).
					WithPhase(opsv1alpha1.OpsCancelledPhase).
					WithCancel().
					Build()

				return testutil.NewFakeClient(succeededOps, cancelledOps)
			},
			namespace:   testutil.TestNamespace,
			clusterName: "non-blocking",
			verify: func(t *testing.T, ops []opsv1alpha1.OpsRequest) {
				assert.Empty(t, ops)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientSetup()
			opsList, err := kbkit.GetAllNonFinalOpsRequests(ctx, client, tt.namespace, tt.clusterName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, opsList)
			}
		})
	}
}

func TestCleanupBlockingOps(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		prepare       func() (client.Client, []opsv1alpha1.OpsRequest)
		expectError   bool
		errorContains string
		verify        func(*testing.T, client.Client)
	}{
		{
			name: "empty blocking list should succeed",
			prepare: func() (client.Client, []opsv1alpha1.OpsRequest) {
				return testutil.NewFakeClient(), nil
			},
		},
		{
			name: "cancel supported ops and expire the rest",
			prepare: func() (client.Client, []opsv1alpha1.OpsRequest) {
				namespace := testutil.TestNamespace
				clusterName := "cleanup-cluster"
				timeoutScale := int32(600)
				timeoutBackup := int32(900)

				scalingOps := testutil.NewOpsRequestBuilder("scale-ops", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				scalingOps.Spec.TimeoutSeconds = &timeoutScale

				blockingBackup := testutil.NewOpsRequestBuilder("backup-ops", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithPhase(opsv1alpha1.OpsPendingPhase).
					Build()
				blockingBackup.Spec.TimeoutSeconds = &timeoutBackup

				client := testutil.NewFakeClient(scalingOps.DeepCopy(), blockingBackup.DeepCopy())

				return client, []opsv1alpha1.OpsRequest{*scalingOps, *blockingBackup}
			},
			verify: func(t *testing.T, c client.Client) {
				ctx := context.Background()
				namespace := testutil.TestNamespace

				scaling := &opsv1alpha1.OpsRequest{}
				assert.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "scale-ops"}, scaling))
				assert.True(t, scaling.Spec.Cancel)

				backup := &opsv1alpha1.OpsRequest{}
				assert.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "backup-ops"}, backup))
				if assert.NotNil(t, backup.Spec.TimeoutSeconds) {
					assert.Equal(t, int32(1), *backup.Spec.TimeoutSeconds)
				}
			},
		},
		{
			name: "cancel failure should return error",
			prepare: func() (client.Client, []opsv1alpha1.OpsRequest) {
				namespace := testutil.TestNamespace
				clusterName := "cancel-error"
				timeout := int32(600)

				scalingOps := testutil.NewOpsRequestBuilder("scale-error", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.HorizontalScalingType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				scalingOps.Spec.TimeoutSeconds = &timeout

				client := testutil.NewErrorClientBuilder(scalingOps.DeepCopy()).
					WithPatchError(errors.New("patch failed")).
					Build()

				return client, []opsv1alpha1.OpsRequest{*scalingOps}
			},
			expectError:   true,
			errorContains: "cancel blocking scaling operations",
		},
		{
			name: "expire failure should return error",
			prepare: func() (client.Client, []opsv1alpha1.OpsRequest) {
				namespace := testutil.TestNamespace
				clusterName := "expire-error"
				timeout := int32(1200)

				backupOps := testutil.NewOpsRequestBuilder("backup-error", namespace).
					WithClusterName(clusterName).
					WithInstanceLabel(clusterName).
					WithType(opsv1alpha1.BackupType).
					WithPhase(opsv1alpha1.OpsRunningPhase).
					Build()
				backupOps.Spec.TimeoutSeconds = &timeout

				client := testutil.NewErrorClientBuilder(backupOps.DeepCopy()).
					WithPatchError(errors.New("patch failed")).
					Build()

				return client, []opsv1alpha1.OpsRequest{*backupOps}
			},
			expectError:   true,
			errorContains: "expire blocking operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, blocking := tt.prepare()
			err := kbkit.CleanupBlockingOps(ctx, client, blocking)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, client)
			}
		})
	}
}
