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
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GPUResourceConfig GPU 资源配置
type GPUResourceConfig struct {
	// GPUResourceName GPU 资源名称 (例如: nvidia.com/gpu, hygon.com/dcunum, huawei.com/Ascend910A)
	GPUResourceName string
	// GPUMemoryResourceName GPU 显存资源名称 (例如: nvidia.com/gpumem)
	GPUMemoryResourceName string
	// GPUCoreResourceName GPU 核心资源名称 (例如: nvidia.com/gpucores)
	GPUCoreResourceName string
	// RelatedResources 其他相关资源
	RelatedResources []string
}

// GPUDetector GPU 检测器
type GPUDetector struct {
	clientset kubernetes.Interface
}

// NewGPUDetector 创建 GPU 检测器
func NewGPUDetector(clientset kubernetes.Interface) *GPUDetector {
	return &GPUDetector{
		clientset: clientset,
	}
}

// DetectGPUPlugin 动态检测集群中的 GPU 资源
func (d *GPUDetector) DetectGPUPlugin(ctx context.Context) (*GPUResourceConfig, error) {
	// 获取所有节点
	nodes, err := d.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Failed to list nodes: %v", err)
		return nil, err
	}

	// 收集所有可能的 GPU 资源
	gpuResources := make(map[string]int) // 资源名 -> 出现次数
	allResources := make(map[string]bool)

	for _, node := range nodes.Items {
		for resourceName := range node.Status.Allocatable {
			resName := string(resourceName)
			allResources[resName] = true

			// 判断是否是 GPU 相关资源
			if isGPUResource(resName) {
				gpuResources[resName]++
			}
		}
	}

	// 如果没有检测到 GPU 资源，返回 nil
	if len(gpuResources) == 0 {
		logrus.Warn("No GPU resources detected in cluster")
		return &GPUResourceConfig{}, nil
	}

	// 找到最常见的 GPU 主资源
	var primaryGPUResource string
	maxCount := 0
	for resName, count := range gpuResources {
		if count > maxCount {
			maxCount = count
			primaryGPUResource = resName
		}
	}

	config := &GPUResourceConfig{
		GPUResourceName:   primaryGPUResource,
		RelatedResources:  []string{},
	}

	// 推断相关资源（基于相同的前缀）
	prefix := getResourcePrefix(primaryGPUResource) // 例如 "nvidia.com", "hygon.com"
	if prefix != "" {
		for resName := range allResources {
			if strings.HasPrefix(resName, prefix) && resName != primaryGPUResource {
				// 检查是否是显存资源
				if isMemoryResource(resName) {
					config.GPUMemoryResourceName = resName
				} else if isCoreResource(resName) {
					config.GPUCoreResourceName = resName
				} else {
					config.RelatedResources = append(config.RelatedResources, resName)
				}
			}
		}
	}

	logrus.Infof("Detected GPU configuration: primary=%s, memory=%s, cores=%s, related=%v",
		config.GPUResourceName,
		config.GPUMemoryResourceName,
		config.GPUCoreResourceName,
		config.RelatedResources)

	return config, nil
}

// isGPUResource 判断资源名是否是 GPU 相关资源
func isGPUResource(resourceName string) bool {
	// 必须包含 .com/ 格式
	if !strings.Contains(resourceName, ".com/") {
		return false
	}

	lowerName := strings.ToLower(resourceName)

	// GPU 相关关键词
	gpuKeywords := []string{
		"gpu",      // 通用 GPU
		"dcu",      // 海光 DCU
		"npu",      // 华为 NPU
		"ascend",   // 华为昇腾
		"sgpu",     // 墨芯
		"mlu",      // 寒武纪
		"gcu",      // Enflame
		"ipu",      // Graphcore
		"vpu",      // 通用 VPU
	}

	// 检查是否包含 GPU 关键词，但排除显存和核心资源
	for _, keyword := range gpuKeywords {
		if strings.Contains(lowerName, keyword) {
			// 排除显存和核心等子资源
			if !strings.Contains(lowerName, "mem") &&
				!strings.Contains(lowerName, "core") &&
				!strings.Contains(lowerName, "memory") {
				return true
			}
		}
	}

	return false
}

// isMemoryResource 判断是否是显存资源
func isMemoryResource(resourceName string) bool {
	lowerName := strings.ToLower(resourceName)
	return strings.Contains(lowerName, "mem") || strings.Contains(lowerName, "memory")
}

// isCoreResource 判断是否是核心资源
func isCoreResource(resourceName string) bool {
	lowerName := strings.ToLower(resourceName)
	return strings.Contains(lowerName, "core")
}

// getResourcePrefix 获取资源前缀 (例如: "nvidia.com/gpu" -> "nvidia.com")
func getResourcePrefix(resourceName string) string {
	if idx := strings.Index(resourceName, "/"); idx > 0 {
		return resourceName[:idx]
	}
	return ""
}

// GetGPUResourceFromNode 从节点获取 GPU 资源数量
func (config *GPUResourceConfig) GetGPUResourceFromNode(node *corev1.Node) (gpuCount int64, gpuMemory int64, gpuCores int64) {
	// 获取 GPU 数量
	if gpuQuantity, exists := node.Status.Allocatable[corev1.ResourceName(config.GPUResourceName)]; exists {
		gpuCount = gpuQuantity.Value()
	}

	// 获取 GPU 显存
	if config.GPUMemoryResourceName != "" {
		if memQuantity, exists := node.Status.Allocatable[corev1.ResourceName(config.GPUMemoryResourceName)]; exists {
			gpuMemory = memQuantity.Value()
		}
	}

	// 获取 GPU 核心
	if config.GPUCoreResourceName != "" {
		if coreQuantity, exists := node.Status.Allocatable[corev1.ResourceName(config.GPUCoreResourceName)]; exists {
			gpuCores = coreQuantity.Value()
		}
	}

	return
}

// GetGPUResourceFromContainer 从容器获取 GPU 资源请求
func (config *GPUResourceConfig) GetGPUResourceFromContainer(container *corev1.Container) (gpuCount int64, gpuMemory int64, gpuCores int64) {
	// 获取 GPU 数量
	if gpuReq, exists := container.Resources.Requests[corev1.ResourceName(config.GPUResourceName)]; exists {
		gpuCount = gpuReq.Value()
	}

	// 获取 GPU 显存
	if config.GPUMemoryResourceName != "" {
		if memReq, exists := container.Resources.Requests[corev1.ResourceName(config.GPUMemoryResourceName)]; exists {
			gpuMemory = memReq.Value()
		}
	}

	// 获取 GPU 核心
	if config.GPUCoreResourceName != "" {
		if coreReq, exists := container.Resources.Requests[corev1.ResourceName(config.GPUCoreResourceName)]; exists {
			gpuCores = coreReq.Value()
		}
	}

	return
}
