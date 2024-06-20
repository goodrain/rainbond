package dao

import (
	"fmt"
	"github.com/tidwall/gjson"
	"testing"
)

func TestJson(t *testing.T) {
	fmt.Println(gjson.Get(`{
    "_index": "appstore_tenant_services_event",
    "_type": "_doc",
    "_id": "1234",
    "_version": 4,
    "_seq_no": 3,
    "_primary_term": 1,
    "found": true,
    "_source": {
        "reason": null,
        "event_id": "1234",
        "message": "更新的消息1"
    }
}`, "_source").Raw)
}
