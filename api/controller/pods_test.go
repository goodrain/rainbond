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

func TestParsePreviousDefaultsToFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?lines=50", nil)

	if parsePrevious(req) {
		t.Fatal("expected previous to default to false")
	}
}

func TestParsePreviousParsesTrue(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?previous=true", nil)

	if !parsePrevious(req) {
		t.Fatal("expected previous to be true")
	}
}

func TestParsePreviousInvalidValueTreatedAsFalse(t *testing.T) {
	req := httptest.NewRequest("GET", "/v2/tenants/demo/services/svc/pods/pod-1/logs?previous=maybe", nil)

	if parsePrevious(req) {
		t.Fatal("expected invalid previous value to be treated as false")
	}
}
