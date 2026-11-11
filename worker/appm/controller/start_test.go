package controller

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/event"
	appstore "github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLogVirtualMachineCreateFailureIncludesManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := event.NewMockLogger(ctrl)
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any())

	app := v1.AppService{
		AppServiceBase: v1.AppServiceBase{
			ServiceAlias: "gre41ac6",
		},
		Logger: mockLogger,
	}
	app.SetVirtualMachine(&kubevirtv1.VirtualMachine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubevirt.io/v1",
			Kind:       "VirtualMachine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gre41ac6",
			Namespace: "rbd-test",
		},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Domain: kubevirtv1.DomainSpec{
						Devices: kubevirtv1.Devices{
							HostDevices: []kubevirtv1.HostDevice{
								{
									Name:       "usb",
									DeviceName: "custom.device/usb-storage",
								},
							},
						},
					},
				},
			},
		},
	})

	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	defer logrus.SetOutput(originalOutput)

	controller := &startController{}
	controller.logVirtualMachineCreateFailure(app, fmt.Errorf("admission webhook denied the request"))

	output := logBuffer.String()
	for _, expected := range []string{
		"Service gre41ac6: VirtualMachine create request manifest:",
		"kind: VirtualMachine",
		"name: gre41ac6",
		"deviceName: custom.device/usb-storage",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %q, got %q", expected, output)
		}
	}
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-log-redaction
func TestLogVirtualMachineCreateFailureRedactsCloudInitUserData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := event.NewMockLogger(ctrl)
	var appLogMessage string
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).Times(2).Do(func(message string, _ map[string]string) {
		appLogMessage = message
	})

	const sentinelKey = "ssh-ed25519 AAAA_SENTINEL_PUBLIC_KEY user@host"
	userData := "#cloud-config\n{\"ssh_pwauth\":false,\"users\":[{\"name\":\"ubuntu\",\"ssh_authorized_keys\":[\"" + sentinelKey + "\"]}]}\n"
	app := v1.AppService{
		AppServiceBase: v1.AppServiceBase{ServiceAlias: "gre41ac6"},
		Logger:         mockLogger,
	}
	app.SetVirtualMachine(&kubevirtv1.VirtualMachine{
		TypeMeta:   metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine"},
		ObjectMeta: metav1.ObjectMeta{Name: "gre41ac6", Namespace: "rbd-test"},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Volumes: []kubevirtv1.Volume{{
						Name: "rainbond-guest-sync",
						VolumeSource: kubevirtv1.VolumeSource{
							CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
								UserData:       userData,
								UserDataBase64: base64.StdEncoding.EncodeToString([]byte(userData)),
							},
						},
					}},
				},
			},
		},
	})

	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	defer logrus.SetOutput(originalOutput)

	tests := []struct {
		name string
		err  string
	}{
		{name: "authorized key only", err: "admission webhook denied request containing " + sentinelKey},
		{name: "full cloud config", err: "admission webhook denied request: " + userData},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logBuffer.Reset()
			appLogMessage = ""
			controller := &startController{}
			controller.logVirtualMachineCreateFailure(app, fmt.Errorf("%s", tt.err))

			output := logBuffer.String()
			for _, leaked := range []string{sentinelKey, userData} {
				if strings.Contains(output, leaked) {
					t.Fatalf("cloud-init content leaked into VM failure log: %q", output)
				}
				if strings.Contains(appLogMessage, leaked) {
					t.Fatalf("cloud-init content leaked into application failure log: %q", appLogMessage)
				}
			}
			for _, expected := range []string{
				"Service gre41ac6: VirtualMachine create request manifest:",
				"name: gre41ac6",
				"[REDACTED]",
			} {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected log output to contain %q, got %q", expected, output)
				}
			}
		})
	}
	if got := app.GetVirtualMachine().Spec.Template.Spec.Volumes[0].CloudInitNoCloud.UserData; !strings.Contains(got, sentinelKey) {
		t.Fatalf("expected log redaction to preserve the original VM user data, got %q", got)
	}
}

// capability_id: rainbond.vm-runtime.password-hash-log-redaction
func TestLogVirtualMachinePasswordHashRedaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	hash := generatedVMPasswordHashForLogTest(t)
	userData := "#cloud-config\n{\"ssh_pwauth\":true,\"users\":[{\"name\":\"ubuntu\",\"hashed_passwd\":\"" + hash + "\",\"lock_passwd\":false}]}\n"
	logger := event.NewMockLogger(ctrl)
	var appMessages []string
	logger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes().Do(func(message string, _ map[string]string) {
		appMessages = append(appMessages, message)
	})
	app := appWithVM(&kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-test", Namespace: "rbd-test"},
		Spec: kubevirtv1.VirtualMachineSpec{Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
			Spec: kubevirtv1.VirtualMachineInstanceSpec{Volumes: []kubevirtv1.Volume{{
				Name: "rainbond-guest-sync",
				VolumeSource: kubevirtv1.VolumeSource{CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
					UserData: userData, UserDataBase64: base64.StdEncoding.EncodeToString([]byte(userData)),
				}}},
			}},
		}},
	})
	app.Logger = logger
	var logs bytes.Buffer
	standardLogger := logrus.StandardLogger()
	oldOutput := standardLogger.Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(oldOutput) })

	encodedUserData := base64.StdEncoding.EncodeToString([]byte(userData))
	for _, echoed := range []string{hash, userData, encodedUserData} {
		logs.Reset()
		appMessages = nil
		(&startController{}).logVirtualMachineCreateFailure(app, fmt.Errorf("admission denied: %s", echoed))
		for _, output := range append([]string{logs.String()}, appMessages...) {
			if strings.Contains(output, hash) || strings.Contains(output, userData) || strings.Contains(output, encodedUserData) {
				t.Fatal("VM password hash or cloud-init data appeared in failure logs")
			}
		}
		if !strings.Contains(logs.String(), "[REDACTED]") {
			t.Fatal("VM failure manifest omitted the redaction marker")
		}
	}
	if !strings.Contains(appMessages[0], "admission denied") {
		t.Fatal("safe VM failure diagnostic was not preserved")
	}
}

func generatedVMPasswordHashForLogTest(t *testing.T) string {
	t.Helper()
	salt := make([]byte, 8)
	if _, err := cryptorand.Read(salt); err != nil {
		t.Fatal("could not generate test hash material")
	}
	return "$6$rounds=500000$" + hex.EncodeToString(salt) + "$" + strings.Repeat("A", 86)
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-log-redaction
func TestStartOneReturnsRedactedVMCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	const sentinelKey = "ssh-ed25519 AAAA_SENTINEL_PUBLIC_KEY user@host"
	vm := vmWithSentinelCloudInit(sentinelKey)
	app := appWithVM(vm)
	logger := event.NewMockLogger(ctrl)
	logger.EXPECT().Error(gomock.Any(), gomock.Any())
	app.Logger = logger

	vmClient := kubecli.NewMockVirtualMachineInterface(ctrl)
	vmClient.EXPECT().Create(gomock.Any(), vm, gomock.Any()).Return(nil, fmt.Errorf("admission denied %s", sentinelKey))
	kubevirtClient := kubecli.NewMockKubevirtClient(ctrl)
	kubevirtClient.EXPECT().VirtualMachine("rbd-test").Return(vmClient)

	manager := &Manager{
		client:      kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "rbd-test"}}),
		kubevirtCli: kubevirtClient,
	}
	controller := &startController{manager: manager, ctx: context.Background()}

	err := controller.startOne(app)
	if err == nil {
		t.Fatal("expected VM create failure")
	}
	if strings.Contains(err.Error(), sentinelKey) || strings.Contains(err.Error(), "AAAA_SENTINEL_PUBLIC_KEY") {
		t.Fatalf("VM create error contains public key: %q", err)
	}
	if !strings.Contains(err.Error(), "[REDACTED]") || !strings.Contains(err.Error(), "admission denied") {
		t.Fatalf("expected safe diagnostic and redaction marker, got %q", err)
	}
}

// capability_id: rainbond.vm-runtime.ssh-authorized-keys-log-redaction
func TestBeginDoesNotLogVMCreateErrorPublicKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	const sentinelKey = "ssh-ed25519 AAAA_SENTINEL_PUBLIC_KEY user@host"
	userData := "#cloud-config\n{\"users\":[{\"name\":\"ubuntu\",\"ssh_authorized_keys\":[\"" + sentinelKey + "\"]}]}\n"
	app := appWithVM(vmWithSentinelCloudInit(sentinelKey))
	var appLogMessages []string
	mockLogger := event.NewMockLogger(ctrl)
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes().Do(func(message string, _ map[string]string) {
		appLogMessages = append(appLogMessages, message)
	})
	app.Logger = mockLogger

	vmClient := kubecli.NewMockVirtualMachineInterface(ctrl)
	vmClient.EXPECT().Create(gomock.Any(), app.GetVirtualMachine(), gomock.Any()).Return(nil, fmt.Errorf("admission denied %s", sentinelKey))
	vmClient.EXPECT().Delete(gomock.Any(), "vm-test", gomock.Any()).Return(apierrors.NewNotFound(schema.GroupResource{Resource: "virtualmachines"}, "vm-test"))
	kubevirtClient := kubecli.NewMockKubevirtClient(ctrl)
	kubevirtClient.EXPECT().VirtualMachine("rbd-test").Return(vmClient).Times(2)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kubevirtv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{
		client:        kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "rbd-test"}}),
		runtimeClient: controllerfake.NewClientBuilder().WithScheme(scheme).Build(),
		store:         &vmFailureTestStore{},
		kubevirtCli:   kubevirtClient,
	}
	controller := &startController{
		manager:    manager,
		ctx:        context.Background(),
		appService: []v1.AppService{app},
	}
	var logBuffer bytes.Buffer
	originalOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logBuffer)
	t.Cleanup(func() { logrus.SetOutput(originalOutput) })

	controller.Begin()

	if strings.Contains(logBuffer.String(), sentinelKey) || strings.Contains(logBuffer.String(), "AAAA_SENTINEL_PUBLIC_KEY") {
		t.Fatalf("Begin/logrus path leaked the public key: %q", logBuffer.String())
	}
	if strings.Contains(logBuffer.String(), userData) {
		t.Fatalf("Begin/logrus path leaked cloud-init data: %q", logBuffer.String())
	}
	joinedAppLogs := strings.Join(appLogMessages, "\n")
	if strings.Contains(joinedAppLogs, sentinelKey) || strings.Contains(joinedAppLogs, "AAAA_SENTINEL_PUBLIC_KEY") || strings.Contains(joinedAppLogs, userData) {
		t.Fatalf("Begin/app logger path leaked cloud-init data: %q", joinedAppLogs)
	}
	if !strings.Contains(joinedAppLogs, "admission denied") || !strings.Contains(joinedAppLogs, "[REDACTED]") {
		t.Fatalf("expected safe create diagnostics in app logs, got %q", joinedAppLogs)
	}
}

func vmWithSentinelCloudInit(publicKey string) *kubevirtv1.VirtualMachine {
	userData := "#cloud-config\n{\"users\":[{\"name\":\"ubuntu\",\"ssh_authorized_keys\":[\"" + publicKey + "\"]}]}\n"
	return &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-test", Namespace: "rbd-test"},
		Spec: kubevirtv1.VirtualMachineSpec{
			Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{
				Spec: kubevirtv1.VirtualMachineInstanceSpec{
					Volumes: []kubevirtv1.Volume{{
						Name: "rainbond-guest-sync",
						VolumeSource: kubevirtv1.VolumeSource{
							CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{UserData: userData},
						},
					}},
				},
			},
		},
	}
}

func appWithVM(vm *kubevirtv1.VirtualMachine) v1.AppService {
	app := v1.AppService{AppServiceBase: v1.AppServiceBase{ServiceAlias: "vm-alias"}}
	app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "rbd-test"}})
	app.SetVirtualMachine(vm)
	return app
}

type vmFailureTestStore struct {
	appstore.Storer
}

func (s *vmFailureTestStore) OnDeletes(...interface{}) {}

func (s *vmFailureTestStore) GetCrd(string) (*apiextensionsv1.CustomResourceDefinition, error) {
	return nil, nil
}

func (s *vmFailureTestStore) GetAppService(string) *v1.AppService { return nil }
