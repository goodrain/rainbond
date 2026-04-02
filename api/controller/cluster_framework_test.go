package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/builder/parser/code"
	httputil "github.com/goodrain/rainbond/util/http"
)

func TestListCNBFrameworksDefaultsToNodejs(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/clusters/cnb/frameworks", nil)
	recorder := httptest.NewRecorder()

	(&ClusterController{}).ListCNBFrameworks(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var body httputil.ResponseBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	list, ok := body.List.([]interface{})
	if !ok {
		t.Fatalf("expected list response, got %#v", body.List)
	}

	expected := code.GetSupportedFrameworks("nodejs")
	if len(list) != len(expected) {
		t.Fatalf("expected %d frameworks, got %d", len(expected), len(list))
	}
}
