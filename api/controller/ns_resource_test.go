package controller

import (
	"testing"

	"github.com/goodrain/rainbond/api/handler"
)

func TestBuildNsResourceCreatePayloadUsesStatusAndMessage(t *testing.T) {
	result := &handler.NsResourceCreateResponse{
		Message: "共创建 3 个资源，2 个成功，1 个失败",
		Summary: handler.NsResourceCreateSummary{
			Total:          3,
			SuccessCount:   2,
			FailureCount:   1,
			PartialSuccess: true,
		},
		Results: []handler.NsResourceCreateResult{
			{Index: 1, Kind: "Deployment", Name: "demo", Success: true},
		},
	}

	payload := buildNsResourceCreatePayload(207, result)

	if payload["code"] != 207 {
		t.Fatalf("expected code 207, got %#v", payload["code"])
	}
	if payload["msg"] != result.Message {
		t.Fatalf("expected msg %q, got %#v", result.Message, payload["msg"])
	}
	if payload["msg_show"] != result.Message {
		t.Fatalf("expected msg_show %q, got %#v", result.Message, payload["msg_show"])
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload data map, got %#v", payload["data"])
	}
	bean, ok := data["bean"].(*handler.NsResourceCreateResponse)
	if !ok {
		t.Fatalf("expected bean payload, got %#v", data["bean"])
	}
	if bean.Summary.FailureCount != 1 {
		t.Fatalf("expected bean failure count 1, got %d", bean.Summary.FailureCount)
	}
}
