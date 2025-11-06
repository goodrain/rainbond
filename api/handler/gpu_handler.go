// Copyright (C) 2014-2024 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GPUHandler GPU 资源管理接口
type GPUHandler interface {
	GetClusterGPUOverview(ctx context.Context) (*model.ClusterGPUOverview, error)
	GetNodeGPUDetail(ctx context.Context, nodeName string) (*model.NodeGPUInfo, error)
	GetAvailableGPUModels(ctx context.Context) (*model.GPUModelsResp, error)
	DetectHAMi(ctx context.Context) (bool, error)
}

// NewGPUHandler 创建 GPU Handler
func NewGPUHandler() GPUHandler {
	clientset := k8s.Default().Clientset

	// 动态检测 GPU 资源
	detector := NewGPUDetector(clientset)
	gpuConfig, err := detector.DetectGPUPlugin(context.Background())
	if err != nil {
		logrus.Warnf("Failed to detect GPU resources: %v", err)
		gpuConfig = &GPUResourceConfig{} // 空配置
	}

	if gpuConfig.GPUResourceName != "" {
		logrus.Infof("Detected GPU configuration: resource=%s, memory=%s, cores=%s",
			gpuConfig.GPUResourceName,
			gpuConfig.GPUMemoryResourceName,
			gpuConfig.GPUCoreResourceName)
	} else {
		logrus.Warn("No GPU resources detected in cluster")
	}

	return &gpuHandler{
		clientset:     clientset,
		prometheusCli: prom.Default().PrometheusCli,
		gpuConfig:     gpuConfig,
	}
}

type gpuHandler struct {
	clientset     kubernetes.Interface
	prometheusCli prometheus.Interface
	gpuConfig     *GPUResourceConfig
}

// DetectHAMi 检测是否安装 HAMi
func (g *gpuHandler) DetectHAMi(ctx context.Context) (bool, error) {
	// 方法1: 检查 kube-system 命名空间下是否有 hami-device-plugin DaemonSet
	_, err := g.clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "hami-device-plugin", metav1.GetOptions{})
	if err == nil {
		logrus.Info("Detected HAMi device plugin DaemonSet")
		return true, nil
	}

	// 方法2: 检查节点是否有 HAMi 标签
	nodes, err := g.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list nodes: %v", err)
		return false, err
	}

	for _, node := range nodes.Items {
		// 检查 HAMi 标签
		if _, exists := node.Labels["hami.io/node-gpu-enable"]; exists {
			logrus.Infof("Node %s has HAMi label", node.Name)
			return true, nil
		}

		// 检查是否有 GPU 资源（任何 *.com/gpu 格式）
		if g.gpuConfig != nil && g.gpuConfig.GPUResourceName != "" {
			if _, exists := node.Status.Allocatable[corev1.ResourceName(g.gpuConfig.GPUResourceName)]; exists {
				logrus.Infof("Node %s has GPU resource: %s", node.Name, g.gpuConfig.GPUResourceName)
				return true, nil
			}
		}
	}

	logrus.Warn("HAMi not detected in cluster")
	return false, nil
}

// GetClusterGPUOverview 获取集群 GPU 总览
func (g *gpuHandler) GetClusterGPUOverview(ctx context.Context) (*model.ClusterGPUOverview, error) {
	overview := &model.ClusterGPUOverview{
		Nodes: []model.NodeGPUInfo{},
	}

	// 获取所有节点
	nodes, err := g.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list nodes: %v", err)
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// 遍历节点，获取 GPU 信息
	for _, node := range nodes.Items {
		gpuResource, err := g.getNodeGPUResource(ctx, &node)
		if err != nil {
			logrus.Warnf("Failed to get GPU resource for node %s: %v", node.Name, err)
			continue
		}

		// 如果节点没有 GPU，跳过
		if gpuResource.TotalGPUCount == 0 {
			continue
		}

		// 获取节点内网 IP
		internalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		// 获取节点状态
		status := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				status = "Ready"
				break
			}
		}

		nodeInfo := model.NodeGPUInfo{
			NodeName:    node.Name,
			Status:      status,
			InternalIP:  internalIP,
			GPUResource: *gpuResource,
		}

		overview.Nodes = append(overview.Nodes, nodeInfo)

		// 累加到集群总数
		overview.GPUNodeCount++
		overview.TotalGPUCount += gpuResource.TotalGPUCount
		overview.UsedGPUCount += gpuResource.UsedGPUCount
		overview.TotalMemory += gpuResource.TotalMemory
		overview.UsedMemory += gpuResource.UsedMemory
	}

	// 计算集群整体使用率
	if overview.TotalGPUCount > 0 {
		overview.UtilizationRate = float64(overview.UsedGPUCount) / float64(overview.TotalGPUCount) * 100
	}

	return overview, nil
}

// GetNodeGPUDetail 获取节点 GPU 详情
func (g *gpuHandler) GetNodeGPUDetail(ctx context.Context, nodeName string) (*model.NodeGPUInfo, error) {
	// 获取节点信息
	node, err := g.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get node %s: %v", nodeName, err)
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// 获取 GPU 资源
	gpuResource, err := g.getNodeGPUResource(ctx, node)
	if err != nil {
		logrus.Errorf("Failed to get GPU resource for node %s: %v", nodeName, err)
		return nil, fmt.Errorf("failed to get GPU resource: %w", err)
	}

	// 获取节点内网 IP
	internalIP := ""
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	// 获取节点状态
	status := "NotReady"
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			status = "Ready"
			break
		}
	}

	return &model.NodeGPUInfo{
		NodeName:    nodeName,
		Status:      status,
		InternalIP:  internalIP,
		GPUResource: *gpuResource,
	}, nil
}

// getNodeGPUResource 获取节点的 GPU 资源信息
func (g *gpuHandler) getNodeGPUResource(ctx context.Context, node *corev1.Node) (*model.GPUResource, error) {
	gpuResource := &model.GPUResource{
		GPUCards: []model.GPUCard{},
	}

	// 使用配置的资源名称获取 GPU 信息
	gpuCount, gpuMemory, _ := g.gpuConfig.GetGPUResourceFromNode(node)
	gpuResource.TotalGPUCount = int(gpuCount)
	gpuResource.TotalMemory = gpuMemory

	// 如果没有 GPU，直接返回
	if gpuResource.TotalGPUCount == 0 {
		return gpuResource, nil
	}

	// 从 Prometheus 获取使用率数据
	gpuResource.UtilizationRate = g.getGPUUtilizationFromPrometheus(node.Name)

	// 获取每张卡的详细信息
	gpuCards, err := g.getGPUCardsFromNodeExporter(node.Name, gpuResource.TotalGPUCount)
	if err != nil {
		logrus.Warnf("Failed to get GPU cards from node exporter for node %s: %v", node.Name, err)
	} else {
		gpuResource.GPUCards = gpuCards
	}

	// 计算已使用的 GPU 数量和显存
	usedGPU, usedMemory := g.calculateUsedResources(ctx, node.Name)
	gpuResource.UsedGPUCount = usedGPU
	gpuResource.UsedMemory = usedMemory

	return gpuResource, nil
}

// getGPUUtilizationFromPrometheus 从 Prometheus 获取 GPU 使用率
func (g *gpuHandler) getGPUUtilizationFromPrometheus(nodeName string) float64 {
	if g.prometheusCli == nil {
		return 0.0
	}

	// PromQL 查询平均 GPU 使用率
	query := fmt.Sprintf(`avg(DCGM_FI_DEV_GPU_UTIL{kubernetes_node="%s"})`, nodeName)

	metric := g.prometheusCli.GetMetric(query, time.Now())
	if metric.Error == "" && len(metric.MetricValues) > 0 {
		if metric.MetricValues[0].Sample != nil {
			return metric.MetricValues[0].Sample.Value()
		}
	}

	return 0.0
}

// getGPUCardsFromNodeExporter 从 Node Exporter 获取 GPU 卡详情
func (g *gpuHandler) getGPUCardsFromNodeExporter(nodeName string, gpuCount int) ([]model.GPUCard, error) {
	if g.prometheusCli == nil {
		return nil, fmt.Errorf("prometheus client is nil")
	}

	cards := []model.GPUCard{}

	// 遍历每张 GPU 卡
	for i := 0; i < gpuCount; i++ {
		card := model.GPUCard{
			ID: i,
		}

		// 查询显存总量
		memQuery := fmt.Sprintf(`DCGM_FI_DEV_FB_TOTAL{kubernetes_node="%s", gpu="%d"}`, nodeName, i)
		memMetric := g.prometheusCli.GetMetric(memQuery, time.Now())
		if memMetric.Error == "" && len(memMetric.MetricValues) > 0 {
			if memMetric.MetricValues[0].Sample != nil {
				card.Memory = int64(memMetric.MetricValues[0].Sample.Value())
			}
		}

		// 查询已使用显存
		usedMemQuery := fmt.Sprintf(`DCGM_FI_DEV_FB_USED{kubernetes_node="%s", gpu="%d"}`, nodeName, i)
		usedMemMetric := g.prometheusCli.GetMetric(usedMemQuery, time.Now())
		if usedMemMetric.Error == "" && len(usedMemMetric.MetricValues) > 0 {
			if usedMemMetric.MetricValues[0].Sample != nil {
				card.UsedMemory = int64(usedMemMetric.MetricValues[0].Sample.Value())
			}
		}

		// 查询使用率
		utilQuery := fmt.Sprintf(`DCGM_FI_DEV_GPU_UTIL{kubernetes_node="%s", gpu="%d"}`, nodeName, i)
		utilMetric := g.prometheusCli.GetMetric(utilQuery, time.Now())
		if utilMetric.Error == "" && len(utilMetric.MetricValues) > 0 {
			if utilMetric.MetricValues[0].Sample != nil {
				card.UtilizationRate = utilMetric.MetricValues[0].Sample.Value()
			}
		}

		// 查询温度
		tempQuery := fmt.Sprintf(`DCGM_FI_DEV_GPU_TEMP{kubernetes_node="%s", gpu="%d"}`, nodeName, i)
		tempMetric := g.prometheusCli.GetMetric(tempQuery, time.Now())
		if tempMetric.Error == "" && len(tempMetric.MetricValues) > 0 {
			if tempMetric.MetricValues[0].Sample != nil {
				card.Temperature = int(tempMetric.MetricValues[0].Sample.Value())
			}
		}

		// 设置默认型号（可以从节点标签或其他地方获取）
		card.Model = g.getGPUModel(nodeName, i)
		card.UUID = fmt.Sprintf("GPU-%s-%d", nodeName, i)

		cards = append(cards, card)
	}

	return cards, nil
}

// getGPUModel 获取 GPU 型号
func (g *gpuHandler) getGPUModel(nodeName string, gpuID int) string {
	// 可以从 Prometheus 或节点标签中获取 GPU 型号
	// 这里先返回默认值
	return "NVIDIA GPU"
}

// calculateUsedResources 计算已使用的 GPU 资源
func (g *gpuHandler) calculateUsedResources(ctx context.Context, nodeName string) (int, int64) {
	// 获取节点上的所有 Pod
	pods, err := g.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		logrus.Errorf("Failed to list pods on node %s: %v", nodeName, err)
		return 0, 0
	}

	usedGPU := 0
	usedMem := int64(0)

	// 遍历所有 Pod，累加 GPU 资源请求
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		for _, container := range pod.Spec.Containers {
			// 使用配置的资源名称获取 GPU 资源
			gpuCount, gpuMemory, _ := g.gpuConfig.GetGPUResourceFromContainer(&container)
			usedGPU += int(gpuCount)
			usedMem += gpuMemory
		}
	}

	return usedGPU, usedMem
}

// GetAvailableGPUModels 获取集群中可用的 GPU 型号列表
func (g *gpuHandler) GetAvailableGPUModels(ctx context.Context) (*model.GPUModelsResp, error) {
	modelsMap := make(map[string]bool)

	// 获取所有节点
	nodes, err := g.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list nodes: %v", err)
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// 动态查找 GPU 型号标签
	// 常见的 GPU 型号标签格式：<vendor>.com/gpu.product, <vendor>.com/gpu.model 等
	if g.gpuConfig != nil && g.gpuConfig.GPUResourceName != "" {
		// 根据 GPU 资源名称推断可能的标签键
		prefix := ""
		if idx := strings.Index(g.gpuConfig.GPUResourceName, "/"); idx > 0 {
			prefix = g.gpuConfig.GPUResourceName[:idx] // 例如: "nvidia.com"
		}

		possibleLabelKeys := []string{
			prefix + "/gpu.product",
			prefix + "/gpu.model",
			prefix + "/model",
			prefix + "/product",
			"gpu-model",
		}

		// 遍历节点，收集 GPU 型号
		for _, node := range nodes.Items {
			// 尝试各种可能的标签键
			for _, labelKey := range possibleLabelKeys {
				if model, exists := node.Labels[labelKey]; exists && model != "" {
					modelsMap[model] = true
				}
			}

			// 尝试从节点注解中获取
			if model, exists := node.Annotations["gpu-model"]; exists && model != "" {
				modelsMap[model] = true
			}
		}
	}

	// 如果没有找到任何型号，返回通用型号
	if len(modelsMap) == 0 {
		logrus.Warn("No GPU models found in node labels, returning generic models")
		return &model.GPUModelsResp{
			Models: []string{
				"Generic GPU",
			},
		}, nil
	}

	// 转换为切片
	models := make([]string, 0, len(modelsMap))
	for model := range modelsMap {
		models = append(models, model)
	}

	return &model.GPUModelsResp{
		Models: models,
	}, nil
}
