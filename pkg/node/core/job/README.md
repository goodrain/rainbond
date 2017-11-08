# JOB设计说明

节点执行master发送的job任务。job任务分为两类：

* 指定时间循环执行或定时执行类任务处理机制安装以下类库说明文档进行：
https://godoc.org/gopkg.in/robfig/cron.v2

* 立即执行类任务，任务接收到即执行。


任务执行完成如果返回格式化数据，解析数据并写回master节点进行处理。

# 
