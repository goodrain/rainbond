# 云帮

<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_logo.png">

----
云帮（Rainbond）是国内首个开源企业级应用管理平台(无服务器PaaS)，集CI/CD自动化应用构建系统、微服务架构应用管理系统、全自动计算资源管理系统于一身，提供“以应用为中心”理念的最佳实践。

云帮深度整合[Kubernetes](https://github.com/kubernetes/kubernetes)、 [Docker](https://github.com/moby/moby)等顶级容器生态开源项目，并历经超过五年的生产运营打磨和验证，形成目前理念最新、生态最完整的应用管理平台。

如今，我们选择开源、拥抱社区，期望吸收最好的想法和实践，进一步完善和提升云帮，让更多企业和个人用户享受“以应用为中心”的技术体验。

----

## 快速开始

1. [安装云帮数据中心]().
2. [安装云帮应用控制台]().
3. [创建你的第一个应用]().

## 快速构建

通过两种方式快速构建云帮组件：

##### Golang开发环境

```
$go get -d github.com/goodrain/rainbond
$cd $GOPATH/src/github.com/goodrain/rainbond
$make all
```
##### Docker环境

```
$git clone https://github.com/goodrain/rainbond.git
$cd rainbond
$make all-image
```
##### BUG提交

在学习和使用中发现Bug，请移步[ISSUES](https://github.com/goodrain/rainbond/issues)，查找类似Bug及其修复方案。若无类似问题，请新建Issue。

## [云帮架构]()

### 架构总图   

<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_architecture.png" href="">

### 云帮构成

云帮由[云帮数据中心](https://github.com/goodrain/rainbond) 和[云帮应用控制台](https://github.com/goodrain/rainbond-ui) 云帮资源控制台(企业版提供)构成，并无缝对接好雨云市，以此实现超融合计算池。

* [云帮数据中心]()    

云帮数据中心由[一系列分布式组件]()构成，面向资源抽象云帮节点，面向应用抽象存储、网络以及计算资源。本着插件化、分布式、软件定义一切的设计原则，云帮可在任何计算环境（公有云，私有云，IDC，行业计算）之上构建统一的应用运行环境。

* [云帮应用控制台]()

云帮应用控制台是一个Web控制台，对接多个云帮数据中心，提供应用的全生命周期管理功能。    

## 社区支持

### 云帮用户交流群(QQ)：

- 477016432(1群)  
- 453475798(2群)  
- 419331946(3群)

### 文档支持

- [云帮开发文档](http://doc.goodrain.com/cloudbang-community-install/247616)
- [安装文档](http://doc.goodrain.com/cloudbang-community-install/247616)
- [使用文档](http://doc.goodrain.com/usage)
- [平台维护](http://doc.goodrain.com/cloudbang-community-install/215655)
- [企业版功能介绍](http://doc.goodrain.com/cloudbang-enterprise)

