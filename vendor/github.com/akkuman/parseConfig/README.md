# 配置文件解析工具

## 安装
```bash
go get github.com/akkuman/parseConfig
```

## 使用说明
### 环境假设
```
.
├── config.go
├── config.json
```
config.json内容
```json
{
    "name" : "akkuman",
    "urls" : ["xx.com","ww.com"],
    "info" : {
        "qq" : "123456",
        "weixin": "123456"
    }
}
```
该库取出来的都是类型为interface{}的数据，如需取出具体类型的数据需要自己加断言

当取嵌套map数据的时候，以“ > ”指定下一级，注意>两边均有空格，具体见下面的例子

例子
config.go内容
```golang
package main

import (
    "github.com/akkuman/parseConfig"
)

func main() {
    var config = parseConfig.New("config.json")
    // 此为interface{}格式数据
    var name = config.Get("name")
    // 断言
    var nameString = name.(string)
    
    // 取数组
    var urls = config.Get("urls").([]interface{})
    var urlsString []string
    for _,v := range urls {
        urlsString = append(urlsString, v.(string))
    }
    
    // 取嵌套map内数据
    var qq = config.Get("info > qq").("string")
    var weixin = config.Get("info > weixin").("string")
}

```

