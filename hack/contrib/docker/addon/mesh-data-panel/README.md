# envoy_discover_service

`PREFIX`  URL前缀path配置，例如/api

`DOMAINS` 内网请求域名配置，基于配置的域名转发至下游应用

`LIMITS`  TCP限速，配置范围0～2048，于框体内填入数字，若配置0则触熔断

`MaxPendingRequests` HTTP挂起请求，配置范围0～2048，于框体内填入数字，配置0则立即挂起请求

`WEIGHT` 转发权重设置，范围1~100，该参数会判断多个拥有相同域名的下游服务来进行权重分配，权重之和必须是100，否则会导致无法访问

`HEADERS` HTTP请求头设置，为k:v格式，多个由“;”隔开，例如header1:mm;header2:nn

`MaxRequests` 最大请求数限制默认为1024, 设置0为0请求

`MaxRetries` 最大重试次数默认为3, 设置0为0重试
