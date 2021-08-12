package prober

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober/results"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/probe"
)

func TestHTTPHeaders(t *testing.T) {
	testCases := []struct {
		input  []v1alpha1.HTTPHeader
		output http.Header
	}{
		{[]v1alpha1.HTTPHeader{}, http.Header{}},
		{[]v1alpha1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins"}}},
		{[]v1alpha1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
			{Name: "X-Muffins-Or-Plumcakes", Value: "Muffins!"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins"},
			"X-Muffins-Or-Plumcakes": {"Muffins!"}}},
		{[]v1alpha1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
			{Name: "X-Muffins-Or-Cupcakes", Value: "Cupcakes, too"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins", "Cupcakes, too"}}},
	}
	for _, test := range testCases {
		headers := buildHeader(test.input)
		if !reflect.DeepEqual(test.output, headers) {
			t.Errorf("Expected %#v, got %#v", test.output, headers)
		}
	}
}

func TestProbe(t *testing.T) {
	httpProbe := &v1alpha1.Probe{
		Handler: v1alpha1.Handler{
			HTTPGet: &v1alpha1.HTTPGetAction{},
		},
	}

	tests := []struct {
		name           string
		probe          *v1alpha1.Probe
		env            []v1.EnvVar
		execError      bool
		expectError    bool
		execResult     probe.Result
		expectedResult results.Result
		expectCommand  []string
	}{
		{
			name:           "No probe",
			probe:          nil,
			expectedResult: results.Success,
		},
		{
			name:           "No handler",
			probe:          &v1alpha1.Probe{},
			expectError:    true,
			expectedResult: results.Failure,
		},
		{
			name:           "Probe fails",
			probe:          httpProbe,
			execResult:     probe.Failure,
			expectedResult: results.Failure,
		},
		{
			name:           "Probe succeeds",
			probe:          httpProbe,
			execResult:     probe.Success,
			expectedResult: results.Success,
		},
		{
			name:           "Probe result is unknown",
			probe:          httpProbe,
			execResult:     probe.Unknown,
			expectedResult: results.Failure,
		},
		{
			name:           "Probe has an error",
			probe:          httpProbe,
			execError:      true,
			expectError:    true,
			execResult:     probe.Unknown,
			expectedResult: results.Failure,
		},
	}

	for i := range tests {
		test := tests[i]
		_ = test

		prober := &prober{
			recorder: &record.FakeRecorder{},
		}
		thirdComponent := &v1alpha1.ThirdComponent{
			Spec: v1alpha1.ThirdComponentSpec{
				Probe: test.probe,
			},
		}
		if test.execError {
			prober.http = fakeHTTPProber{test.execResult, errors.New("exec error")}
		} else {
			prober.http = fakeHTTPProber{test.execResult, nil}
		}

		result, err := prober.probe(thirdComponent, &v1alpha1.ThirdComponentEndpointStatus{}, "foobar")
		if test.expectError && err == nil {
			t.Errorf("[%s] Expected probe error but no error was returned.", test.name)
		}
		if !test.expectError && err != nil {
			t.Errorf("[%s] Didn't expect probe error but got: %v", test.name, err)
		}
		if test.expectedResult != result {
			t.Errorf("[%s] Expected result to be %v but was %v", test.name, test.expectedResult, result)
		}
	}
}

type fakeHTTPProber struct {
	result probe.Result
	err    error
}

func (p fakeHTTPProber) Probe(url *url.URL, headers http.Header, timeout time.Duration) (probe.Result, string, error) {
	return p.result, "", p.err
}
