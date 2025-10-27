package testutil

import (
	"time"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/furutachiKurea/block-mechanica/internal/index"
	"github.com/furutachiKurea/block-mechanica/internal/model"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	// TestServiceID 默认的测试 service_id
	TestServiceID = "test-service-id"
	// TestNamespace 默认的测试命名空间
	TestNamespace = "default"
)

// Resources 创建资源列表
func Resources(cpu, memory string) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
}

// ClusterBuilder 链式构建 Cluster
type ClusterBuilder struct {
	cluster *kbappsv1.Cluster
}

// NewClusterBuilder 创建 ClusterBuilder
func NewClusterBuilder(name, namespace string) *ClusterBuilder {
	return &ClusterBuilder{
		cluster: &kbappsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    make(map[string]string),
			},
			Spec: kbappsv1.ClusterSpec{},
		},
	}
}

// WithClusterDef 设置 cluster definition
func (cb *ClusterBuilder) WithClusterDef(clusterDef string) *ClusterBuilder {
	cb.cluster.Spec.ClusterDef = clusterDef
	return cb
}

// WithServiceID 设置 service_id 标签
func (cb *ClusterBuilder) WithServiceID(serviceID string) *ClusterBuilder {
	cb.cluster.Labels[index.ServiceIDLabel] = serviceID
	return cb
}

// WithComponent 添加组件
func (cb *ClusterBuilder) WithComponent(name, componentDef string) *ClusterBuilder {
	cb.cluster.Spec.ComponentSpecs = append(cb.cluster.Spec.ComponentSpecs, kbappsv1.ClusterComponentSpec{
		Name:         name,
		ComponentDef: componentDef,
		Replicas:     1,
	})
	return cb
}

// WithComponentServiceVersion 设置组件的 serviceVersion
func (cb *ClusterBuilder) WithComponentServiceVersion(componentName, serviceVersion string) *ClusterBuilder {
	for i := range cb.cluster.Spec.ComponentSpecs {
		if cb.cluster.Spec.ComponentSpecs[i].Name == componentName {
			cb.cluster.Spec.ComponentSpecs[i].ServiceVersion = serviceVersion
			break
		}
	}
	return cb
}

// WithComponentReplicas 设置组件副本数
func (cb *ClusterBuilder) WithComponentReplicas(componentName string, replicas int32) *ClusterBuilder {
	for i := range cb.cluster.Spec.ComponentSpecs {
		if cb.cluster.Spec.ComponentSpecs[i].Name == componentName {
			cb.cluster.Spec.ComponentSpecs[i].Replicas = replicas
			break
		}
	}
	return cb
}

// WithComponentResources 设置组件资源
func (cb *ClusterBuilder) WithComponentResources(componentName string, requests, limits corev1.ResourceList) *ClusterBuilder {
	for i := range cb.cluster.Spec.ComponentSpecs {
		if cb.cluster.Spec.ComponentSpecs[i].Name == componentName {
			cb.cluster.Spec.ComponentSpecs[i].Resources = corev1.ResourceRequirements{
				Requests: requests,
				Limits:   limits,
			}
			break
		}
	}
	return cb
}

// WithComponentVolumeClaimTemplate 为组件添加存储声明模板
func (cb *ClusterBuilder) WithComponentVolumeClaimTemplate(componentName, volumeName, storageClass, storageSize string) *ClusterBuilder {
	for i := range cb.cluster.Spec.ComponentSpecs {
		if cb.cluster.Spec.ComponentSpecs[i].Name == componentName {
			template := kbappsv1.ClusterComponentVolumeClaimTemplate{
				Name: volumeName,
				Spec: kbappsv1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storageSize),
						},
					},
				},
			}
			if storageClass != "" {
				template.Spec.StorageClassName = &storageClass
			}
			cb.cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates = append(
				cb.cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates, template)
			break
		}
	}
	return cb
}

// WithSystemAccountSecret 为第一个 ComponentSpec 配置 SystemAccount
func (cb *ClusterBuilder) WithSystemAccountSecret(accountName, secretName string) *ClusterBuilder {
	if len(cb.cluster.Spec.ComponentSpecs) == 0 {
		return cb
	}

	cb.cluster.Spec.ComponentSpecs[0].SystemAccounts = []kbappsv1.ComponentSystemAccount{
		{
			Name: accountName,
			SecretRef: &kbappsv1.ProvisionSecretRef{
				Name:      secretName,
				Namespace: cb.cluster.Namespace,
			},
		},
	}
	return cb
}

// WithPhase 设置 Cluster 状态
func (cb *ClusterBuilder) WithPhase(phase kbappsv1.ClusterPhase) *ClusterBuilder {
	cb.cluster.Status.Phase = phase
	return cb
}

// WithTerminationPolicy 设置 Cluster 终止策略
func (cb *ClusterBuilder) WithTerminationPolicy(policy kbappsv1.TerminationPolicyType) *ClusterBuilder {
	cb.cluster.Spec.TerminationPolicy = policy
	return cb
}

// WithBackup 设置 Cluster 备份配置
func (cb *ClusterBuilder) WithBackup(backup *kbappsv1.ClusterBackup) *ClusterBuilder {
	cb.cluster.Spec.Backup = backup
	return cb
}

// Build 构建 Cluster 对象
func (cb *ClusterBuilder) Build() *kbappsv1.Cluster {
	return cb.cluster
}

// Cluster 预设模板函数

// NewMySQLCluster 创建一个 MySQL 集群的预设模板
func NewMySQLCluster(name, namespace string) *ClusterBuilder {
	return NewClusterBuilder(name, namespace).
		WithClusterDef("mysql").
		WithComponent("mysql", "mysql-8.0")
}

// NewPostgreSQLCluster 创建一个 PostgreSQL 集群的预设模板
func NewPostgreSQLCluster(name, namespace string) *ClusterBuilder {
	return NewClusterBuilder(name, namespace).
		WithClusterDef("postgresql").
		WithComponent("postgresql", "postgresql-14")
}

// NewRedisCluster 创建一个 Redis 集群的预设模板 (包含 redis + sentinel 两个组件)
func NewRedisCluster(name, namespace string) *ClusterBuilder {
	return NewClusterBuilder(name, namespace).
		WithClusterDef("redis").
		WithComponent("redis", "redis-5-1.0.1").
		WithComponentServiceVersion("redis", "5.0.12").
		WithComponent("redis-sentinel", "redis-sentinel-8-1.0.1").
		WithComponentServiceVersion("redis-sentinel", "8.2.1")
}

// BackupBuilder 链式构建 Backup
type BackupBuilder struct {
	backup *datav1alpha1.Backup
}

// NewBackupBuilder 创建 BackupBuilder
func NewBackupBuilder(name, namespace string) *BackupBuilder {
	return &BackupBuilder{
		backup: &datav1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    make(map[string]string),
			},
			Spec: datav1alpha1.BackupSpec{},
		},
	}
}

// WithBackupPolicyRef 设置备份策略引用
func (bb *BackupBuilder) WithBackupPolicyRef(policyName string) *BackupBuilder {
	bb.backup.Spec.BackupPolicyName = policyName
	return bb
}

// WithBackupMethod 设置备份方法
func (bb *BackupBuilder) WithBackupMethod(method string) *BackupBuilder {
	bb.backup.Spec.BackupMethod = method
	return bb
}

// WithInstanceName 设置实例名称标签
func (bb *BackupBuilder) WithInstanceName(instance string) *BackupBuilder {
	bb.backup.Labels[index.InstanceLabel] = instance
	return bb
}

// WithServiceID 设置 service_id 标签
func (bb *BackupBuilder) WithServiceID(serviceID string) *BackupBuilder {
	bb.backup.Labels[index.ServiceIDLabel] = serviceID
	return bb
}

// WithClusterInstance 设置集群实例标签
func (bb *BackupBuilder) WithClusterInstance(clusterName string) *BackupBuilder {
	bb.backup.Labels[constant.AppInstanceLabelKey] = clusterName
	return bb
}

// WithPhase 设置 Backup 状态
func (bb *BackupBuilder) WithPhase(phase datav1alpha1.BackupPhase) *BackupBuilder {
	bb.backup.Status.Phase = phase
	return bb
}

// WithCreationTime 设置创建时间
func (bb *BackupBuilder) WithCreationTime(t time.Time) *BackupBuilder {
	bb.backup.CreationTimestamp = metav1.Time{Time: t}
	return bb
}

// WithStartTime 设置开始时间
func (bb *BackupBuilder) WithStartTime(t time.Time) *BackupBuilder {
	bb.backup.Status.StartTimestamp = &metav1.Time{Time: t}
	return bb
}

// WithDeletionTimestamp 设置删除时间戳（模拟正在删除的对象）
func (bb *BackupBuilder) WithDeletionTimestamp() *BackupBuilder {
	now := metav1.Time{Time: time.Now()}
	bb.backup.DeletionTimestamp = &now
	// fake client 要求有 DeletionTimestamp 的对象必须有 finalizers
	bb.backup.Finalizers = []string{"test.finalizer/cleanup"}
	return bb
}

// Build 构建 Backup 对象
func (bb *BackupBuilder) Build() *datav1alpha1.Backup {
	return bb.backup
}

// SecretBuilder 链式构建 Secret
type SecretBuilder struct {
	secret *corev1.Secret
}

// NewSecretBuilder 创建 SecretBuilder
func NewSecretBuilder(name, namespace string) *SecretBuilder {
	return &SecretBuilder{
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    make(map[string]string),
			},
			Type: corev1.SecretTypeOpaque,
			Data: make(map[string][]byte),
		},
	}
}

// WithType 设置 Secret 类型
func (sb *SecretBuilder) WithType(secretType corev1.SecretType) *SecretBuilder {
	sb.secret.Type = secretType
	return sb
}

// WithData 设置 Secret data 字段（字节数组）
func (sb *SecretBuilder) WithData(key string, value []byte) *SecretBuilder {
	sb.secret.Data[key] = value
	return sb
}

// WithStringData 设置 Secret data 字段（字符串，自动转换为字节数组）
func (sb *SecretBuilder) WithStringData(key, value string) *SecretBuilder {
	sb.secret.Data[key] = []byte(value)
	return sb
}

// WithLabels 设置 Secret labels
func (sb *SecretBuilder) WithLabels(labels map[string]string) *SecretBuilder {
	for k, v := range labels {
		sb.secret.Labels[k] = v
	}
	return sb
}

// WithServiceID 设置 service_id 标签
func (sb *SecretBuilder) WithServiceID(serviceID string) *SecretBuilder {
	sb.secret.Labels[index.ServiceIDLabel] = serviceID
	return sb
}

// WithImmutable 设置 Secret 是否不可变
func (sb *SecretBuilder) WithImmutable(immutable bool) *SecretBuilder {
	sb.secret.Immutable = ptr.To(immutable)
	return sb
}

// Build 构建 Secret 对象
func (sb *SecretBuilder) Build() *corev1.Secret {
	return sb.secret
}

// OpsRequestBuilder 提供链式 API 来构建 OpsRequest 对象
type OpsRequestBuilder struct {
	opsRequest *opv1alpha1.OpsRequest
}

// NewOpsRequestBuilder 创建一个新的 OpsRequestBuilder
func NewOpsRequestBuilder(name, namespace string) *OpsRequestBuilder {
	return &OpsRequestBuilder{
		opsRequest: &opv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    make(map[string]string),
			},
			Spec: opv1alpha1.OpsRequestSpec{},
		},
	}
}

// WithClusterName 设置目标集群名称
func (orb *OpsRequestBuilder) WithClusterName(clusterName string) *OpsRequestBuilder {
	orb.opsRequest.Spec.ClusterName = clusterName
	return orb
}

// WithType 设置操作类型
func (orb *OpsRequestBuilder) WithType(opsType opv1alpha1.OpsType) *OpsRequestBuilder {
	orb.opsRequest.Spec.Type = opsType
	orb.opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsType)
	return orb
}

// WithCancel 设置取消操作
func (orb *OpsRequestBuilder) WithCancel() *OpsRequestBuilder {
	orb.opsRequest.Spec.Cancel = true
	return orb
}

// WithPhase 设置操作状态
func (orb *OpsRequestBuilder) WithPhase(phase opv1alpha1.OpsPhase) *OpsRequestBuilder {
	orb.opsRequest.Status.Phase = phase
	return orb
}

// WithServiceID 设置 service_id 标签
func (orb *OpsRequestBuilder) WithServiceID(serviceID string) *OpsRequestBuilder {
	orb.opsRequest.Labels[index.ServiceIDLabel] = serviceID
	return orb
}

// WithInstanceLabel 设置 cluster instance 标签
func (orb *OpsRequestBuilder) WithInstanceLabel(clusterName string) *OpsRequestBuilder {
	orb.opsRequest.Labels[index.InstanceLabel] = clusterName
	return orb
}

// WithRestore 设置 Restore 操作的 Spec
func (orb *OpsRequestBuilder) WithRestore(backupName string) *OpsRequestBuilder {
	orb.opsRequest.Spec.Restore = &opv1alpha1.Restore{
		BackupName:                        backupName,
		VolumeRestorePolicy:               "Serial",
		DeferPostReadyUntilClusterRunning: true,
	}
	return orb
}

// WithRestart 设置 Restart 操作的 Spec
func (orb *OpsRequestBuilder) WithRestart(componentName string) *OpsRequestBuilder {
	orb.opsRequest.Spec.RestartList = []opv1alpha1.ComponentOps{
		{
			ComponentName: componentName,
		},
	}
	return orb
}

// Build 构建 OpsRequest 对象
func (orb *OpsRequestBuilder) Build() *opv1alpha1.OpsRequest {
	return orb.opsRequest
}

// StorageClassBuilder 链式构建 StorageClass
type StorageClassBuilder struct {
	storageClass *storagev1.StorageClass
}

// NewStorageClassBuilder 创建 StorageClassBuilder
func NewStorageClassBuilder(name string) *StorageClassBuilder {
	return &StorageClassBuilder{
		storageClass: &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Provisioner: "test-provisioner",
		},
	}
}

// WithAllowVolumeExpansion 设置是否允许存储扩容
func (scb *StorageClassBuilder) WithAllowVolumeExpansion(allow bool) *StorageClassBuilder {
	scb.storageClass.AllowVolumeExpansion = &allow
	return scb
}

// WithProvisioner 设置存储提供者
func (scb *StorageClassBuilder) WithProvisioner(provisioner string) *StorageClassBuilder {
	scb.storageClass.Provisioner = provisioner
	return scb
}

// Build 构建 StorageClass 对象
func (scb *StorageClassBuilder) Build() *storagev1.StorageClass {
	return scb.storageClass
}

// BackupRepoBuilder 链式构建 BackupRepo
type BackupRepoBuilder struct {
	backupRepo *datav1alpha1.BackupRepo
}

// NewBackupRepoBuilder 创建 BackupRepoBuilder
func NewBackupRepoBuilder(name string) *BackupRepoBuilder {
	return &BackupRepoBuilder{
		backupRepo: &datav1alpha1.BackupRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec:   datav1alpha1.BackupRepoSpec{},
			Status: datav1alpha1.BackupRepoStatus{},
		},
	}
}

// WithStorageProvider 设置存储提供者
func (brb *BackupRepoBuilder) WithStorageProvider(provider string) *BackupRepoBuilder {
	brb.backupRepo.Spec.StorageProviderRef = provider
	return brb
}

// WithAccessMethod 设置访问方法
func (brb *BackupRepoBuilder) WithAccessMethod(method datav1alpha1.AccessMethod) *BackupRepoBuilder {
	brb.backupRepo.Spec.AccessMethod = method
	return brb
}

// WithPhase 设置 BackupRepo 状态
func (brb *BackupRepoBuilder) WithPhase(phase datav1alpha1.BackupRepoPhase) *BackupRepoBuilder {
	brb.backupRepo.Status.Phase = phase
	return brb
}

// Build 构建 BackupRepo 对象
func (brb *BackupRepoBuilder) Build() *datav1alpha1.BackupRepo {
	return brb.backupRepo
}

// InstanceSetBuilder 链式构建 InstanceSet
type InstanceSetBuilder struct {
	instanceSet *workloadsv1.InstanceSet
}

// NewInstanceSetBuilder 创建 InstanceSetBuilder
func NewInstanceSetBuilder(name, namespace string) *InstanceSetBuilder {
	return &InstanceSetBuilder{
		instanceSet: &workloadsv1.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Spec:   workloadsv1.InstanceSetSpec{},
			Status: workloadsv1.InstanceSetStatus{},
		},
	}
}

// WithClusterInstance 设置集群实例标签
func (isb *InstanceSetBuilder) WithClusterInstance(clusterName string) *InstanceSetBuilder {
	isb.instanceSet.Labels[constant.AppInstanceLabelKey] = clusterName
	return isb
}

// WithComponentName 设置组件名称标签
func (isb *InstanceSetBuilder) WithComponentName(componentName string) *InstanceSetBuilder {
	isb.instanceSet.Labels["apps.kubeblocks.io/component-name"] = componentName
	return isb
}

// WithComponentAnnotation 设置组件注解
func (isb *InstanceSetBuilder) WithComponentAnnotation(component string) *InstanceSetBuilder {
	isb.instanceSet.Annotations["app.kubernetes.io/component"] = component
	return isb
}

// WithServiceVersionAnnotation 设置服务版本注解
func (isb *InstanceSetBuilder) WithServiceVersionAnnotation(serviceVersion string) *InstanceSetBuilder {
	isb.instanceSet.Annotations["apps.kubeblocks.io/service-version"] = serviceVersion
	return isb
}

// WithReplicas 设置副本数
func (isb *InstanceSetBuilder) WithReplicas(replicas int32) *InstanceSetBuilder {
	isb.instanceSet.Spec.Replicas = &replicas
	return isb
}

// WithInstanceStatus 设置实例状态
func (isb *InstanceSetBuilder) WithInstanceStatus(podNames ...string) *InstanceSetBuilder {
	instanceStatus := make([]workloadsv1.InstanceStatus, len(podNames))
	for i, podName := range podNames {
		instanceStatus[i] = workloadsv1.InstanceStatus{
			PodName: podName,
		}
	}
	isb.instanceSet.Status.InstanceStatus = instanceStatus
	return isb
}

// WithAvailableReplicas 设置可用副本数
func (isb *InstanceSetBuilder) WithAvailableReplicas(available int32) *InstanceSetBuilder {
	isb.instanceSet.Status.AvailableReplicas = available
	return isb
}

// WithReadyReplicas 设置就绪副本数
func (isb *InstanceSetBuilder) WithReadyReplicas(ready int32) *InstanceSetBuilder {
	isb.instanceSet.Status.ReadyReplicas = ready
	return isb
}

// Build 构建 InstanceSet 对象
func (isb *InstanceSetBuilder) Build() *workloadsv1.InstanceSet {
	return isb.instanceSet
}

// NewParameterEntry -
func NewParameterEntry(name string, value any) model.ParameterEntry {
	return model.ParameterEntry{Name: name, Value: value}
}

// ParameterConstraintBuilder 链式构建参数约束
type ParameterConstraintBuilder struct {
	param model.Parameter
}

// NewParameterConstraint 创建参数约束构建器
func NewParameterConstraint(name string) *ParameterConstraintBuilder {
	return &ParameterConstraintBuilder{
		param: model.Parameter{
			ParameterEntry: model.ParameterEntry{Name: name},
		},
	}
}

// WithType 设置参数类型
func (pcb *ParameterConstraintBuilder) WithType(paramType model.ParameterType) *ParameterConstraintBuilder {
	pcb.param.Type = paramType
	return pcb
}

// WithRange 设置参数范围
func (pcb *ParameterConstraintBuilder) WithRange(min, max *float64) *ParameterConstraintBuilder {
	pcb.param.MinValue = min
	pcb.param.MaxValue = max
	return pcb
}

// WithEnumValues 设置枚举值
func (pcb *ParameterConstraintBuilder) WithEnumValues(enums []string) *ParameterConstraintBuilder {
	pcb.param.EnumValues = enums
	return pcb
}

// WithDynamic 设置是否为动态参数
func (pcb *ParameterConstraintBuilder) WithDynamic(dynamic bool) *ParameterConstraintBuilder {
	pcb.param.IsDynamic = dynamic
	return pcb
}

// WithRequired 设置是否为必填参数
func (pcb *ParameterConstraintBuilder) WithRequired(required bool) *ParameterConstraintBuilder {
	pcb.param.IsRequired = required
	return pcb
}

// WithImmutable 设置是否为不可变参数
func (pcb *ParameterConstraintBuilder) WithImmutable(immutable bool) *ParameterConstraintBuilder {
	pcb.param.IsImmutable = immutable
	return pcb
}

// Build 构建参数约束
func (pcb *ParameterConstraintBuilder) Build() model.Parameter {
	return pcb.param
}

// Parameter 工厂函数

// CreateTypicalMySQLParameterEntries 创建典型的 MySQL 参数条目
func CreateTypicalMySQLParameterEntries() []model.ParameterEntry {
	return []model.ParameterEntry{
		NewParameterEntry("max_connections", 100),
		NewParameterEntry("innodb_buffer_pool_size", "128M"),
		NewParameterEntry("sql_mode", "STRICT_TRANS_TABLES"),
		NewParameterEntry("autocommit", "ON"),
		NewParameterEntry("query_cache_size", 0), // 这个参数在约束中不存在，应被过滤
	}
}

// CreateTypicalMySQLParameterConstraints 创建典型的 MySQL 参数约束
func CreateTypicalMySQLParameterConstraints() map[string]model.Parameter {
	return map[string]model.Parameter{
		"max_connections": NewParameterConstraint("max_connections").
			WithType(model.ParameterTypeInteger).
			WithRange(ptr.To(1.0), ptr.To(100000.0)).
			WithDynamic(true).
			Build(),
		"innodb_buffer_pool_size": NewParameterConstraint("innodb_buffer_pool_size").
			WithType(model.ParameterTypeString).
			WithImmutable(true).
			Build(),
		"sql_mode": NewParameterConstraint("sql_mode").
			WithType(model.ParameterTypeString).
			WithEnumValues([]string{`"STRICT_TRANS_TABLES"`, `"NO_ZERO_DATE"`}).
			WithDynamic(true).
			Build(),
		"autocommit": NewParameterConstraint("autocommit").
			WithType(model.ParameterTypeBoolean).
			WithDynamic(true).
			Build(),
	}
}
