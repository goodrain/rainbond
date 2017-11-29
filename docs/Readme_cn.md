
<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_logo.png">

----
好雨云帮（Rainbond）是国内首个开源的生产级无服务器PasS平台，深度整合基于Kubernetes的容器管理、多类型CI/CD应用构建与交付、多数据中心的资源管理等技术提供完整的云原生应用管理解决方案，构建出应用、基础设施之间的互联互通生态体系。

Rainbond历经超过五年的生产运营打磨和验证，形成目前理念最新、生态最完整的无服务器PasS平台。如今，我们选择开源、拥抱社区，期望吸收最好的想法和实践，进一步完善和提升云帮，让更多企业和个人用户享受“以应用为中心”的技术体验。

----
[设计理念](http://www.rainbond.com/docs/stable/getting-started/design-concept.html) -- -- [技术架构](http://www.rainbond.com/docs/stable/getting-started/architecture.html) -- -- [应用场景](getting-started/scenario-microservice.html) -- -- [系统安装](http://www.rainbond.com/docs/stable/getting-started/pre-install.html)

----
## 使用Rainbond

1. [安装Rainbond](http://www.rainbond.com/docs/stable/getting-started/pre-install.html)
2. [创建第一个应用](http://www.rainbond.com/docs/stable/user-app-docs/addapp/addapp-code.html)
3. [快速构建微服务架构](http://www.rainbond.com/docs/stable/user-app-docs/addapp/addapp-cloud_framework.html)

## 开发Rainbond

本仓库具有Rainbond数据中心核心组件，通过两种方式快速构建：

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
##### BUG与建议

在学习和使用中发现Bug，请移步[ISSUES](https://github.com/goodrain/rainbond/issues)，查找类似Bug及其修复方案。若无类似问题，请新建Issue。

## [Rainbond架构](http://www.rainbond.com/docs/stable/getting-started/architecture.html)

### 组件架构图 

<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_architecture.png" href="http://www.rainbond.com/docs/stable/getting-started/architecture.html">

## 社区支持

### Rainbond用户交流群(QQ)：

- 477016432(1群)  
- 453475798(2群)  
- 419331946(3群)

### [项目文档支持](http://www.rainbond.com/docs/stable/)       
文档地址：https://www.rainbond.com/docs
博客地址：https://blog.goodrain.com/

