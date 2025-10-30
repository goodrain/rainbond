package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestListAvailableBackupRepos(t *testing.T) {
	tests := []struct {
		name        string
		clientSetup func() client.Client
		setup       func(client.Client) error
		want        []*model.BackupRepo
		expectErr   bool
		errContains string
	}{
		{
			name:        "multiple_repos_only_ready_returned",
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				return testutil.CreateObjects(ctx, c, []client.Object{
					testutil.NewBackupRepoBuilder("repo1").
						WithStorageProvider("s3").
						WithAccessMethod(datav1alpha1.AccessMethodMount).
						WithPhase(datav1alpha1.BackupRepoReady).
						Build(),
					testutil.NewBackupRepoBuilder("repo2").
						WithStorageProvider("s3").
						WithAccessMethod(datav1alpha1.AccessMethodTool).
						WithPhase(datav1alpha1.BackupRepoFailed).
						Build(),
					testutil.NewBackupRepoBuilder("repo3").
						WithStorageProvider("minio").
						WithAccessMethod(datav1alpha1.AccessMethodTool).
						WithPhase(datav1alpha1.BackupRepoPreChecking).
						Build(),
				})
			},
			want: []*model.BackupRepo{
				{Name: "repo1", Type: "s3", AccessMethod: datav1alpha1.AccessMethodMount, Phase: datav1alpha1.BackupRepoReady},
			},
			expectErr: false,
		},
		{
			name:        "empty_list",
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup:       func(c client.Client) error { return nil },
			want:        []*model.BackupRepo{},
			expectErr:   false,
		},
		{
			name: "list_error",
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().WithListError(errors.New("list failed")).Build()
			},
			setup:       func(c client.Client) error { return nil },
			want:        nil,
			expectErr:   true,
			errContains: "list failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientSetup()
			require.NoError(t, tt.setup(c))

			svc := NewService(c)
			got, err := svc.ListAvailableBackupRepos(context.Background())

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestReScheduleBackup(t *testing.T) {
	tests := []struct {
		name        string
		request     model.BackupScheduleInput
		clientSetup func() client.Client
		setup       func(client.Client) error
		expectErr   bool
		errContains string
		validate    func(t *testing.T, client client.Client)
	}{
		{
			name: "enable_backup_for_cluster_without_backup_config",
			request: model.BackupScheduleInput{
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "backup-repo-1",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: "7d",
				},
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
			validate: func(t *testing.T, client client.Client) {
				cluster := getClusterByServiceID(t, client, testutil.TestServiceID)
				require.NotNil(t, cluster.Spec.Backup)
				backup := cluster.Spec.Backup
				require.NotNil(t, backup.Enabled)
				assert.True(t, *backup.Enabled)
				assert.Equal(t, "backup-repo-1", backup.RepoName)
				assert.Equal(t, "0 2 * * *", backup.CronExpression)
				assert.Equal(t, "7d", string(backup.RetentionPeriod))
				assert.Equal(t, "xtrabackup", backup.Method)
			},
		},
		{
			name: "update_existing_backup_configuration",
			request: model.BackupScheduleInput{
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "backup-repo-2",
					Schedule: model.BackupSchedule{
						Frequency: model.Weekly,
						Hour:      3,
						Minute:    30,
						DayOfWeek: 1,
					},
					RetentionPeriod: "30d",
				},
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				enabled := true
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					WithBackup(&appsv1.ClusterBackup{
						RepoName:        "backup-repo-1",
						Enabled:         &enabled,
						CronExpression:  "0 2 * * *",
						RetentionPeriod: "7d",
					}).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
			validate: func(t *testing.T, client client.Client) {
				cluster := getClusterByServiceID(t, client, testutil.TestServiceID)
				require.NotNil(t, cluster.Spec.Backup)
				backup := cluster.Spec.Backup
				require.Equal(t, "backup-repo-2", backup.RepoName)
				assert.Equal(t, "30 3 * * 1", backup.CronExpression)
				assert.Equal(t, "30d", string(backup.RetentionPeriod))
				require.NotNil(t, backup.Enabled)
				assert.True(t, *backup.Enabled)
				assert.Equal(t, "xtrabackup", backup.Method)
			},
		},
		{
			name: "disable_backup_by_setting_empty_repo",
			request: model.BackupScheduleInput{
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "",
				},
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				enabled := true
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					WithBackup(&appsv1.ClusterBackup{
						RepoName:        "backup-repo-1",
						Enabled:         &enabled,
						CronExpression:  "0 2 * * *",
						RetentionPeriod: "7d",
					}).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
			validate: func(t *testing.T, client client.Client) {
				cluster := getClusterByServiceID(t, client, testutil.TestServiceID)
				require.Nil(t, cluster.Spec.Backup)
			},
		},
		{
			name: "cluster_not_found_error",
			request: model.BackupScheduleInput{
				RBDService: model.RBDService{ServiceID: "non-existent-service-id"},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup:       func(c client.Client) error { return nil },
			expectErr:   true,
			errContains: "get cluster by service_id: resource not found",
		},
		{
			name: "multiple_clusters_found_error",
			request: model.BackupScheduleInput{
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster1 := testutil.NewMySQLCluster("test-cluster-1", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				cluster2 := testutil.NewMySQLCluster("test-cluster-2", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster1, cluster2})
			},
			expectErr:   true,
			errContains: "get cluster by service_id: multiple resources found",
		},
		{
			name: "enable_backup_from_disabled_state",
			request: model.BackupScheduleInput{
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "backup-repo-new",
					Schedule: model.BackupSchedule{
						Frequency: model.Weekly,
						Hour:      10,
						Minute:    30,
						DayOfWeek: 5,
					},
					RetentionPeriod: "14d",
				},
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				enabled := false
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					WithBackup(&appsv1.ClusterBackup{
						RepoName:        "backup-repo-old",
						Enabled:         &enabled,
						CronExpression:  "0 0 * * 0",
						RetentionPeriod: "1d",
					}).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
			validate: func(t *testing.T, client client.Client) {
				cluster := getClusterByServiceID(t, client, testutil.TestServiceID)
				require.NotNil(t, cluster.Spec.Backup)
				backup := cluster.Spec.Backup
				require.NotNil(t, backup.Enabled)
				assert.True(t, *backup.Enabled)
				assert.Equal(t, "backup-repo-new", backup.RepoName)
				assert.Equal(t, "30 10 * * 5", backup.CronExpression)
				assert.Equal(t, "14d", string(backup.RetentionPeriod))
				assert.Equal(t, "xtrabackup", backup.Method)
			},
		},
		{
			name: "client_error_during_list_operation",
			request: model.BackupScheduleInput{
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().WithListError(errors.New("list failed")).Build()
			},
			setup:       func(c client.Client) error { return nil },
			expectErr:   true,
			errContains: "list failed",
		},
		{
			name: "no_change_when_configuration_matches",
			request: model.BackupScheduleInput{
				ClusterBackup: model.ClusterBackup{
					BackupRepo: "backup-repo-1",
					Schedule: model.BackupSchedule{
						Frequency: model.Daily,
						Hour:      2,
						Minute:    0,
					},
					RetentionPeriod: "7d",
				},
				RBDService: model.RBDService{ServiceID: testutil.TestServiceID},
			},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				enabled := true
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					WithBackup(&appsv1.ClusterBackup{
						RepoName:        "backup-repo-1",
						Enabled:         &enabled,
						CronExpression:  "0 2 * * *",
						RetentionPeriod: "7d",
					}).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			expectErr: false,
			validate: func(t *testing.T, client client.Client) {
				cluster := getClusterByServiceID(t, client, testutil.TestServiceID)
				require.NotNil(t, cluster.Spec.Backup)
				backup := cluster.Spec.Backup
				require.NotNil(t, backup.Enabled)
				assert.True(t, *backup.Enabled)
				assert.Equal(t, "backup-repo-1", backup.RepoName)
				assert.Equal(t, "0 2 * * *", backup.CronExpression)
				assert.Equal(t, "7d", string(backup.RetentionPeriod))
				assert.Equal(t, "xtrabackup", backup.Method)
			},
		},
	}

	for _, tc := range tests {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			c := testCase.clientSetup()
			require.NoError(t, testCase.setup(c))

			svc := NewService(c)
			err := svc.ReScheduleBackup(context.Background(), testCase.request)

			if testCase.expectErr {
				require.Error(t, err)
				if testCase.errContains != "" {
					assert.Contains(t, err.Error(), testCase.errContains)
				}
			} else {
				require.NoError(t, err)
				if testCase.validate != nil {
					testCase.validate(t, c)
				}
			}
		})
	}
}

// getClusterByServiceID helper function
func getClusterByServiceID(t *testing.T, c client.Client, serviceID string) *appsv1.Cluster {
	t.Helper()

	var clusters appsv1.ClusterList
	require.NoError(t, c.List(context.Background(), &clusters, client.MatchingLabels{index.ServiceIDLabel: serviceID}))
	require.Len(t, clusters.Items, 1)
	cluster := clusters.Items[0]
	return &cluster
}

func TestListBackups(t *testing.T) {
	tests := []struct {
		name        string
		req         model.RBDService
		clientSetup func() client.Client
		setup       func(client.Client) error
		want        []model.BackupItem
		expectErr   bool
	}{
		{
			name:        "successful_backup_list",
			req:         model.RBDService{ServiceID: testutil.TestServiceID},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()

				backup1 := testutil.NewBackupBuilder("backup-1", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseCompleted).
					WithCreationTime(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)).
					WithStartTime(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)).
					Build()

				backup2 := testutil.NewBackupBuilder("backup-2", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseFailed).
					WithCreationTime(time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)).
					WithStartTime(time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)).
					Build()

				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backup1, backup2})
			},
			want: []model.BackupItem{
				{
					Name:   "backup-2",
					Status: datav1alpha1.BackupPhaseFailed,
					Time:   time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
				},
				{
					Name:   "backup-1",
					Status: datav1alpha1.BackupPhaseCompleted,
					Time:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				},
			},
			expectErr: false,
		},
		{
			name:        "no_cluster_found",
			req:         model.RBDService{ServiceID: "non-existent-service-id"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup:       func(c client.Client) error { return nil },
			want:        nil,
			expectErr:   true,
		},
		{
			name:        "multiple_clusters_found",
			req:         model.RBDService{ServiceID: testutil.TestServiceID},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster1 := testutil.NewMySQLCluster("cluster-1", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				cluster2 := testutil.NewMySQLCluster("cluster-2", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster1, cluster2})
			},
			want:      nil,
			expectErr: true,
		},
		{
			name:        "no_backups_found",
			req:         model.RBDService{ServiceID: testutil.TestServiceID},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			want:      nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientSetup()
			require.NoError(t, tt.setup(c))

			svc := NewService(c)
			got, err := svc.ListBackups(context.Background(), model.BackupListQuery{RBDService: tt.req})

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, got.Items)
				} else {
					assert.Equal(t, tt.want, got.Items)
				}
			}
		})
	}
}

func TestDeleteBackups(t *testing.T) {
	tests := []struct {
		name        string
		serviceID   string
		backupNames []string
		clientSetup func() client.Client
		setup       func(client.Client) error
		want        []string
		expectErr   bool
		errContains string
	}{
		{
			name:        "delete_single_backup_success",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-1"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				backup := testutil.NewBackupBuilder("backup-1", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseCompleted).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backup})
			},
			want:      []string{"backup-1"},
			expectErr: false,
		},
		{
			name:        "delete_multiple_backups_success",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-1", "backup-2"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				backup1 := testutil.NewBackupBuilder("backup-1", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseCompleted).
					Build()
				backup2 := testutil.NewBackupBuilder("backup-2", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseFailed).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backup1, backup2})
			},
			want:      []string{"backup-1", "backup-2"},
			expectErr: false,
		},
		{
			name:        "delete_non_existent_backup",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"non-existent-backup"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			want:      []string{},
			expectErr: false,
		},
		{
			name:        "delete_running_backup_rejected",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-running"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				backup := testutil.NewBackupBuilder("backup-running", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseRunning).
					WithStartTime(time.Now().Add(-10 * time.Minute)).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backup})
			},
			want:      []string{},
			expectErr: false,
		},
		{
			name:        "delete_backup_being_deleted",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-deleting"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				backup := testutil.NewBackupBuilder("backup-deleting", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseDeleting).
					WithDeletionTimestamp().
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backup})
			},
			want:      []string{"backup-deleting"},
			expectErr: false,
		},
		{
			name:        "mixed_scenario_completed_running_nonexistent",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-completed", "backup-running", "non-existent"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				backupCompleted := testutil.NewBackupBuilder("backup-completed", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseCompleted).
					Build()
				backupRunning := testutil.NewBackupBuilder("backup-running", testutil.TestNamespace).
					WithClusterInstance("test-cluster").
					WithPhase(datav1alpha1.BackupPhaseRunning).
					WithStartTime(time.Now().Add(-10 * time.Minute)).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster, backupCompleted, backupRunning})
			},
			want:      []string{"backup-completed"},
			expectErr: false,
		},
		{
			name:        "cluster_not_found_error",
			serviceID:   "non-existent-service-id",
			backupNames: []string{"backup-1"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup:       func(c client.Client) error { return nil },
			want:        nil,
			expectErr:   true,
			errContains: "get cluster by service_id",
		},
		{
			name:        "multiple_clusters_error",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-1"},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster1 := testutil.NewMySQLCluster("cluster-1", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				cluster2 := testutil.NewMySQLCluster("cluster-2", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster1, cluster2})
			},
			want:        nil,
			expectErr:   true,
			errContains: "multiple resources found",
		},
		{
			name:        "client_list_error",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{"backup-1"},
			clientSetup: func() client.Client {
				return testutil.NewErrorClientBuilder().WithListError(errors.New("list failed")).Build()
			},
			setup:       func(c client.Client) error { return nil },
			want:        nil,
			expectErr:   true,
			errContains: "list failed",
		},
		{
			name:        "empty_backup_names_list",
			serviceID:   testutil.TestServiceID,
			backupNames: []string{},
			clientSetup: func() client.Client { return testutil.NewFakeClient() },
			setup: func(c client.Client) error {
				ctx := context.Background()
				cluster := testutil.NewMySQLCluster("test-cluster", testutil.TestNamespace).
					WithServiceID(testutil.TestServiceID).
					Build()
				return testutil.CreateObjects(ctx, c, []client.Object{cluster})
			},
			want:      []string{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientSetup()
			require.NoError(t, tt.setup(c))

			svc := NewService(c)
			rbd := model.RBDService{ServiceID: tt.serviceID}

			got, err := svc.DeleteBackups(context.Background(), rbd, tt.backupNames)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.want, got)
			}
		})
	}
}

func TestCanDeleteBackup(t *testing.T) {
	tests := []struct {
		name          string
		backup        *datav1alpha1.Backup
		wantCanDelete bool
		wantReason    string
	}{
		{
			name: "can_delete_completed_backup",
			backup: testutil.NewBackupBuilder("completed-backup", testutil.TestNamespace).
				WithClusterInstance("test-cluster").
				WithPhase(datav1alpha1.BackupPhaseCompleted).
				Build(),
			wantCanDelete: true,
			wantReason:    "",
		},
		{
			name: "can_delete_failed_backup",
			backup: testutil.NewBackupBuilder("failed-backup", testutil.TestNamespace).
				WithClusterInstance("test-cluster").
				WithPhase(datav1alpha1.BackupPhaseFailed).
				Build(),
			wantCanDelete: true,
			wantReason:    "",
		},
		{
			name: "cannot_delete_running_backup",
			backup: testutil.NewBackupBuilder("running-backup", testutil.TestNamespace).
				WithClusterInstance("test-cluster").
				WithPhase(datav1alpha1.BackupPhaseRunning).
				WithStartTime(time.Now().Add(-10 * time.Minute)).
				Build(),
			wantCanDelete: false,
			wantReason:    ReasonBackupRunning,
		},
		{
			name: "can_delete_deleting_backup",
			backup: testutil.NewBackupBuilder("deleting-backup", testutil.TestNamespace).
				WithClusterInstance("test-cluster").
				WithPhase(datav1alpha1.BackupPhaseDeleting).
				WithDeletionTimestamp().
				Build(),
			wantCanDelete: true,
			wantReason:    "",
		},
		{
			name: "can_delete_new_backup",
			backup: testutil.NewBackupBuilder("new-backup", testutil.TestNamespace).
				WithClusterInstance("test-cluster").
				WithPhase(datav1alpha1.BackupPhaseNew).
				Build(),
			wantCanDelete: true,
			wantReason:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(nil)
			canDelete, reason := svc.canDeleteBackup(tt.backup)

			assert.Equal(t, tt.wantCanDelete, canDelete)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}
