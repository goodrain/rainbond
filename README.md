# Rainbond

<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_logo.png" width="30%">

----
Rainbond is the first opensource enterprise application management platform (serverless PaaS) in China. It integrates CI/CD automation application building system, microservice architecture application management system and fully-automatic computing resource management system, to provide best practice of application-centic philosophy.

Rainbond is cutting edge application management platform with complete ecosystem, based on [Kubernetes](https://github.com/kubernetes/kubernetes) and [Docker](https://github.com/moby/moby), has been optimized and verified for 5 five years.

We choose to open source and embrace the community, absorbing best ideas and practices to further improve and enhance Rainbond, enabling more enterprise and individuals to enjoy "application-centric" experience.

----
## [中文Readme](https://github.com/goodrain/rainbond/blob/master/docs/Readme_cn.md)
## Quick Start

1. [Install Rainbond Data Center]()
2. [Install Rainbond Application Console]()
3. [Build Your First Application](http://www.rainbond.com/docs/stable/getting-started/how-to-create-app.html)

## Quick Build

Quickly build Rainbond components in two ways:

##### Golang

```
$go get -d github.com/goodrain/rainbond
$cd $GOPATH/src/github.com/goodrain/rainbond
$make all
```
##### Docker

```
$git clone https://github.com/goodrain/rainbond.git
$cd rainbond
$make all-image
```
##### BUG Submission

Bug found in learning and using, please visit [ISSUES](https://github.com/goodrain/rainbond/issues) to find similar Bug and solutions. If there is no similar result, please create a new issure.

## [Rainbond Architecture](http://www.rainbond.com/docs/stable/getting-started/architecture.html)

### Architecture

<img src="https://github.com/goodrain/rainbond/blob/master/docs/rainbond_architecture.png"  href="http://www.rainbond.com/docs/stable/getting-started/architecture.html">

### Rainbond Structure

Rainbond consisted of [Rainbond Data Center](https://github.com/goodrain/rainbond) and [Rainbond Resource Console](https://github.com/goodrain/rainbond-ui)(Enterprise edition available), seamlessly docked with 好雨云市, enabling hyper-converged computing pools.

* [Rainbond Data Center](https://github.com/goodrain/rainbond)    

Rainbond Data Center consisted of [a series of distributed components](http://www.rainbond.com/docs/stable/platform-maintenance/add-management-node/component-introduction/overview.html), enabling resource-oriented Rainbond node abstraction, application-oriented storage, network and computing resources. With plug-in, distributed and software-defined principles, Rainbond can build unified application runtime environment on any computing environment, includes public cloud, private cloud, IDC and industry computing cloud.

* [Rainbond Application Console](https://github.com/goodrain/rainbond-ui)

Rainbond Application Console is Web console that interfaces with multiple Rainbond Data Centers, to provide application lifecycle management capabilities.

## Community

### Rainbond QQ Group

- 477016432(Group 1)  
- 453475798(Group 2)  
- 419331946(Group 3)

### Documentation

- [Development](http://doc.goodrain.com/cloudbang-community-install/247616)
- [Installation](http://www.rainbond.com/docs/stable/getting-started/pre-install.html)
- [Manual](http://www.rainbond.com/docs/stable/user-app-docs/addapp/addapp-image.html)
- [Maintenance](http://www.rainbond.com/docs/stable/platform-maintenance/overview/overview.html)