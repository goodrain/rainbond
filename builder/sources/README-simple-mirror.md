# Containerd 镜像加速 - 环境变量配置

## 概述

这是一个简化的镜像加速功能，只需要通过环境变量配置即可使用。

## 使用方法

### 1. 设置环境变量

```bash
# 配置镜像加速器
export CONTAINERD_MIRRORS='{"docker.io":["https://registry.cn-hangzhou.aliyuncs.com","https://docker.mirrors.ustc.edu.cn"]}'

# 可选：配置代理
export HTTP_PROXY="http://proxy.example.com:8080"
export HTTPS_PROXY="https://proxy.example.com:8080"
```

### 2. 环境变量格式

`CONTAINERD_MIRRORS` 是一个 JSON 格式的字符串，格式如下：

```json
{
  "镜像仓库域名": ["镜像加速器地址1", "镜像加速器地址2"]
}
```

### 3. 常用镜像加速器配置

#### Docker Hub 镜像加速

```bash
# 阿里云镜像加速器
export CONTAINERD_MIRRORS='{"docker.io":["https://registry.cn-hangzhou.aliyuncs.com"]}'

# 中科大镜像加速器
export CONTAINERD_MIRRORS='{"docker.io":["https://docker.mirrors.ustc.edu.cn"]}'

# 网易镜像加速器
export CONTAINERD_MIRRORS='{"docker.io":["https://hub-mirror.c.163.com"]}'

# 腾讯云镜像加速器
export CONTAINERD_MIRRORS='{"docker.io":["https://mirror.ccs.tencentyun.com"]}'
```

#### 多个镜像加速器（备选）

```bash
export CONTAINERD_MIRRORS='{"docker.io":["https://registry.cn-hangzhou.aliyuncs.com","https://docker.mirrors.ustc.edu.cn","https://hub-mirror.c.163.com"]}'
```

#### 其他镜像仓库

```bash
# 配置多个镜像仓库
export CONTAINERD_MIRRORS='{
  "docker.io": ["https://registry.cn-hangzhou.aliyuncs.com"],
  "gcr.io": ["https://gcr.mirrors.ustc.edu.cn"],
  "quay.io": ["https://quay.mirrors.ustc.edu.cn"]
}'
```

### 4. 验证配置

启动应用后，查看日志输出：

```
INFO Applying mirror config for docker.io: [https://registry.cn-hangzhou.aliyuncs.com https://docker.mirrors.ustc.edu.cn]
```

### 5. 完整示例

```bash
#!/bin/bash

# 设置镜像加速器
export CONTAINERD_MIRRORS='{
  "docker.io": [
    "https://registry.cn-hangzhou.aliyuncs.com",
    "https://docker.mirrors.ustc.edu.cn"
  ],
  "gcr.io": [
    "https://gcr.mirrors.ustc.edu.cn"
  ]
}'

# 设置代理（如果需要）
export HTTP_PROXY="http://proxy.example.com:8080"
export HTTPS_PROXY="https://proxy.example.com:8080"

# 启动应用
./your-app
```

## 支持的镜像仓库

- `docker.io` - Docker Hub
- `gcr.io` - Google Container Registry
- `quay.io` - Quay.io
- `ghcr.io` - GitHub Container Registry
- 自定义镜像仓库域名

## 注意事项

1. 环境变量必须在应用启动前设置
2. JSON 格式必须正确，不能有多余的逗号
3. 镜像加速器地址必须是完整的 URL（包含协议）
4. 如果镜像加速器不可用，会自动回退到原始地址

## 故障排除

### 配置不生效

检查环境变量是否正确设置：

```bash
echo $CONTAINERD_MIRRORS
```

### 镜像拉取失败

检查镜像加速器是否可访问：

```bash
curl -I https://registry.cn-hangzhou.aliyuncs.com/v2/
```

### JSON 格式错误

使用在线 JSON 验证工具检查格式，或者使用单行格式：

```bash
export CONTAINERD_MIRRORS='{"docker.io":["https://registry.cn-hangzhou.aliyuncs.com"]}'
``` 