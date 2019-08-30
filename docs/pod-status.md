# Pod status

## 实例(Pod)详情

**url**

http://xxx/v2/tenants/<tenant_name>/service/<service_alias>/pod/<pod_name>/detail

**响应**

| 字段       | 类型   | 说明                                                 |
|:-----------|:-------|:-----------------------------------------------------|
| name       | string | 名字                                                 |
| node       | string | 所在节点                                             |
| start_time | string | 创建时间                                             |
| status     | object | 实例状态, <a href="#podstatus">`status`</a>          |
| ip         | string | 实例IP地址                                           |
| containers | array  | 实例中的容器, <a href="#containers">`containers`</a> |
| events     | array  | 事件 , <a href="#events">`events`</a>                |

<a id="podstatus">podstatus</a>

| 字段    | 类型   | 说明     |
|:--------|:-------|:---------|
| status  | string | 实例状态 |
| reason  | string | 原因     |
| message | string | 说明     |
| advice  | string | 建议     |

<a id="containers">containers</a>

| 字段           | 类型   | 说明                                              |
|:---------------|:-------|:--------------------------------------------------|
| image          | string | 镜像名                                            |
| state          | string | 状态                                              |
| reason         | string | 原因, 对应上一个字段State, 异常状态的原因         |
| started        | string | 原因, 对应上一个字段State, 容器开始正常运行的时间 |
| limit_memory   | 待定   | 内存上限                                          |
| limit_cpu      | 待定   | CPU上限                                           |
| request_memory | 待定   | 内存下限                                          |
| request_cpu    | 待定   | CPU下限                                           |

<a id="events">events</a>

| 字段    | 类型   | 说明 |
|:--------|:-------|:-----|
| type    | string | 类型 |
| reason  | string | 原因 |
| age     | string | 时间 |
| Message | string | 说明 |