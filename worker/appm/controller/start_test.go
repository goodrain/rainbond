package controller

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/event"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
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
