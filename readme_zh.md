<img src="./docs/rainbond_logo.png" width="100%">

[![Go Report Card](https://goreportcard.com/badge/github.com/goodrain/rainbond)](https://goreportcard.com/report/github.com/goodrain/rainbond)
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v3.7-brightgreen.svg)
[![Build Status](https://travis-ci.org/goodrain/rainbond.svg?branch=master)](https://travis-ci.org/goodrain/rainbond)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[网站](http://www.rainbond.com) • [文档](https://www.rainbond.com/docs/stable/) • [公有云](https://console.goodrain.com) • [README in English](https://github.com/goodrain/rainbond/blob/master/README.md)

**Rainbond**是以应用为中心的 PaaS，领先的理念和完整的生态源自于不断的验证和优化。

Rainbond（云帮）是"以应用为中心”的开源PaaS， 深度整合基于Kubernetes的容器管理、ServiceMesh微服务架构最佳实践、多类型CI/CD应用构建与交付、多数据中心资源管理等技术， 为用户提供云原生应用全生命周期解决方案，构建应用与基础设施、应用与应用、基础设施与基础设施之间互联互通的生态体系， 满足支撑业务高速发展所需的敏捷开发、高效运维和精益管理需求。

## 功能特性

#### 应用管理

* 应用级（完整业务系统）编排
* 集成 Kubernetes 的服务组件级编排与调度
* 基于扩展式 ServiceMesh 的服务网格系统提供原生微服务治理支持（服务发现、动态路由、限流与熔断）
* 支持其他各类(SpringCloud、Dubbo、API-Gateway)微服务架构
* 支持多种(HTTP、Mysql)类型协议的业务级应用性能分析
* 支持基于源代码（Java、PHP、Python、Golang、Ruby 等）构建服务
* 支持基于公(私)有Git、Svn 代码仓库、镜像仓库或对接第三方CI系统的服务持续构建和部署
* 支持基于 Docker 容器镜像、Docker-Run 命令、DockerCompose 文件智能快捷构建应用
* 支持应用级全量备份与恢复，跨租户或跨数据中心迁移应用
* 服务插件体系支持，灵活扩展应用附属功能，例如：日志处理、防火墙、流量分析等
* 公（私）有应用市场支持，完善的应用交付体系

#### 资源/云管理

- 作为建设云原生数据中心的基础系统
- 插件化支持部署不同的服务网关(Openresty、F5等)、SDN 网络（Midonet、Calico）、分布式存储系统（GlusterFS、Ali-NAS、Ceph等）
- 支持多数据中心(集群)管理和应用编排
- 平台高可用支持
- 集群自动化管理与运维，自动化的健康检查机制
- 节点物理硬件与系统指标的自动监控
- 服务实例容器指标自动监控

## 快速开始

1.  [安装 Rainbond平台](https://www.rainbond.com/docs/stable/getting-started/installation-guide.html)
2.  [创建第一个应用（服务）](https://www.rainbond.com/docs/stable/user-manual/create-an-app.html)
3.  [搭建 ServiceMesh 微服务架构](https://www.rainbond.com/docs/stable/microservice/service-mesh/use-case.html)

## 社区

[Rainbond 社区](https://t.goodrain.com)    

[Rainbond 官网](https://www.rainbond.com)

## Roadmap

点击查看 Rainbond 版本开发计划 [Roadmap](http://www.rainbond.com/docs/stable/architecture/roadmap.html)

## 架构

<img src="https://static.goodrain.com/images/docs/3.6/architecture/architecture.png" href="http://www.rainbond.com/docs/stable/architecture/architecture.html">

## 产品图示

<img src="./docs/buildfromsourcecode.gif" href="http://www.rainbond.com/docs/stable">

- 源码构建示意图

<img src="./docs/topology.gif" href="http://www.rainbond.com/docs/stable">

- 应用流量拓扑示意图

## 参与贡献

你可以参与Rainbond社区关于平台、应用、插件等领域的贡献和分享。
请移步： [Rainbond 贡献者社区](https://t.goodrain.com/c/contribution)

## License

Rainbond 遵循 LGPL-3.0 license 协议发布，详情查看[LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE)及[Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。

## 鸣谢

感谢以下开源项目

- [Kubernetes](https://github.com/kubernetes/kubernetes)
- [Docker/Moby](https://github.com/moby/moby)
- [Heroku Buildpacks](https://github.com/heroku?utf8=%E2%9C%93&q=buildpack&type=&language=)
- [OpenResty](https://github.com/openresty/)
- [Calico](https://github.com/projectcalico)
- [Midonet](https://github.com/midonet/midonet)
- [Etcd](https://github.com/coreos/etcd)
- [Prometheus](https://github.com/prometheus/prometheus)
- [GlusterFS](https://github.com/gluster/glusterfs)
- [Ceph](https://github.com/ceph/ceph)
- [CockroachDB](https://github.com/cockroachdb/cockroach)
- [MySQL](https://github.com/mysql/mysql-server)
- [Weave Scope](https://github.com/weaveworks/scope)
- [Ant Design](https://github.com/ant-design/ant-design)

## 加入我们

[非常欢迎热爱技术的你加入我们](https://www.rainbond.com/docs/recruitment/join.html)
