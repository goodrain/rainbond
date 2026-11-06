package controller

import (
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

func TestCleanupReferenceRestConfigUsesIndependentRateLimiter(t *testing.T) {
	originalLimiter := flowcontrol.NewFakeAlwaysRateLimiter()
	original := &rest.Config{QPS: 1, Burst: 2, RateLimiter: originalLimiter}
	configured := cleanupReferenceRestConfig(original)
	if configured == original || configured.RateLimiter != nil || configured.QPS != 20 || configured.Burst != 40 {
		t.Fatal("cleanup reference client did not receive independent bounded rate limits")
	}
	if original.RateLimiter != originalLimiter || original.QPS != 1 || original.Burst != 2 {
		t.Fatal("shared platform REST config was mutated")
	}
}
