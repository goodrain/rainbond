<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/rainbond%20log_full.png" width="60%">

[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v5.2-brightgreen.svg)
[![Build Status](https://travis-ci.org/goodrain/rainbond.svg?branch=master)](https://travis-ci.org/goodrain/rainbond)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)

[项目官网](http://www.rainbond.com) • [文档](https://www.rainbond.com/docs/)

|![notification](./docs/bell-outline-badge.svg) What is NEW!|
|------------------|
|2020年8月24日 Rainbond 5.2 稳定版正式发布 [查看发布文案](https://mp.weixin.qq.com/s/q1sgEaBPaepsuUOfk1un-w)|


## Rainbond初识

<b>云原生且易用的应用管理平台</b>

Rainbond 是云原生且易用的云原生应用管理平台，云原生应用交付的最佳实践，简单易用。专注于以应用为中心的理念。赋能企业搭建云原生开发云、云原生交付云。

<b>对于企业：</b> Rainbond 是可以直接开箱即用的云原生平台，借助 Rainbond 可以快速完成企业研发和交付体系的云原生转型。

<b>对于开发者：</b> 基于 Rainbond 开发、测试和运维企业业务应用，可以开箱即用的获得全方位的云原生技术能力。包括但不仅限于持续集成、服务治理、架构支撑、多维度应用观测、流量管理。

<b>对于交付人员：</b> 基于 Rainbond 搭建产品版本化管理体系，搭建标准化客户交付环境，使传统的交付流程可以自动化、简单化和可管理。

[我要试用](https://cloud.goodrain.com/enterprise-server/trial)

<img src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/Rainbond%E4%BA%A7%E5%93%81%E6%9E%B6%E6%9E%84.png" width="100%">

## 应用场景

* 企业云原生DevOps

面向应用的DevOps开发流水线，提供从源码或简单镜像持续构建云原生应用的能力，不要求开发者具有容器化能力，面向开发者友好，对源码无侵入，业务持续发布到云端。

* 企业微服务治理

内置ServiceMesh微服务框架，微服务治理开箱即用，传统业务上云即服务化。插件化扩展和增强微服务治理功能体现，与SpringCloud Dubbo等微服务框架协同工作，有效降低微服务技术门槛。

* Kubernetes多云管理

Kubernetes技术复杂上手难；大量Deployment、Statefulset或Operator资源管理复杂都是直接使用Kubernetes集群的难题，Rainbond以应用为中心的资源管理模型屏蔽了Kubernetes的复杂度，Kubernetes资源交给Rainbond来编排管理。

* 企业中台建设与应用交付

企业业务系统多，交付项目多，IT产品多，建设统一的企业业务中台，整合企业所有数字系统、通用组件库形成数字资产，内部各团队高效复用，ToB交付场景中实现最大层度的自动化，标准化与可定制相结合。

* 国产易用的PaaS

Rainbond完成与龙芯、飞腾、麒麟操作系统等为代表的国产化计算基座的双向认证，使Rainbond具有屏蔽底层CPU架构、操作系统的差异，对用户提供统一的国产化业务应用管理平台。

## 主要功能特性

| 特性                       | 描述                                                         |
| -------------------------- | ------------------------------------------------------------ |
| Kubernetes多云管理         | 平台底层基于Kubernetes，但用户无需学习和编辑复杂的yaml文件，开发者仅需要以最简单的方式构建和维护应用模型，所有Kubernetes资源由Rainbond编排创建和维护。 |
| Service Mesh微服务架构 | 内置跨语言、跨协议、代码无侵入的Service Mesh微服务架构原生支持，传统应用直接变成微服务架构。同时支持常见微服务架构Spring Cloud、Dubbo等，通过插件扩展架构能力及治理功能。 |
| 源码构建 | 开发者无需关注底层资源，从源代码（无需Dockerfile）或已有简单镜像即可持续发布应用组件。支持常用的Java Python PHP Golang NodeJS NodeJS前端 .NetCore 等开发语言。 |
| 一体化DevOps               | 以应用为中心，衔接开发、测试、构建、上线、运维的一体化DevOps。 |
| 企业级应用市场             | 非镜像市场和服务目录，支持各类企业级应用，像手机应用一样即点即用，全流程管理（应用开发、应用发布、应用展示、应用离线导入/导出、应用安装/升级、应用运维） |
| 自动化运维                 | 应用自动化运维。节点自动安装、扩容、监控、容错。平台支持高可用、多数据中心管理、多租户管理。 |
| 无侵入性 | Rainbond在源码构建，服务组装，服务治理，微服务框架等多个方面体现无侵入性 |
| Serverless PaaS            | 以应用为核心，使用过程不需要了解服务器相关概念，简单灵活。通过对接行业应用，快速构建行业专有PaaS。 |
| 应用网关                   | 基于HTTP、HTTPs、TCP、UDP等协议应用访问控制策略，轻松操作应用灰度发布、A/B测试。 |
| 异构服务统一管理            | 以第三方组件集成的方式，支持集群内外不同架构服务统一管理、监控和通信治理。                  |
| 应用描述模型              | 以应用为中心描述应用包含的组件特性，应用特性，部署运维特性，实现复杂应用的标准化交付。 |

更多功能特性详见： 

[Rainbond功能特性说明](https://www.rainbond.com/docs/quick-start/edition/)
[Rainbond开发计划](https://www.rainbond.com/docs/quick-start/roadmap/)

## 快速开始

1.  [快速安装 Rainbond 集群](https://www.rainbond.com/docs/quick-start/rainbond_install/)
2.  [创建第一个应用（服务）](https://www.rainbond.com/docs/user-manual/app-creation/)
3.  [观看教程视频，快速学习Rainbond](https://www.rainbond.com/video.html)
4.  [搭建 ServiceMesh 微服务架构](https://www.rainbond.com/docs/advanced-scenarios/micro/)

## 参与社区

[Rainbond 开源社区](https://t.goodrain.com)    欢迎你在社区中查阅或贡献Rainbond的用例用法。    

[Rainbond 项目官网](https://www.rainbond.com)    查阅关于Rainbond的更多信息。

<center><img width="200px" src="https://grstatic.oss-cn-shanghai.aliyuncs.com/images/12141565594759_.pic_hd.jpg"/></center>
<center>加入微信群，与社区用户一起交流 Rainbond </center>

## 参与贡献

我们非常欢迎你参与Rainbond社区关于平台使用经验、标准化应用、插件分享等领域的贡献和分享。

若你是正在使用Rainbond的用户，且对Rainbond有深入的了解和技术路线的认同，在你的企业内部有较大的需求，我们非常欢迎你 [参与Rainbond项目开发贡献](https://www.rainbond.com/docs/contribution/)

## 相关项目

当前仓库为Rainbond数据中心端核心服务实现代码，项目还包括以下子项目：

   * [Rainbond-Console](https://github.com/goodrain/rainbond-console) Rainbond控制台服务端项目
   * [Rainbond-Console-UI](https://github.com/goodrain/rainbond-ui) Rainbond控制台前端项目
   * [Rainbond-Operator](https://github.com/goodrain/rainbond-operator) Rainbond安装运维项目
   * [Rainbond-Builder](https://github.com/goodrain/builder) Rainbond源码构建工具集
   * [Rainbond-Docs](https://github.com/goodrain/rainbond-docs) Rainbond文档

## License

Rainbond 遵循 LGPL-3.0 license 协议发布，详情查看[LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE)及[Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。
