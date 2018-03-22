# Since release-3.4.2 
| history version | current version | upgrade doc|
|-----|---|-----|
|v3.4.2 | v3.5 | [upgrade](http://www.rainbond.com/docs/dev/FAQs/install-maintenance-faqs.html#3-4-2-3-5)
## FEATURE&&CHANGE FROM v3.4.2

> Console UI

* Console UI has been rebuilt as SPA architecture with `ant design pro` for better user experience
* Presentation layer of front-end code has been separated from business logic layer for better maintainability

> Application CI

* Build application by docking private git repository via SSH & HTTP protocol
* Integrated Gitlab and Github Service
* Specify sub-repository directory as build directory
* Generate default application attributes based on source code types
intelligently
* Get and switch source code repository branches
* Stable application build with dockerun and docker-compose
* Parsing application attributes from dockerfile and images

> Application management

* Application performance analysis stably supports HTTP and MySQL protocol
* Define application connection attributes
* Display application access info intelligently
* Quick statistics and query team and application resource usage such as memory and disk

> Application market

* New application sharing process and business logics
* Share application to Rainbond's internal market
* Interconnect with Goodrain application market to download free application

> User and team

* Create multiple teams
* Connect custom data center (public cloud data center support is on the way)
* Improved team user management

> Install

* Optimized install process, simplified install steps
* Optimized etcd and Kubernetes' install method, support containerized deployment
* Support for Centos & Debian/Ubuntu

### BUG FIX

* Application port creation exception
* Incomplete dependency display
* Invalid HTTPS under certain circumstances
* TCP load balancing with Openresty
* Application build exception with dockerfile
* Port alias cannot be set
* Add storage exception when create images[(#31)](https://github.com/goodrain/rainbond/issues/31)
* Slow resource usage query interfaces

