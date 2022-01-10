<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/rainbond%20log_full.png" width="60%">

[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v5.5-brightgreen.svg)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[项目官网](http://www.rainbond.com?channel=github) • [文档](https://www.rainbond.com/docs?channel=github)

| ![notification](./docs/bell-outline-badge.svg) What is NEW!                                                      |
| ---------------------------------------------------------------------------------------------------------------- |
| 2021 年 12 月 13 日 Rainbond 5.5.0 发布 [查看发布文案](https://www.rainbond.com/docs/community/change/5.4.0-5.5.0?channel=github)        |

## Rainbond 初识

<b>云原生且易用的应用管理平台</b>

Rainbond 是云原生且易用的云原生应用管理平台，云原生应用交付的最佳实践，简单易用。专注于以应用为中心的理念。赋能企业搭建云原生开发云、云原生交付云。

<b>对于企业：</b> Rainbond 是开箱即用的云原生平台，借助 Rainbond 可以快速完成企业研发和交付体系的云原生转型。

<b>对于开发者：</b> 基于 Rainbond 开发、测试和运维企业业务应用，开箱即用的获得全方位的云原生技术能力。包括但不仅限于持续集成、服务治理、架构支撑、多维度应用观测、流量管理。

<b>对于交付人员：</b> 基于 Rainbond 搭建产品版本化管理体系，搭建标准化客户交付环境，使传统的交付流程可以自动化、简单化和可管理。

[我要试用](https://cloud.goodrain.com/enterprise-server/trial)

### 价值场景

#### <b>企业云原生 DevOps</b>

面向应用的云原生 DevOps， 开发、测试、生产运维一体化，不要求开发者具有容器、Kubernetes 等复杂能力，面向开发者友好；提供从源码或简单镜像持续构建云原生应用的能力，对源码无侵入，业务持续发布到云端；高效的自动化运维，帮助开发者高效管理高可用的、安全的且去中心化的业务系统。

#### <b>搭建 To B 软件交付系统  </b>

- <b>构建在线的多客户持续交付体系</b>

传统 To B 交付往往投入较大的人力、时间成本。客户多，定制多，产品版本升级都会带来挑战。

(1). Rainbond 搭建高效的开发平台，产出标准化交付产品。

(2). Rainbond 作为客户环境的基础平台，即可在线完成交付产品的安装和持续升级。

(3). 将所有的 Rainbond 端都接入到 [Rainstore](https://store.goodrain.com) 中。在线管理客户，管理客户的交付应用，在线批量持续升级。

- <b>构建离线的高效交付体系</b>

离线环境的不确定性往往意味着人力的持续投入和沟通成本的增加，更需要将交付的产品和持续的升级能力标准化。Rainbond 开发平台产出标准化应用离线安装包，人工导入到离线 Rainbond 环境即可持续升级。

#### <b>企业从交付软件到交付服务转型</b>

交付服务意味着持续的收入，业务厂商提供持续的业务服务需要两个能力：获得较强的运维能力和对客户交付业务的持续迭代能力。Rainbond 使业务厂商可以高效交付多套业务系统，对每个客户业务系统可以持续开发集成，自动化运维保障所有业务的可用性。

另外 借助 [Rainstore](https://store.goodrain.com) 的产品（解决方案）展示、在线交易、产品管理、在线自动化交付、批量升级等能力帮助企业快速实现转型。

#### <b>行业集成商集成行业应用交付</b>

行业集成商既要面对客户，又要面对供应商。Rainbond 给行业集成商赋予建立应用交付标准的能力。为供应商提供 Rainbond 标准应用接入平台，产品统一发布到组件库中。行业集成商即可从组件库选择合适的产品组成解决方案一键交付到客户环境。

另外 借助 [Rainstore](https://store.goodrain.com) 的产品（解决方案）展示、组装能力，建立行业云应用商店，整合行业 IT 解决方案。

#### <b>企业技术中台建设</b>

企业技术中台包括技术中间件管理和基础业务模块化。Rainbond 结合可扩展的组件控制器，扩充统一管理云数据库、大数据中间件、人工智能中间件等技术中间件基础设施。提供业务中间件持续发布共享，积累业务通用模块。基础能力服务于企业业务场景。

#### <b>Kubernetes 多云管理</b>

Kubernetes 技术复杂上手难；大量 Deployment、Statefulset 或 Operator 资源管理复杂都是直接使用 Kubernetes 集群的难题，Rainbond 以应用为中心的资源管理模型屏蔽了 Kubernetes 的复杂度，Kubernetes 资源全部交给 Rainbond 来编排管理。

#### <b>国产易用的云原生 PaaS</b>

Rainbond 完成与龙芯、飞腾、麒麟操作系统等为代表的国产化计算基座的双向认证，使 Rainbond 具有屏蔽底层 CPU 架构、操作系统的差异的能力，对用户提供统一的国产化业务应用管理平台。

### 核心能力与技术

| 场景                         | 主要功能与能力                                                                               | 核心技术                                                                                                                |
| ---------------------------- | -------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| 企业云原生 DevOps            | 持续集成；持续交付；内置微服务架构；流量管理；应用全维度监控；异构服务统一管理；             | 云原生 CI/CD；Code To Image(无需 Dockerfile)；以应用为中心抽象；ServiceMesh；应用网关；应用监控；业务通用服务治理框架。 |
| 搭建 To B 软件交付系统       | 应用模型管理；应用模型离线导出；应用模型同步到云端；应用持续升级                             | 云原生应用模型抽象（类似于 [OAM](https://oam.dev/))；多端交付模型转换；应用升级控制；跨云互联                           |
| 企业从交付软件到交付服务转型 | 自动化运维；应用一键安装；应用升级；流量管理                                                 | 业务自恢复控制；应用模型转换；资源自动化按需调度                                                                        |
| 企业技术中台建设             | 开源中间件同步复用；业务模块发布复用；中间价集群管理；数据库、大数据、人工智能基础服务管理。 | 组件库模型打包与管理；Operator 组件类型扩展；业务集群监控                                                               |
| Kubernetes 多云管理          | 多集群接入；集群监控视图；自动化调度                                                         | 集群自动化接入；公有云 Kubernetes 服务接入；自动化资源生成与维护。                                                      |
| 国产易用的云原生 PaaS        | 支持常见国产 CPU 和操作系统，支持 Windows 操作系统过度到国产操作系统。                       | 异构资源调度；异构操作系统应用编排。                                                                                    |

[Rainbond 功能特性说明](https://www.rainbond.com/docs/quick-start/edition?channel=github)
[Rainbond 开发计划](https://www.rainbond.com/docs/quick-start/roadmap?channel=github)

## 快速开始

1. [快速安装 Rainbond 集群](https://www.rainbond.com/docs/quick-start/rainbond_install?channel=github)
2. [创建第一个应用（组件）](https://www.rainbond.com/docs/get-start/create-app-from-source?channel=github)
3. [搭建 ServiceMesh 微服务架构](https://www.rainbond.com/docs/get-start/create-dependency?channel=github)
4. [观看教程视频，快速学习 Rainbond](https://www.rainbond.com/video.html?channel=github)

## 参与社区

[Rainbond 开源社区](https://t.goodrain.com) 欢迎你在社区中查阅或贡献 Rainbond 的用例用法。

[Rainbond 项目官网](https://www.rainbond.com?channel=github) 查阅关于 Rainbond 的更多信息。

微信扫码关注Rainbond公众号，添加群助手进入Rainbond交流群喔！

<img width="300px" src="https://static.goodrain.com/wechat/WechatQRCode.gif"/>

钉钉搜索群号加入Rainbond技术交流群: `31096419`

## 参与贡献

我们非常欢迎你参与 Rainbond 社区关于平台使用经验、标准化应用、插件分享等领域的贡献和分享。

若你是正在使用 Rainbond 的用户，且对 Rainbond 有深入的了解和技术路线的认同，在你的企业内部有较大的需求，我们非常欢迎你 [参与 Rainbond 项目开发贡献](https://www.rainbond.com/docs/community/contribution?channel=github)

## 相关项目

当前仓库为 Rainbond 数据中心端核心服务实现代码，项目还包括以下子项目：

- [Rainbond-Console](https://github.com/goodrain/rainbond-console) Rainbond 控制台服务端项目
- [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui) Rainbond 控制台前端项目
- [Rainbond-Operator](https://github.com/goodrain/rainbond-operator) Rainbond 安装运维项目
- [Rainbond-Builder](https://github.com/goodrain/builder) Rainbond 源码构建工具集
- [Rainbond-Docs](https://github.com/goodrain/rainbond-docs) Rainbond 文档

## License

Rainbond 遵循 LGPL-3.0 license 协议发布，详情查看[LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE)及[Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。
