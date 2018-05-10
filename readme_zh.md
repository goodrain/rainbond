<img src="./docs/rainbond_logo.png" width="100%">

[![Go Report Card](https://goreportcard.com/badge/github.com/goodrain/rainbond)](https://goreportcard.com/report/github.com/goodrain/rainbond) 
[![GitHub stars](https://img.shields.io/github/stars/goodrain/rainbond.svg?style=flat-square)](https://github.com/goodrain/rainbond/stargazers)
![Rainbond version](https://img.shields.io/badge/version-v3.5-brightgreen.svg)
[![Build Status](https://travis-ci.org/goodrain/rainbond.svg?branch=master)](https://travis-ci.org/goodrain/rainbond)
[![GoDoc](https://godoc.org/github.com/goodrain/rainbond?status.svg)](https://godoc.org/github.com/goodrain/rainbond)


[网站](http://www.rainbond.com) • [文档](https://www.rainbond.com/docs/stable/) • [公有云](https://sso.goodrain.com/#/login/https%3A%2F%2Fwww.goodrain.com%2F%23%2Findex) • [README in English](https://github.com/goodrain/rainbond/blob/V3.6/readme.md)

**Rainbond**是以应用为中心的PaaS，领先的理念和完整的生态源自于不断的验证和优化。

Rainbond深度整合基于Kubernetes的容器管理、Service Mesh微服务架构最佳实践、多类型CI/CD应用构建与交付、多数据中心资源管理等技术，为用户提供云原生应用全生命周期解决方案，构建应用与基础设施、应用与应用、基础设施与基础设施之间的互联互通，满足支撑业务高速发展所需的敏捷开发、高效运维和精益管理需求，赋予企业快速将已有应用SaaS化，服务化转型的能力。

## 功能特性

* 集成Kubernetes的应用调度系统
* 以应用为中心的控制台
* 支持各类(SpringCloud、Dubbo、API-Gateway)微服务架构应用部署
* 基于扩展式ServiceMesh的服务网格系统提供原生微服务治理支持（服务发现、动态路由、限流与熔断）
* 支持多种(HTTP、Mysql、PostgraSQL)类型协议的业务级应用性能分析
* 支持基于源代码（Java、PHP、Python、Golang、Ruby等）构建应用
* 支持基于私有Git仓库、Github等公有仓库的应用持续构建和部署
* 支持基于Docker容器镜像、Docker-Run命令、DockerCompose文件智能快捷构建应用
* 数据中心插件化支持部署不同的负载均衡、SDN网络、各类型存储系统
* 公有、私有应用商店支持，完善的应用分享体系
* 多数据中心、跨数据中心应用调度部署
* 管理节点（平台服务）高可用支持
* 计算集群自动化管理，按需伸缩，全面的节点监控

## 快速开始

1. [安装Rainbond](http://www.rainbond.com/docs/stable/getting-started/pre-install.html)
2. [创建应用](http://www.rainbond.com/docs/stable/user-app-docs/addapp/addapp-code.html)
3. [搭建微服务架构](http://www.rainbond.com/docs/stable/user-app-docs/addapp/addapp-cloud_framework.html)

## 架构

<img src="./docs/rainbond_architecture.png" href="http://www.rainbond.com/docs/stable/getting-started/architecture.html">

## 产品图示

<img src="./docs/buildfromsourcecode.gif" href="http://www.rainbond.com/docs/stable">

* 源码构建示意图

<img src="./docs/topology.gif" href="http://www.rainbond.com/docs/stable">

* 应用流量拓扑示意图

## Roadmap

[>>3.6-Roadmap](https://github.com/goodrain/rainbond/projects/3)

## 参与贡献

阅读[CONTRIBUTING](https://github.com/goodrain/rainbond/blob/master/CONTRIBUTING.md)了解如何参与贡献。

## 社区

* 微信：添加微信号 "**qingguo-wei**" 并接受邀请入群  
* Stack Overflow: https://stackoverflow.com/questions/tagged/rainbond

## License

Rainbond遵循LGPL-3.0 license协议发布，详情查看[LICENSE](https://github.com/goodrain/rainbond/blob/master/LICENSE)及[Licensing](https://github.com/goodrain/rainbond/blob/master/Licensing.md)。

