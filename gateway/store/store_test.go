package store

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	api "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRbdStore_checkIngress(t *testing.T) {
	ing := buildIngress()

	type foo struct {
		data     map[string]string
		expected bool
	}

	getFoo := func(expected bool, enable, host, port string) foo {
		return foo{
			expected: expected,
			data: map[string]string{
				parser.GetAnnotationWithPrefix("l4-enable"): enable,
				parser.GetAnnotationWithPrefix("l4-host"):   host,
				parser.GetAnnotationWithPrefix("l4-port"):   port,
			},
		}
	}

	testCases := []foo{
		getFoo(true, "true", "0.0.0.0", "12345"),
		getFoo(true, "true", "", "12345"),
		getFoo(false, "true", "0.0.0.0", "0"),
		getFoo(false, "true", "0.0.0.0", "65536"),
		getFoo(false, "true", "0.0.0.0", "65535"),
		{
			expected: true,
			data:     nil,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {})
	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%v", 65535),
		Handler: mux,
	}
	go server.ListenAndServe()

	for _, testCase := range testCases {
		ing.SetAnnotations(testCase.data)
		s := k8sStore{}
		if s.checkIngress(&ing.ObjectMeta) != testCase.expected {
			t.Errorf("Expected %v for s.checkIngress(ing), but returned %v. data: %v", testCase.expected,
				s.checkIngress(&ing.ObjectMeta), testCase.data)
		}
	}
}

func buildIngress() *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobar",
			Namespace: api.NamespaceDefault,
		},
	}
}
