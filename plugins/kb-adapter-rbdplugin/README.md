# [Kubeblock Adapter for Rainbond Plugin](https://github.com/furutachiKurea/block-mechanica)(原 Block Mechanica)

> 本项目原名为 "Block Mechanica"，现已更名为 "Kubeblock Adapter for Rainbond Plugin"，
> 后续项目中出现的两个名称等价，go module 和 repo 名称维持不变，更名仅用于提升可读性

Kubeblock Adapter for Rainbond Plugin <del>Block Mechanica</del> 是一个轻量化的 Kubernetes 服务，通过使用 Echo 编写的 API 服务实现 KubeBlocks 与 Rainbond 的集成

## How does it work?

[如何实现 Rainbond 与 KubeBlocks 的集成](./doc/design_document.md)

## 如何部署

[在 Rainbond 中部署 KubeBlocks 和 Kubeblock Adapter for Rainbond Plugin ](./doc/Deploy.md)

## 如何在 Rainbond 中使用 KubeBlocks

绝大部分情况下，都能像使用 Rainbond 组件一样使用通过 KubeBlocks 创建的数据库

当然也存在一些不同，详见 [在 Rainbond 中使用 KubeBlocks](./doc/Use_KubeBlocks_in_Rainbond.md)

## 目录结构

```txt
📁 ./
├── 📁 api/
│   ├── 📁 handler/
│   ├── 📁 req/
│   └── 📁 res/
├── 📁 deploy/
│   ├── 📁 docker/
│   └── 📁 k8s/
├── 📁 doc/
│   └── 📁 assets/
├── 📁 internal/
│   ├── 📁 config/
│   ├── 📁 index/
│   ├── 📁 k8s/
│   ├── 📁 log/
│   ├── 📁 model/
│   ├── 📁 mono/
│   └── 📁 testutil/
└── 📁 service/
    ├── 📁 adapter/
    ├── 📁 backup/
    ├── 📁 builder/
    ├── 📁 cluster/
    ├── 📁 coordinator/
    ├── 📁 kbkit/
    ├── 📁 registry/
    └── 📁 resource/
```

## Make

- 构建 Docker 镜像（默认标签 latest）

  ```sh
  make image
  ```

- 构建 Docker 镜像并指定标签（如 v1.0.0）

  ```sh
  make image TAG=v1.0.0
  ```

- 构建可执行文件到 bin/kb-adapter

  ```sh
  make build
  ```

- 运行所有测试

  ```sh
  make test
  ```

- 运行指定目录下的测试（如 service 目录）

  ```sh
  make test TESTDIR=./service/...
  ```

## Contributing

[开发仓库](https://github.com/furutachiKurea/block-mechanica)

欢迎提交 PR 和 Issue，感谢您的贡献！

## License

[Apache 2.0](./LICENSE)