package middleware

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type wrapELTestManager struct {
	db.Manager
	eventDao dbdao.EventDao
}

func (m wrapELTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

type wrapELTestEventDao struct {
	dbdao.EventDao
	added       int
	requestBody string
}

func (d *wrapELTestEventDao) GetLastASyncEvent(target, targetID string) (*dbmodel.ServiceEvent, error) {
	return nil, gorm.ErrRecordNotFound
}

func (d *wrapELTestEventDao) AddModel(arg dbmodel.Interface) error {
	d.added++
	if serviceEvent, ok := arg.(*dbmodel.ServiceEvent); ok {
		d.requestBody = serviceEvent.RequestBody
	}
	return nil
}

// capability_id: rainbond.vm-live-update.running-shrink-restart-allowed-before-event
func TestWrapELAllowsRunningVMShrinkToReachHandler(t *testing.T) {
	eventDao := &wrapELTestEventDao{}
	db.SetTestManager(wrapELTestManager{eventDao: eventDao})
	defer db.SetTestManager(nil)

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "cpu_shrink",
			body: `{"container_cpu":4000,"container_memory":12288}`,
		},
		{
			name: "memory_shrink",
			body: `{"container_cpu":6000,"container_memory":8192}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := WrapEL(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}, dbmodel.TargetTypeService, "vertical-service", dbmodel.ASYNEVENTTYPE, false)

			req := httptest.NewRequest(http.MethodPut, "/v2/tenants/demo/services/service-vm/vertical", bytes.NewBufferString(tc.body))
			ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant-1")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service"), &dbmodel.TenantServices{
				ServiceID:       "service-vm",
				ExtendMethod:    "vm",
				ContainerCPU:    6000,
				ContainerMemory: 12288,
			})
			req = req.WithContext(ctx)

			resp := httptest.NewRecorder()
			handler(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.Code)
			}
			if !called {
				t.Fatal("expected request to reach wrapped handler")
			}
		})
	}

	if eventDao.added != len(testCases) {
		t.Fatalf("expected one event per allowed vm shrink request, got %d", eventDao.added)
	}
}

// capability_id: rainbond.vm-runtime.password-hash-request-redaction
func TestDebugRequestBodyRedactsVMLoginSecrets(t *testing.T) {
	hash := generatedVMSHA512CryptTestValue(t)
	plain := generatedVMLoginPlaintextTestValue(t)
	body := `{"vm_login_password":"` + plain + `","nested":[{"vm_login_password":"` + plain + `"}],"login_password":"` + plain + `","login_password_hash":"` + hash + `","ssh_public_key":"ssh-ed25519 PUBKEY_SENTINEL","component_k8s_attributes":[{"name":"login_password","attribute_value":"` + plain + `"},{"name":"vm_login_password_hash","attribute_value":"` + hash + `","value":"` + hash + `"},{"name":"vm_ssh_authorized_key","attribute_value":"ssh-ed25519 PUBKEY_SENTINEL"}],"service_id":"service-vm","large_number":9007199254740993123456789}`
	logger := logrus.StandardLogger()
	var logs bytes.Buffer
	oldOutput, oldLevel := logger.Out, logger.Level
	logger.SetOutput(&logs)
	logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.SetOutput(oldOutput); logger.SetLevel(oldLevel) })
	request := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services", strings.NewReader(body))
	debugRequestBody(request)

	for _, forbidden := range []string{hash, plain, "ssh-ed25519 PUBKEY_SENTINEL"} {
		if strings.Contains(logs.String(), forbidden) {
			t.Fatal("request debug log contains a VM login credential")
		}
	}
	if !strings.Contains(logs.String(), "9007199254740993123456789") || !strings.Contains(logs.String(), "service-vm") {
		t.Fatal("safe request fields or exact large integer were not preserved in debug logging")
	}
	original, err := io.ReadAll(request.Body)
	if err != nil || string(original) != body {
		t.Fatal("debug redaction changed the request body passed to the handler")
	}

	logs.Reset()
	malformed := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services", strings.NewReader(`{"login_password":"`+hash))
	debugRequestBody(malformed)
	if strings.Contains(logs.String(), hash) || !strings.Contains(logs.String(), "[REDACTED") {
		t.Fatal("malformed sensitive request was logged without fail-closed redaction")
	}
}

// capability_id: rainbond.vm-runtime.password-hash-request-redaction
func TestWrapELRedactsVMLoginSecretsFromLogsAndEvents(t *testing.T) {
	hash := generatedVMSHA512CryptTestValue(t)
	plain := generatedVMLoginPlaintextTestValue(t)
	body := `{"vm_login_password":"` + plain + `","nested":[{"vm_login_password":"` + plain + `"}],"login_password":"` + plain + `","login_password_hash":"` + hash + `","ssh_public_key":"ssh-ed25519 PUBKEY_SENTINEL","component_k8s_attributes":[{"name":"login_password","attribute_value":"` + plain + `"},{"name":"vm_login_password_hash","attribute_value":"` + hash + `"},{"name":"vm_ssh_authorized_key","value":"ssh-ed25519 PUBKEY_SENTINEL"}],"service_id":"service-vm","large_number":9007199254740993123456789}`
	eventDAO := &wrapELTestEventDao{}
	db.SetTestManager(wrapELTestManager{eventDao: eventDAO})
	t.Cleanup(func() { db.SetTestManager(nil) })
	logger := logrus.StandardLogger()
	var logs bytes.Buffer
	oldOutput, oldLevel := logger.Out, logger.Level
	logger.SetOutput(&logs)
	logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.SetOutput(oldOutput); logger.SetLevel(oldLevel) })
	var handlerBody string
	wrapped := WrapEL(func(w http.ResponseWriter, r *http.Request) {
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error("wrapped handler could not read request body")
			return
		}
		handlerBody = string(requestBody)
		w.WriteHeader(http.StatusOK)
	}, dbmodel.TargetTypeService, "create-service", dbmodel.ASYNEVENTTYPE, false)
	request := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services", strings.NewReader(body))
	ctx := context.WithValue(request.Context(), ctxutil.ContextKey("tenant_id"), "tenant-1")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
	request = request.WithContext(ctx)
	wrapped(httptest.NewRecorder(), request)

	if handlerBody != body {
		t.Fatal("WrapEL changed the original request body before calling the handler")
	}
	for _, output := range []string{logs.String(), eventDAO.requestBody} {
		for _, forbidden := range []string{hash, plain, "ssh-ed25519 PUBKEY_SENTINEL"} {
			if strings.Contains(output, forbidden) {
				t.Fatal("request log or event body contains a VM login credential")
			}
		}
		if !strings.Contains(output, "9007199254740993123456789") || !strings.Contains(output, "service-vm") {
			t.Fatal("safe event fields or exact large integer were not preserved")
		}
	}

	logs.Reset()
	eventDAO.requestBody = ""
	malformedBody := `{"login_password_hash":"` + hash
	malformed := httptest.NewRequest(http.MethodPost, "/v2/tenants/demo/services", strings.NewReader(malformedBody))
	ctx = context.WithValue(malformed.Context(), ctxutil.ContextKey("tenant_id"), "tenant-1")
	ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
	malformed = malformed.WithContext(ctx)
	wrapped(httptest.NewRecorder(), malformed)
	if strings.Contains(logs.String(), hash) || strings.Contains(eventDAO.requestBody, hash) ||
		!strings.Contains(logs.String(), "[REDACTED") || !strings.Contains(eventDAO.requestBody, "[REDACTED") {
		t.Fatal("malformed sensitive route did not fail closed in logs and event body")
	}
}

func generatedVMSHA512CryptTestValue(t *testing.T) string {
	t.Helper()
	salt := make([]byte, 8)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatal("could not generate test hash material")
	}
	return "$6$rounds=500000$" + hex.EncodeToString(salt) + "$" + strings.Repeat("A", 86)
}

func generatedVMLoginPlaintextTestValue(t *testing.T) string {
	t.Helper()
	value := make([]byte, 16)
	if _, err := cryptorand.Read(value); err != nil {
		t.Fatal("could not generate test input")
	}
	return hex.EncodeToString(value)
}
