package controller

import (
	"net/http/httptest"
	"testing"
)

func TestParseFollowDefaultsToTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50", nil)

	follow, err := parseFollow(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !follow {
		t.Fatal("expected follow to default to true")
	}
}

func TestParseFollowParsesFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50&follow=false", nil)

	follow, err := parseFollow(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if follow {
		t.Fatal("expected follow to be false")
	}
}

func TestParseFollowRejectsInvalidValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?follow=maybe", nil)

	if _, err := parseFollow(req); err == nil {
		t.Fatal("expected invalid follow value error")
	}
}
