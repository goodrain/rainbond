# Containerd 私有镜像仓库 TLS 配置指南

## 概述

针对 containerd 运行时拉取私有 HTTPS 镜像仓库时的 x509 证书验证问题，我们提供了灵活的 TLS 配置解决方案。

## 配置方式

### 1. 环境变量配置

#### 全局跳过 TLS 验证（不推荐用于生产环境）
```bash
export REGISTRY_INSECURE_SKIP_VERIFY=true
```

#### 特定域名跳过 TLS 验证
```bash
# 对于 registry.company.com
export REGISTRY_INSECURE_SKIP_VERIFY_REGISTRY_COMPANY_COM=true

# 对于 harbor.internal
export REGISTRY_INSECURE_SKIP_VERIFY_HARBOR_INTERNAL=true
```

#### 自定义 CA 证书路径
```bash
export REGISTRY_CA_CERT_PATH=/path/to/your/ca.crt
```

### 2. 证书文件配置

系统会自动搜索以下路径的证书文件：

#### Docker 风格证书路径
```
/etc/docker/certs.d/<registry-domain>/ca.crt
```

#### Containerd 风格证书路径
```
/etc/containerd/certs.d/<registry-domain>/ca.crt
```

#### 系统证书路径
```
/usr/local/share/ca-certificates/<registry-domain>.crt
```

## 使用示例

### 场景1：企业内部私有仓库（自签名证书）

```bash
# 1. 将 CA 证书放到指定位置
sudo mkdir -p /etc/containerd/certs.d/registry.company.com
sudo cp company-ca.crt /etc/containerd/certs.d/registry.company.com/ca.crt

# 2. 重启相关服务
sudo systemctl restart rbd-chaos
```

### 场景2：开发环境（跳过证书验证）

```bash
# 设置环境变量
export REGISTRY_INSECURE_SKIP_VERIFY_REGISTRY_DEV_COM=true

# 或者在 Kubernetes 部署中设置
kubectl patch deployment rbd-chaos -p '{"spec":{"template":{"spec":{"containers":[{"name":"rbd-chaos","env":[{"name":"REGISTRY_INSECURE_SKIP_VERIFY_REGISTRY_DEV_COM","value":"true"}]}]}}}}'
```

### 场景3：Harbor 私有仓库

```bash
# 1. 获取 Harbor 的 CA 证书
curl -k https://harbor.company.com/api/v2.0/systeminfo/getcert > harbor-ca.crt

# 2. 安装证书
sudo mkdir -p /etc/containerd/certs.d/harbor.company.com
sudo cp harbor-ca.crt /etc/containerd/certs.d/harbor.company.com/ca.crt

# 3. 验证配置
docker pull harbor.company.com/library/nginx:latest
```

## 自动检测机制

系统会自动检测以下情况并跳过 TLS 验证：

1. **内网地址**：
   - `10.x.x.x`
   - `172.16.x.x - 172.31.x.x`
   - `192.168.x.x`

2. **本地地址**：
   - `localhost`
   - `127.0.0.1`
   - `registry.local`

3. **环境变量控制**：
   - `REGISTRY_INSECURE_SKIP_VERIFY=true`
   - `REGISTRY_INSECURE_SKIP_VERIFY_<DOMAIN>=true`

## 故障排除

### 1. 查看 TLS 配置日志

```bash
# 查看 builder 日志
kubectl logs -f -l name=rbd-chaos -n rbd-system | grep -i tls

# 查看构建任务日志
kubectl logs -f -l job=codebuild -n rbd-system
```

### 2. 测试证书配置

```bash
# 测试证书是否有效
openssl s_client -connect registry.company.com:443 -CAfile /etc/containerd/certs.d/registry.company.com/ca.crt

# 验证证书信息
openssl x509 -in /etc/containerd/certs.d/registry.company.com/ca.crt -text -noout
```

### 3. 常见错误解决

#### x509: certificate signed by unknown authority
```bash
# 解决方案1：添加 CA 证书
sudo cp your-ca.crt /etc/containerd/certs.d/<registry-domain>/ca.crt

# 解决方案2：跳过验证（仅开发环境）
export REGISTRY_INSECURE_SKIP_VERIFY_<DOMAIN>=true
```

#### x509: certificate is valid for X, not Y
```bash
# 检查证书 SAN
openssl x509 -in cert.crt -text -noout | grep -A 1 "Subject Alternative Name"

# 如果域名不匹配，可以跳过验证
export REGISTRY_INSECURE_SKIP_VERIFY_<DOMAIN>=true
```

### 4. 验证配置是否生效

```bash
# 1. 检查环境变量
env | grep REGISTRY_

# 2. 查看证书文件
ls -la /etc/containerd/certs.d/

# 3. 测试镜像拉取
# 创建测试构建任务，观察日志输出
```

## 安全建议

1. **生产环境**：
   - 避免使用 `REGISTRY_INSECURE_SKIP_VERIFY=true`
   - 使用有效的 CA 证书
   - 定期更新证书

2. **开发环境**：
   - 可以使用跳过验证的方式
   - 建议使用特定域名的跳过配置而不是全局跳过

3. **证书管理**：
   - 使用自动化工具管理证书
   - 设置证书过期提醒
   - 备份重要的 CA 证书

## 配置优先级

1. 自定义 CA 证书（如果找到有效证书）
2. 特定域名的跳过验证设置
3. 全局跳过验证设置
4. 内网地址自动检测
5. 默认严格验证

这样的设计既保证了安全性，又提供了足够的灵活性来处理各种企业环境的需求。
