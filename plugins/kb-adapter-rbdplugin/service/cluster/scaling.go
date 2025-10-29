package cluster

import (
	"context"
	"errors"
	"fmt"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExpansionCluster 对 Cluster 进行伸缩操作
//
// 使用 opsrequest 将 Cluster 的资源规格进行伸缩，使其变为 model.ExpansionInput 的期望状态
func (s *Service) ExpansionCluster(ctx context.Context, expansion model.ExpansionInput) error {
	log.Debug("Expansion",
		log.String("service_id", expansion.ServiceID),
		log.Any("expansion", expansion),
	)

	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, expansion.ServiceID)
	if err != nil {
		return err
	}

	// 禁止对停止或停止中的 Cluster 进行伸缩
	if cluster.Status.Phase == kbappsv1.StoppedClusterPhase || cluster.Status.Phase == kbappsv1.StoppingClusterPhase {
		return fmt.Errorf("cluster %s/%s is not running", cluster.Namespace, cluster.Name)
	}

	if len(cluster.Spec.ComponentSpecs) == 0 {
		return fmt.Errorf("cluster %s/%s has no componentSpecs", cluster.Namespace, cluster.Name)
	}

	// 解析期望的资源配置
	desiredResources, err := expansion.ParseResources()
	if err != nil {
		return fmt.Errorf("parse desired resources: %w", err)
	}

	// 为各个组件构建当前状态
	// 用于记录所有组件的伸缩操作的上下文
	var components = make(
		map[model.ComponentName]model.ComponentExpansionContext,
		len(cluster.Spec.ComponentSpecs),
	)

	// 为每个组件分别构建完整的伸缩上下文
	for _, spec := range cluster.Spec.ComponentSpecs {
		componentName := spec.Name
		if componentName == "" {
			componentName = cluster.Spec.ClusterDef
		}

		// 构建当前组件的 CPU 和内存状态
		currentCPU := spec.Resources.Limits.Cpu()
		currentMem := spec.Resources.Limits.Memory()

		// 构建当前组件的存储状态
		var (
			hasPVC          = len(spec.VolumeClaimTemplates) > 0
			volumeTplName   string
			currentStorage  resource.Quantity
			storageClassRef *string
		)
		if hasPVC {
			volumeTpl := spec.VolumeClaimTemplates[0]
			volumeTplName = volumeTpl.Name
			if size, ok := volumeTpl.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
				currentStorage = size
			}
			storageClassRef = volumeTpl.Spec.StorageClassName
		}

		// 构建完整的组件伸缩上下文
		components[model.ComponentName(componentName)] = model.ComponentExpansionContext{
			// 水平伸缩
			CurrentReplicas: spec.Replicas,
			DesiredReplicas: expansion.Replicas,
			// 垂直伸缩
			CurrentCPU: *currentCPU,
			CurrentMem: *currentMem,
			DesiredCPU: desiredResources.CPU,
			DesiredMem: desiredResources.Memory,
			// 存储扩容
			HasPVC:          hasPVC,
			VolumeTplName:   volumeTplName,
			CurrentStorage:  currentStorage,
			DesiredStorage:  desiredResources.Storage,
			StorageClassRef: storageClassRef,
		}
	}

	var opsCreated bool

	expansionCtx := model.ExpansionContext{
		Cluster:    cluster,
		Components: components, // 传递所有组件的完整状态
	}

	hCreated, err := s.handleHorizontalScaling(ctx, expansionCtx)
	if err != nil {
		return fmt.Errorf("horizontal scaling: %w", err)
	}
	opsCreated = opsCreated || hCreated

	vCreated, err := s.handleVerticalScaling(ctx, expansionCtx)
	if err != nil {
		return fmt.Errorf("vertical scaling: %w", err)
	}
	opsCreated = opsCreated || vCreated

	sCreated, err := s.handleVolumeExpansion(ctx, expansionCtx)
	if err != nil {
		return fmt.Errorf("volume expansion: %w", err)
	}
	opsCreated = opsCreated || sCreated

	if !opsCreated {
		log.Info("No expansion needed, cluster already matches desired spec",
			log.String("cluster", cluster.Name),
			log.String("service_id", expansion.ServiceID))
	}

	return nil
}

// handleHorizontalScaling 处理水平伸缩（副本数）
func (s *Service) handleHorizontalScaling(ctx context.Context, expansionCtx model.ExpansionContext) (bool, error) {
	// 检查是否有任何组件需要水平伸缩
	var needHScaling bool
	var components []model.ComponentHorizontalScaling

	for componentName, componentCtx := range expansionCtx.Components {
		if componentCtx.DesiredReplicas != componentCtx.CurrentReplicas {
			needHScaling = true
			delta := componentCtx.DesiredReplicas - componentCtx.CurrentReplicas
			components = append(components, model.ComponentHorizontalScaling{
				Name:          string(componentName),
				DeltaReplicas: delta,
			})
		}
	}

	if !needHScaling {
		return false, nil
	}

	opsParams := model.HorizontalScalingOpsParams{
		Cluster:    expansionCtx.Cluster,
		Components: components,
	}

	if err := kbkit.CreateHorizontalScalingOpsRequest(ctx, s.client, opsParams); err != nil {
		if errors.Is(err, kbkit.ErrCreateOpsSkipped) {
			return false, nil
		}
		return false, fmt.Errorf("create horizontal scaling opsrequest: %w", err)
	}

	log.Info("Created horizontal scaling OpsRequest for multiple components",
		log.String("cluster", expansionCtx.Cluster.Name),
		log.Any("components", components))

	return true, nil
}

// handleVerticalScaling 处理垂直伸缩（CPU/内存）
func (s *Service) handleVerticalScaling(ctx context.Context, expansionCtx model.ExpansionContext) (bool, error) {
	// 检查是否有任何组件需要垂直伸缩
	var needVScaling bool
	var components []model.ComponentVerticalScaling

	for componentName, componentCtx := range expansionCtx.Components {
		needCPUScale := componentCtx.CurrentCPU.Cmp(componentCtx.DesiredCPU) != 0
		needMemScale := componentCtx.CurrentMem.Cmp(componentCtx.DesiredMem) != 0

		if needCPUScale || needMemScale {
			needVScaling = true
			components = append(components, model.ComponentVerticalScaling{
				Name:   string(componentName),
				CPU:    componentCtx.DesiredCPU,
				Memory: componentCtx.DesiredMem,
			})
		}
	}

	if !needVScaling {
		return false, nil
	}

	opsParams := model.VerticalScalingOpsParams{
		Cluster:    expansionCtx.Cluster,
		Components: components,
	}

	if err := kbkit.CreateVerticalScalingOpsRequest(ctx, s.client, opsParams); err != nil {
		if errors.Is(err, kbkit.ErrCreateOpsSkipped) {
			return false, nil
		}
		return false, fmt.Errorf("create vertical scaling opsrequest: %w", err)
	}

	log.Info("Created vertical scaling OpsRequest for multiple components",
		log.String("cluster", expansionCtx.Cluster.Name),
		log.Any("components", components))

	return true, nil
}

// handleVolumeExpansion 处理存储扩容
func (s *Service) handleVolumeExpansion(ctx context.Context, expansionCtx model.ExpansionContext) (bool, error) {
	// 检查是否有任何组件需要存储扩容
	var needVolumeExpansion bool
	var components []model.ComponentVolumeExpansion

	for componentName, componentCtx := range expansionCtx.Components {
		// 如果该组件没有 PVC，跳过
		if !componentCtx.HasPVC {
			continue
		}

		switch componentCtx.DesiredStorage.Cmp(componentCtx.CurrentStorage) {
		case 0:
			// 存储大小相同，无需扩容
			continue
		case -1:
			// 存储缩容，记录警告但不处理
			log.Warn("Storage shrinking detected but not supported, skipping component",
				log.String("cluster", expansionCtx.Cluster.Name),
				log.String("component", string(componentName)),
				log.String("volumeTemplate", componentCtx.VolumeTplName),
				log.String("currentStorage", componentCtx.CurrentStorage.String()),
				log.String("desiredStorage", componentCtx.DesiredStorage.String()))
			continue
		case 1:
			// 需要存储扩容，先验证存储类
			canExpand := true
			var skipReason string

			if componentCtx.StorageClassRef == nil || *componentCtx.StorageClassRef == "" {
				canExpand = false
				skipReason = "storageClass not set on volumeClaimTemplate"
			} else {
				var sc storagev1.StorageClass
				if err := s.client.Get(ctx, client.ObjectKey{Name: *componentCtx.StorageClassRef}, &sc); err != nil {
					log.Warn("Failed to get StorageClass, skipping component volume expansion",
						log.String("cluster", expansionCtx.Cluster.Name),
						log.String("component", string(componentName)),
						log.String("volumeTemplate", componentCtx.VolumeTplName),
						log.String("storageClass", *componentCtx.StorageClassRef),
						log.String("error", err.Error()))
					canExpand = false
					skipReason = "failed to get StorageClass"
				} else if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
					canExpand = false
					skipReason = "StorageClass does not allow volume expansion"
				}
			}

			if !canExpand {
				log.Warn("Volume expansion skipped due to configuration constraints",
					log.String("cluster", expansionCtx.Cluster.Name),
					log.String("component", string(componentName)),
					log.String("volumeTemplate", componentCtx.VolumeTplName),
					log.String("reason", skipReason),
					log.String("currentStorage", componentCtx.CurrentStorage.String()),
					log.String("desiredStorage", componentCtx.DesiredStorage.String()))
				continue
			}

			// 添加到需要扩容的组件列表
			needVolumeExpansion = true
			components = append(components, model.ComponentVolumeExpansion{
				Name:                    string(componentName),
				VolumeClaimTemplateName: componentCtx.VolumeTplName,
				Storage:                 componentCtx.DesiredStorage,
			})
		}
	}

	if !needVolumeExpansion {
		return false, nil
	}

	opsParams := model.VolumeExpansionOpsParams{
		Cluster:    expansionCtx.Cluster,
		Components: components,
	}

	if err := kbkit.CreateVolumeExpansionOpsRequest(ctx, s.client, opsParams); err != nil {
		if errors.Is(err, kbkit.ErrCreateOpsSkipped) {
			return false, nil
		}
		return false, fmt.Errorf("create volume expansion opsrequest: %w", err)
	}

	log.Info("Created volume expansion OpsRequest for multiple components",
		log.String("cluster", expansionCtx.Cluster.Name),
		log.Any("components", components))

	return true, nil
}
