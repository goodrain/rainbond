// Copyright (C) 2014-2018 Goodrain Co., Ltd.
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
	"os"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

// PlatformHealthHandler 平台健康检测处理器接口
type PlatformHealthHandler interface {
	GetPlatformHealth(ctx context.Context) (*model.PlatformHealthResponse, error)
}

// NewPlatformHealthHandler 创建平台健康检测处理器
func NewPlatformHealthHandler() PlatformHealthHandler {
	prometheusURL := os.Getenv("PROMETHEUS_URL")
	if prometheusURL == "" {
		prometheusURL = "http://rbd-monitor:9999"
	}

	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		logrus.Errorf("Failed to create Prometheus client: %v", err)
		return &platformHealthHandler{
			prometheusURL: prometheusURL,
			prometheusAPI: nil,
		}
	}

	return &platformHealthHandler{
		prometheusURL: prometheusURL,
		prometheusAPI: apiv1.NewAPI(client),
	}
}

type platformHealthHandler struct {
	prometheusURL string
	prometheusAPI apiv1.API
}

// GetPlatformHealth 获取平台整体健康状态
func (h *platformHealthHandler) GetPlatformHealth(ctx context.Context) (*model.PlatformHealthResponse, error) {
	var issues []model.HealthIssue

	// 优先检查 Prometheus 监控服务是否可用
	monitorIssue := h.checkPrometheusConnectivity(ctx)
	if monitorIssue != nil {
		// 如果监控服务不可用，直接返回错误，不再检查其他指标
		return &model.PlatformHealthResponse{
			Status:      "unhealthy",
			TotalIssues: 1,
			Issues:      []model.HealthIssue{*monitorIssue},
		}, nil
	}

	// P0 级别检查 - 平台依赖基础设施
	issues = append(issues, h.checkP0Infrastructure(ctx)...)

	// P1 级别检查 - 平台关键资源
	issues = append(issues, h.checkP1Resources(ctx)...)

	// 统计问题数量
	p0Count := 0
	p1Count := 0
	for _, issue := range issues {
		if issue.Priority == "P0" {
			p0Count++
		} else if issue.Priority == "P1" {
			p1Count++
		}
	}

	// 确定整体状态
	status := "healthy"
	if p0Count > 0 {
		status = "unhealthy"
	} else if p1Count > 0 {
		status = "warning"
	}

	return &model.PlatformHealthResponse{
		Status:      status,
		TotalIssues: len(issues),
		Issues:      issues,
	}, nil
}

// checkPrometheusConnectivity 检查 Prometheus 监控服务连接性
func (h *platformHealthHandler) checkPrometheusConnectivity(ctx context.Context) *model.HealthIssue {
	_, err := h.queryPrometheus(ctx, "up")
	if err != nil {
		logrus.Errorf("Failed to connect to Prometheus: %v", err)
		return &model.HealthIssue{
			Priority: "P0",
			Category: "monitor",
			Name:     "监控服务",
			Instance: h.prometheusURL,
			Status:   "down",
			Message:  "监控服务不可用，无法获取平台健康状态",
			Solution: "1. 检查 Prometheus 服务状态：kubectl get pods -n rbd-system | grep rbd-monitor\n2. 查看 Prometheus 日志：kubectl logs -n rbd-system <prometheus-pod>\n3. 验证 Prometheus 配置：kubectl get cm -n rbd-system\n4. 检查网络连接：curl http://rbd-monitor:9999/-/healthy\n5. 重启 Prometheus 服务：kubectl rollout restart deployment rbd-monitor -n rbd-system",
			Metric:   "prometheus_up",
			Value:    0,
		}
	}
	return nil
}

// checkP0Infrastructure P0级别检查：平台依赖基础设施（致命）
func (h *platformHealthHandler) checkP0Infrastructure(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue

	// 1.1 数据库检查
	issues = append(issues, h.checkMySQL(ctx)...)

	// 1.2 Kubernetes 集群检查
	issues = append(issues, h.checkKubernetes(ctx)...)

	// 1.3 镜像仓库检查
	issues = append(issues, h.checkRegistry(ctx)...)

	// 1.4 对象存储检查
	issues = append(issues, h.checkMinIO(ctx)...)

	return issues
}

// checkP1Resources P1级别检查：平台关键资源（严重）
func (h *platformHealthHandler) checkP1Resources(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue

	// 2.1 磁盘空间检查
	issues = append(issues, h.checkDiskSpace(ctx)...)

	// 2.2 计算资源检查
	issues = append(issues, h.checkComputeResources(ctx)...)

	// 2.3 节点状态检查
	issues = append(issues, h.checkNodeStatus(ctx)...)

	return issues
}

// checkMySQL 检查MySQL数据库状态
func (h *platformHealthHandler) checkMySQL(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue
	query := "mysql_up == 0"
	result, err := h.queryPrometheus(ctx, query)
	if err != nil {
		logrus.Errorf("Failed to check MySQL status: %v", err)
		return issues
	}

	for _, sample := range result {
		instance := string(sample.Metric["instance"])
		if instance == "" {
			instance = "unknown"
		}
		host := string(sample.Metric["host"])
		if host == "" {
			host = instance
		}

		issues = append(issues, model.HealthIssue{
			Priority: "P0",
			Category: "database",
			Name:     "MySQL数据库",
			Instance: instance,
			Status:   "down",
			Message:  fmt.Sprintf("数据库 (%s) 无法访问", host),
			Solution: "1. 检查数据库服务是否运行：kubectl get pods -n rbd-system | grep rbd-db\n2. 查看数据库日志：kubectl logs -n rbd-system <pod-name>\n3. 验证数据库连接配置：检查用户名、密码、端口是否正确\n4. 确认网络连通性：ping 数据库地址\n5. 如果是认证失败，重置数据库密码并更新配置",
			Metric:   "mysql_up",
			Value:    0,
		})
	}

	return issues
}

// checkKubernetes 检查Kubernetes集群核心组件
func (h *platformHealthHandler) checkKubernetes(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue

	// API Server
	issues = append(issues, h.checkComponent(ctx,
		"kubernetes_apiserver_up", "P0", "kubernetes",
		"API Server", "K8s API 不可用",
		"1. 检查 API Server Pod 状态：kubectl get pods -n kube-system | grep apiserver\n2. 查看 API Server 日志：kubectl logs -n kube-system <apiserver-pod>\n3. 验证证书是否过期：kubeadm certs check-expiration\n4. 检查端口是否被占用：netstat -tlnp | grep 6443\n5. 重启 API Server：systemctl restart kube-apiserver")...)

	// CoreDNS
	issues = append(issues, h.checkComponent(ctx,
		"coredns_up", "P0", "kubernetes",
		"CoreDNS", "集群内部域名解析异常",
		"1. 检查 CoreDNS Pod 状态：kubectl get pods -n kube-system | grep coredns\n2. 查看 CoreDNS 日志：kubectl logs -n kube-system <coredns-pod>\n3. 检查 CoreDNS 配置：kubectl get cm coredns -n kube-system -o yaml\n4. 重启 CoreDNS：kubectl rollout restart deployment coredns -n kube-system\n5. 验证 DNS 解析：nslookup kubernetes.default.svc.cluster.local")...)

	// Etcd
	issues = append(issues, h.checkComponent(ctx,
		"etcd_up", "P0", "kubernetes",
		"Etcd", "Etcd 故障",
		"1. 检查 Etcd Pod 状态：kubectl get pods -n kube-system | grep etcd\n2. 查看 Etcd 日志：kubectl logs -n kube-system <etcd-pod>\n3. 检查 Etcd 集群健康：etcdctl endpoint health\n4. 验证存储空间：df -h (Etcd 数据目录)\n5. 检查 Etcd 成员状态：etcdctl member list")...)

	// 存储类
	issues = append(issues, h.checkStorageClass(ctx)...)

	return issues
}

// checkStorageClass 检查存储类可用性
func (h *platformHealthHandler) checkStorageClass(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue
	query := "cluster_storage_up == 0"
	result, err := h.queryPrometheus(ctx, query)
	if err != nil {
		logrus.Errorf("Failed to check storage class status: %v", err)
		return issues
	}

	for _, sample := range result {
		storageClass := string(sample.Metric["storage_class"])
		if storageClass == "" {
			storageClass = "unknown"
		}

		issues = append(issues, model.HealthIssue{
			Priority: "P0",
			Category: "kubernetes",
			Name:     "集群存储",
			Instance: storageClass,
			Status:   "down",
			Message:  fmt.Sprintf("(%s) 外部存储类不可用", storageClass),
			Solution: "1. 检查存储类是否存在：kubectl get storageclass\n2. 验证 Provisioner 是否运行：kubectl get pods -A | grep provisioner\n3. 查看 PVC 状态：kubectl get pvc -A\n4. 检查存储后端服务（如 NFS、Ceph）是否正常\n5. 查看 Provisioner 日志排查问题",
			Metric:   "cluster_storage_up",
			Value:    0,
		})
	}

	return issues
}

// checkRegistry 检查容器镜像仓库
func (h *platformHealthHandler) checkRegistry(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue
	query := "registry_up == 0"
	result, err := h.queryPrometheus(ctx, query)
	if err != nil {
		logrus.Errorf("Failed to check registry status: %v", err)
		return issues
	}

	for _, sample := range result {
		instance := string(sample.Metric["instance"])
		if instance == "" {
			instance = "unknown"
		}
		url := string(sample.Metric["url"])

		message := "镜像仓库不可达"
		if url != "" {
			message = fmt.Sprintf("镜像仓库 (%s) 不可达", url)
		}

		issues = append(issues, model.HealthIssue{
			Priority: "P0",
			Category: "registry",
			Name:     "容器镜像仓库",
			Instance: instance,
			Status:   "down",
			Message:  message,
			Solution: "1. 检查镜像仓库服务状态\n2. 查看仓库日志\n3. 验证仓库地址和端口是否正确\n4. 检查证书配置：确认 TLS 证书是否有效\n5. 测试仓库连接：curl -k https://<registry-url>/v2/",
			Metric:   "registry_up",
			Value:    0,
		})
	}

	return issues
}

// checkMinIO 检查MinIO对象存储
func (h *platformHealthHandler) checkMinIO(ctx context.Context) []model.HealthIssue {
	return h.checkComponent(ctx,
		"minio_up", "P0", "storage",
		"MinIO对象存储", "对象存储不可用",
		"1. 检查 MinIO 服务状态：kubectl get pods -n rbd-system | grep minio\n2. 查看 MinIO 日志：kubectl logs -n rbd-system <minio-pod>\n3. 验证访问凭证：检查 AccessKey 和 SecretKey\n4. 检查存储空间：确认磁盘未满\n5. 测试连接：mc admin info <alias>")
}

// checkDiskSpace 检查磁盘空间
func (h *platformHealthHandler) checkDiskSpace(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue
	query := `(1 - node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100 > 70`
	result, err := h.queryPrometheus(ctx, query)
	if err != nil {
		logrus.Warnf("Failed to check disk space: %v", err)
		return issues
	}

	for _, sample := range result {
		node := string(sample.Metric["node"])
		if node == "" {
			node = "unknown"
		}

		usage := float64(sample.Value)

		issues = append(issues, model.HealthIssue{
			Priority: "P1",
			Category: "disk",
			Name:     "节点磁盘",
			Instance: node,
			Status:   "warning",
			Message:  fmt.Sprintf("节点 (%s) 磁盘空间占用超过70%%，磁盘空间不足", node),
			Solution: "1. 清理 Docker 镜像：docker system prune -a\n2. 清理未使用的容器：docker container prune\n3. 清理系统日志：journalctl --vacuum-time=7d\n4. 检查大文件：du -sh /* | sort -hr | head -10\n5. 扩容根分区或添加新磁盘",
			Metric:   "node_filesystem_usage",
			Value:    usage,
		})
	}

	return issues
}

// checkComputeResources 检查计算资源
func (h *platformHealthHandler) checkComputeResources(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue

	// 集群内存检查（可用内存 < 10%）
	memQuery := "(1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)) * 100 > 90"
	memResult, err := h.queryPrometheus(ctx, memQuery)
	if err == nil {
		for _, sample := range memResult {
			usage := float64(sample.Value)

			issues = append(issues, model.HealthIssue{
				Priority: "P1",
				Category: "compute",
				Name:     "集群内存",
				Instance: "cluster",
				Status:   "warning",
				Message:  fmt.Sprintf("集群内存超过 %.2f%%，即将耗尽", usage),
				Solution: "1. 查看内存占用高的 Pod：kubectl top pods -A --sort-by=memory\n2. 检查是否有内存泄漏的应用\n3. 调整 Pod 资源限制：降低不必要的 memory requests\n4. 增加集群节点或扩容现有节点内存\n5. 清理未使用的资源：kubectl delete pods --field-selector=status.phase=Failed -A",
				Metric:   "cluster_memory_usage",
				Value:    usage,
			})
		}
	}

	// 集群CPU检查（可用CPU < 10%）
	cpuQuery := `(1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m]))) * 100 > 90`
	cpuResult, err := h.queryPrometheus(ctx, cpuQuery)
	if err == nil {
		for _, sample := range cpuResult {
			usage := float64(sample.Value)

			issues = append(issues, model.HealthIssue{
				Priority: "P1",
				Category: "compute",
				Name:     "集群CPU",
				Instance: "cluster",
				Status:   "warning",
				Message:  fmt.Sprintf("集群CPU资源超过 %.2f%%，即将耗尽", usage),
				Solution: "1. 查看 CPU 占用高的 Pod：kubectl top pods -A --sort-by=cpu\n2. 检查是否有 CPU 密集型任务在运行\n3. 调整 Pod 资源限制：降低不必要的 cpu requests\n4. 增加集群节点或扩容现有节点 CPU\n5. 优化应用性能，减少 CPU 消耗",
				Metric:   "cluster_cpu_usage",
				Value:    usage,
			})
		}
	}

	return issues
}

// checkNodeStatus 检查节点状态
func (h *platformHealthHandler) checkNodeStatus(ctx context.Context) []model.HealthIssue {
	var issues []model.HealthIssue

	// 节点可用性检查（NotReady状态）
	nodeQuery := `kube_node_status_condition{condition="Ready",status="true"} == 0`
	nodeResult, err := h.queryPrometheus(ctx, nodeQuery)
	if err == nil {
		for _, sample := range nodeResult {
			node := string(sample.Metric["node"])
			if node == "" {
				node = "unknown"
			}

			issues = append(issues, model.HealthIssue{
				Priority: "P1",
				Category: "node",
				Name:     "节点可用性",
				Instance: node,
				Status:   "down",
				Message:  fmt.Sprintf("节点 (%s) 宕机或不可用", node),
				Solution: "1. 检查节点状态：kubectl describe node <node-name>\n2. 查看节点事件：kubectl get events --field-selector involvedObject.name=<node-name>\n3. 登录节点检查 kubelet 服务：systemctl status kubelet\n4. 检查网络连接：ping <node-ip>\n5. 重启 kubelet 服务：systemctl restart kubelet",
				Metric:   "kube_node_status_condition",
				Value:    0,
			})
		}
	}

	// 节点高负载检查（load average > CPU核心数）
	loadQuery := `node_load15 / count(node_cpu_seconds_total{mode="idle"}) by (instance) > 1`
	loadResult, err := h.queryPrometheus(ctx, loadQuery)
	if err == nil {
		for _, sample := range loadResult {
			node := string(sample.Metric["instance"])
			if node == "" {
				node = "unknown"
			}

			load := float64(sample.Value)

			issues = append(issues, model.HealthIssue{
				Priority: "P1",
				Category: "node",
				Name:     "节点负载",
				Instance: node,
				Status:   "warning",
				Message:  fmt.Sprintf("节点 (%s) 负载过高", node),
				Solution: "1. 查看节点上的进程：top 或 htop\n2. 检查高负载进程：ps aux --sort=-%cpu | head\n3. 查看节点上的 Pod：kubectl get pods -A -o wide | grep <node-name>\n4. 迁移部分工作负载到其他节点\n5. 考虑增加节点或升级节点配置",
				Metric:   "node_load15",
				Value:    load,
			})
		}
	}

	return issues
}

// checkComponent 通用组件检查方法
func (h *platformHealthHandler) checkComponent(ctx context.Context, metricName, priority, category, componentName, errorMessage, solution string) []model.HealthIssue {
	var issues []model.HealthIssue
	query := fmt.Sprintf("%s == 0", metricName)
	result, err := h.queryPrometheus(ctx, query)
	if err != nil {
		logrus.Errorf("Failed to check %s status: %v", componentName, err)
		return issues
	}

	for _, sample := range result {
		instance := string(sample.Metric["instance"])
		if instance == "" {
			instance = "unknown"
		}

		issues = append(issues, model.HealthIssue{
			Priority: priority,
			Category: category,
			Name:     componentName,
			Instance: instance,
			Status:   "down",
			Message:  errorMessage,
			Solution: solution,
			Metric:   metricName,
			Value:    0,
		})
	}

	return issues
}

// queryPrometheus 查询Prometheus
func (h *platformHealthHandler) queryPrometheus(ctx context.Context, query string) (prommodel.Vector, error) {
	if h.prometheusAPI == nil {
		return nil, fmt.Errorf("prometheus client not initialized")
	}

	result, warnings, err := h.prometheusAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query prometheus: %w", err)
	}

	if len(warnings) > 0 {
		logrus.Warnf("Prometheus query warnings: %v", warnings)
	}

	// 将结果转换为 Vector 类型
	vector, ok := result.(prommodel.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return vector, nil
}
